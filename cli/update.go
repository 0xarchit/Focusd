package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"focusd/coreapi"
	"focusd/system"
	"focusd/ui"
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

	if err := performUpdate(latestVer); err != nil {
		ui.PrintError(fmt.Sprintf("Update failed: %v", err))
		os.Exit(1)
	}

	os.Exit(0)
}

// parseVersion splits a dotted version into comparable numeric parts. A
// pre-release suffix ("-rc1") is returned separately because it sorts before
// the release it leads to; build metadata ("+build") is ignored.
func parseVersion(v string) ([]int, string) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}

	pre := ""
	if i := strings.IndexByte(v, '-'); i >= 0 {
		pre = v[i:]
		v = v[:i]
	}

	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			break
		}
		out = append(out, n)
	}
	return out, pre
}

// compareVersions returns 1 when a is newer than b, -1 when older, 0 when equal.
// String equality was used before, so "2.13" and "2.13.0" looked different.
// When the numbers match, the release outranks its pre-releases: "2.13.0" is
// newer than "2.13.0-rc1".
func compareVersions(a, b string) int {
	pa, preA := parseVersion(a)
	pb, preB := parseVersion(b)
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
	if preA == preB {
		return 0
	}
	if preA == "" {
		return 1
	}
	if preB == "" {
		return -1
	}
	aIdentifiers := strings.Split(strings.TrimPrefix(preA, "-"), ".")
	bIdentifiers := strings.Split(strings.TrimPrefix(preB, "-"), ".")
	for i := 0; i < len(aIdentifiers) && i < len(bIdentifiers); i++ {
		aID, bID := aIdentifiers[i], bIdentifiers[i]
		if aID == bID {
			continue
		}
		aNumeric, bNumeric := numericVersionIdentifier(aID), numericVersionIdentifier(bID)
		if aNumeric != bNumeric {
			if aNumeric {
				return -1
			}
			return 1
		}
		if aNumeric {
			aID, bID = strings.TrimLeft(aID, "0"), strings.TrimLeft(bID, "0")
			if len(aID) < len(bID) {
				return -1
			}
			if len(aID) > len(bID) {
				return 1
			}
		}
		if order := strings.Compare(aID, bID); order != 0 {
			return order
		}
	}
	if len(aIdentifiers) < len(bIdentifiers) {
		return -1
	}
	if len(aIdentifiers) > len(bIdentifiers) {
		return 1
	}
	return 0
}

func numericVersionIdentifier(identifier string) bool {
	if identifier == "" {
		return false
	}
	for _, r := range identifier {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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

const maxInstallerBytes = 128 << 20

type updateStage interface {
	Launch() error
	Close() error
}

var (
	stageInstaller = func(reader io.Reader, expected string, limit int64) (updateStage, error) {
		return system.StageVerifiedInstaller(reader, expected, limit)
	}
	prepareForUpdate = prepareUpdateProcesses
	updateRPC        = coreapi.Call
	updateEndpoint   = coreapi.ReadEndpoint
	updateCount      = system.CountInstancesInDirChecked
)

func performUpdate(version string) error {
	expectedHash, err := fetchExpectedSHA256(version)
	if err != nil {
		return fmt.Errorf("refusing to update without a verified checksum: %w", err)
	}

	ui.PrintStatus("Downloading update...", "0%", false)

	resp, err := downloadClient.Get(installerURL(version))
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	if resp.ContentLength > maxInstallerBytes {
		return fmt.Errorf("installer exceeds %d byte limit", maxInstallerBytes)
	}
	stage, err := stageInstaller(resp.Body, expectedHash, maxInstallerBytes)
	if err != nil {
		return fmt.Errorf("verify installer: %w", err)
	}
	defer stage.Close()

	restart, err := prepareForUpdate()
	if err != nil {
		if restart != nil {
			if restartErr := restart(); restartErr != nil {
				return fmt.Errorf("update preparation failed: %v; daemon restart failed: %w", err, restartErr)
			}
		}
		return err
	}
	ui.PrintStatus("Installing...", "", false)
	if err := stage.Launch(); err != nil {
		if restart != nil && !errors.Is(err, system.ErrInstallerStartupUncertain) {
			if restartErr := restart(); restartErr != nil {
				return fmt.Errorf("installer launch failed: %v; daemon restart failed: %w", err, restartErr)
			}
		}
		return fmt.Errorf("failed to start installer: %w", err)
	}

	ui.PrintOK("Installer started silently. focusd will now close to complete the update.")
	return nil
}

func prepareUpdateProcesses() (func() error, error) {
	endpoint, err := updateEndpoint()
	if os.IsNotExist(err) {
		siblingDirectory, dirErr := daemonDirectory()
		if dirErr != nil {
			return nil, dirErr
		}
		for _, directory := range []string{siblingDirectory, system.GetInstallDir()} {
			if directory == "" {
				continue
			}
			count, countErr := updateCount(system.DaemonProcessName, directory)
			if countErr != nil || count > 0 {
				return nil, fmt.Errorf("cannot verify running daemon before update: %v", countErr)
			}
		}
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read daemon identity: %w", err)
	}
	request := coreapi.Request{Command: "shutdown-clients", Generation: endpoint.Generation}
	if _, err := updateRPC(request); err != nil {
		return nil, fmt.Errorf("request terminal cleanup: %w", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		response, err := updateRPC(coreapi.Request{Command: "status", Generation: endpoint.Generation})
		if err != nil {
			return nil, fmt.Errorf("verify terminal cleanup: %w", err)
		}
		var status struct {
			ActiveClients *int `json:"active_clients"`
		}
		if err := json.Unmarshal(response.Data, &status); err != nil || status.ActiveClients == nil {
			return nil, fmt.Errorf("daemon did not provide terminal cleanup status")
		}
		if *status.ActiveClients == 0 {
			break
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("a Focusd terminal has not exited; update cancelled")
		}
		time.Sleep(100 * time.Millisecond)
	}
	if _, err := updateRPC(coreapi.Request{Command: "stop", Generation: endpoint.Generation}); err != nil {
		return nil, fmt.Errorf("stop daemon before update: %w", err)
	}
	restart := func() error { return system.StartUpdateDaemon(endpoint.ExePath) }
	deadline = time.Now().Add(3 * time.Second)
	for {
		count, err := updateCount(system.DaemonProcessName, filepath.Dir(endpoint.ExePath))
		if err != nil {
			return restart, fmt.Errorf("verify daemon shutdown: %w", err)
		}
		if count == 0 {
			break
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("daemon has not exited; update cancelled")
		}
		time.Sleep(100 * time.Millisecond)
	}
	return restart, nil
}
