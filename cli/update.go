package cli

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"focusd/coreapi"
	"focusd/system"
	"focusd/ui"

	"golang.org/x/sys/windows"
)

// metaClient has a short timeout for small API calls. The download client
// needs its own, longer budget so a slow connection does not abort a transfer.
var (
	metaClient     = &http.Client{Timeout: 20 * time.Second}
	downloadClient = &http.Client{Timeout: 10 * time.Minute}
)

func runUpdate() {
	ui.PrintHeader()
	fmt.Printf("Current Version: %s\n", system.Version)
	fmt.Println("Checking for updates...")

	latestVer, err := fetchLatestVersion()
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to check for updates: %v", err))
		return
	}

	switch compareVersions(latestVer, system.Version) {
	case 0:
		ui.PrintOK("You are on the latest version.")
		return
	case -1:
		ui.PrintInfo(fmt.Sprintf("Running a newer version than the latest release (%s).", system.Version))
		return
	}

	ui.PrintInfo(fmt.Sprintf("New version available: %s", latestVer))
	fmt.Print("Do you want to update? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to read input: %v", err))
		return
	}
	if strings.ToLower(strings.TrimSpace(response)) != "y" {
		fmt.Println("Update cancelled.")
		return
	}

	// Ask the daemon to stop over IPC first so buffered sessions are written
	// out. Hard-killing it dropped up to five minutes of tracked time.
	daemonWasRunning := system.CountInstancesInDir(system.DaemonProcessName, system.GetInstallDir()) >= 1
	if daemonWasRunning {
		ui.PrintStatus("Stopping focusd daemon...", "", false)
		if !coreapi.SendIPCCmd("stop") {
			ui.PrintWarn("Daemon did not acknowledge the stop request.")
			if err := system.KillInstancesInDir(system.DaemonProcessName, system.GetInstallDir()); err != nil {
				ui.PrintWarn(fmt.Sprintf("Could not stop daemon: %v", err))
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	ui.PrintStatus("Closing other focusd instances...", "", false)
	// Scoped to the install directory so an unrelated focusd.exe elsewhere on
	// the machine is not terminated.
	if err := system.KillInstancesInDir("focusd.exe", system.GetInstallDir()); err != nil {
		log.Printf("WARN: could not close other focusd instances: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	if err := performUpdate(latestVer); err != nil {
		ui.PrintError(fmt.Sprintf("Update failed: %v", err))
		if daemonWasRunning {
			ui.PrintInfo("Attempting to restart daemon...")
			if _, err := system.StartDaemon(); err != nil {
				log.Printf("WARN: failed to restart daemon after update: %v", err)
			}
		}
		os.Exit(1)
	}

	os.Exit(0)
}

// parseVersion splits a dotted version into comparable numeric parts,
// ignoring any pre-release suffix such as "-rc1" or "+build".
func parseVersion(v string) []int {
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")

	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			break
		}
		out = append(out, n)
	}
	return out
}

// compareVersions returns 1 when a is newer than b, -1 when older, 0 when equal.
// String equality was used before, so "2.13" and "2.13.0" looked different.
func compareVersions(a, b string) int {
	pa, pb := parseVersion(a), parseVersion(b)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			if x > y {
				return 1
			}
			return -1
		}
	}
	return 0
}

// checksumsURL is published alongside the installer in every release.
func checksumsURL(version string) string {
	return fmt.Sprintf("https://github.com/%s/%s/releases/download/v%s/checksums.txt",
		system.RepoOwner, system.RepoName, version)
}

func installerURL(version string) string {
	return fmt.Sprintf("https://github.com/%s/%s/releases/download/v%s/focusd_setup.exe",
		system.RepoOwner, system.RepoName, version)
}

// fetchExpectedSHA256 reads the published checksum for the installer. An empty
// result means the release has no usable checksum and the caller must refuse to
// run the download.
func fetchExpectedSHA256(version string) (string, error) {
	resp, err := metaClient.Get(checksumsURL(version))
	if err != nil {
		return "", fmt.Errorf("could not fetch checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksum fetch returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return "", err
	}

	sum, err := parseChecksumFor(string(body), installerAssetName)
	if err != nil {
		return "", err
	}
	return sum, nil
}

const installerAssetName = "focusd_setup.exe"

// parseChecksumFor pulls the SHA-256 for one asset out of a sha256sum-style
// manifest. It returns an error rather than an empty string so a caller can
// refuse to install when the release publishes nothing verifiable.
func parseChecksumFor(manifest, asset string) (string, error) {
	for _, line := range strings.Split(manifest, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if !strings.EqualFold(fields[1], asset) {
			continue
		}
		sum := strings.ToLower(fields[0])
		if len(sum) != 64 || !isHex(sum) {
			return "", fmt.Errorf("published checksum for %s is malformed", asset)
		}
		return sum, nil
	}
	return "", fmt.Errorf("no checksum published for %s", asset)
}

func isHex(s string) bool {
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

func fetchLatestVersion() (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", system.RepoOwner, system.RepoName), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "focusd-updater")
	resp, err := metaClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	return strings.TrimPrefix(release.TagName, "v"), nil
}

func performUpdate(version string) (err error) {
	expectedHash, err := fetchExpectedSHA256(version)
	if err != nil {
		return fmt.Errorf("refusing to update without a verified checksum: %w", err)
	}

	ui.PrintStatus("Downloading update...", "0%", false)

	tmpFile, err := os.CreateTemp("", "focusd-setup-*.exe")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	defer func() {
		tmpFile.Close()
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	resp, err := downloadClient.Get(installerURL(version))
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	hasher := sha256.New()
	if _, err = io.Copy(io.MultiWriter(tmpFile, hasher), resp.Body); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}
	if err = tmpFile.Close(); err != nil {
		return err
	}

	// The installer runs unelevated from here on. Verifying first means a
	// tampered or truncated download never reaches ShellExecute.
	actual := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(actual, expectedHash) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actual)
	}

	ui.PrintStatus("Installing...", "", false)

	pathPtr, err := syscall.UTF16PtrFromString(tmpPath)
	if err != nil {
		return err
	}
	argsPtr, err := syscall.UTF16PtrFromString("/S")
	if err != nil {
		return err
	}

	// No "runas" verb: focusd installs per-user under %APPDATA% and writes
	// HKCU, so elevation is unnecessary. It also wrote to the wrong profile
	// when an over-the-shoulder token was used.
	verbPtr, err := syscall.UTF16PtrFromString("open")
	if err != nil {
		return err
	}

	if err = windows.ShellExecute(0, verbPtr, pathPtr, argsPtr, nil, windows.SW_HIDE); err != nil {
		return fmt.Errorf("failed to start installer: %w", err)
	}

	ui.PrintOK("Installer started silently. focusd will now close to complete the update.")
	return nil
}
