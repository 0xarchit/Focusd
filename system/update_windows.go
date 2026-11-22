//go:build windows

package system

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var ErrInstallerStartupUncertain = errors.New("installer launcher owns the staged image but startup confirmation is unavailable")

// StagedInstaller keeps the verified bytes locked until an independent launcher
// has acquired the same read-only sharing lock. The launcher owns cleanup after
// the installer exits, including when the installer closes the invoking CLI.
type StagedInstaller struct {
	Path        string
	directory   string
	expectedSHA string
	file        *os.File
	launched    bool
}

// StageVerifiedInstaller bounds the download and publishes only verified bytes
// beneath a private, uniquely named directory in this user's Focusd profile.
func StageVerifiedInstaller(reader io.Reader, expectedSHA string, maxBytes int64) (*StagedInstaller, error) {
	if len(expectedSHA) != sha256.Size*2 {
		return nil, fmt.Errorf("invalid installer checksum")
	}
	if _, err := hex.DecodeString(expectedSHA); err != nil || maxBytes < 1 {
		return nil, fmt.Errorf("invalid installer checksum or size limit")
	}
	root := GetInstallDir()
	if root == "" {
		return nil, fmt.Errorf("APPDATA environment variable not set")
	}
	updates := filepath.Join(root, "updates")
	if err := ensurePrivateFileParent(filepath.Join(updates, "placeholder")); err != nil {
		return nil, err
	}
	if err := CleanupUpdateStages(updates); err != nil {
		return nil, fmt.Errorf("clean completed update stages: %w", err)
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	directory := filepath.Join(updates, "stage-"+hex.EncodeToString(nonce[:]))
	if err := createPrivateUpdateDirectory(directory); err != nil {
		return nil, err
	}
	stage := &StagedInstaller{Path: filepath.Join(directory, "focusd_setup.exe"), directory: directory, expectedSHA: strings.ToLower(expectedSHA)}
	if err := WritePrivateFileAtomic(stage.Path, func(writer io.Writer) error {
		hash := sha256.New()
		written, err := io.Copy(io.MultiWriter(writer, hash), io.LimitReader(reader, maxBytes+1))
		if err != nil {
			return fmt.Errorf("write installer: %w", err)
		}
		if written > maxBytes {
			return fmt.Errorf("installer exceeds %d byte limit", maxBytes)
		}
		actual := hex.EncodeToString(hash.Sum(nil))
		if actual != stage.expectedSHA {
			return fmt.Errorf("installer checksum mismatch")
		}
		return nil
	}); err != nil {
		stage.Close()
		return nil, err
	}
	file, err := lockVerifiedInstaller(stage.Path, stage.expectedSHA)
	if err != nil {
		stage.Close()
		return nil, err
	}
	stage.file = file
	return stage, nil
}

// CleanupUpdateStages removes only recognized files from prior operation
// directories. An image with a live verification/launcher lock is left alone;
// reparse points, unknown contents and other directories are never followed.
func CleanupUpdateStages(updates string) error {
	if err := rejectReparsePath(updates); err != nil {
		return err
	}
	entries, err := os.ReadDir(updates)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || !strings.HasPrefix(name, "stage-") || len(name) != len("stage-")+32 {
			continue
		}
		if _, err := hex.DecodeString(strings.TrimPrefix(name, "stage-")); err != nil {
			continue
		}
		directory := filepath.Join(updates, name)
		info, infoErr := entry.Info()
		if infoErr != nil || time.Since(info.ModTime()) < time.Hour {
			continue
		}
		if err := rejectReparsePath(directory); err != nil {
			continue
		}
		files, err := os.ReadDir(directory)
		if err != nil || len(files) == 0 {
			continue
		}
		recognized := true
		for _, file := range files {
			if file.IsDir() || (file.Name() != "focusd_setup.exe" && file.Name() != "launch.ps1" && file.Name() != "ready.json" && file.Name() != "failed.txt" && file.Name() != "acknowledged" && file.Name() != "locked") || rejectReparsePath(filepath.Join(directory, file.Name())) != nil {
				recognized = false
				break
			}
		}
		if !recognized {
			continue
		}
		image := filepath.Join(directory, "focusd_setup.exe")
		if err := os.Remove(image); err != nil && !os.IsNotExist(err) {
			continue
		}
		for _, file := range files {
			_ = os.Remove(filepath.Join(directory, file.Name()))
		}
		_ = os.Remove(directory)
	}
	return nil
}

func createPrivateUpdateDirectory(path string) error {
	descriptor, err := privateFileDescriptor()
	if err != nil {
		return err
	}
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attributes := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: descriptor}
	err = windows.CreateDirectory(ptr, &attributes)
	runtime.KeepAlive(descriptor)
	return err
}

// StartUpdateDaemon restarts only the image identified by the authenticated
// daemon endpoint. It never searches PATH or replaces installation files.
func StartUpdateDaemon(path string) error {
	if !filepath.IsAbs(path) || !strings.EqualFold(filepath.Base(path), DaemonProcessName) {
		return fmt.Errorf("invalid daemon restart path")
	}
	command := exec.Command(path)
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP}
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

func lockVerifiedInstaller(path, expectedSHA string) (*os.File, error) {
	if err := validatePrivateFilePath(path); err != nil {
		return nil, err
	}
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(ptr, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, fmt.Errorf("lock verified installer: %w", err)
	}
	file := os.NewFile(uintptr(handle), path)
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		file.Close()
		return nil, err
	}
	if hex.EncodeToString(hash.Sum(nil)) != strings.ToLower(expectedSHA) {
		file.Close()
		return nil, fmt.Errorf("installer changed before launch")
	}
	return file, nil
}

// Close removes an aborted operation. After Launch succeeds the independent
// launcher owns the staged files, so closing the CLI does not remove a running
// installer's image.
func (stage *StagedInstaller) Close() error {
	if stage.file != nil {
		stage.file.Close()
		stage.file = nil
	}
	if stage.launched {
		return nil
	}
	var result error
	for _, name := range []string{"focusd_setup.exe", "launch.ps1", "ready.json", "failed.txt", "acknowledged", "locked"} {
		if err := os.Remove(filepath.Join(stage.directory, name)); err != nil && !os.IsNotExist(err) {
			result = err
		}
	}
	if err := os.Remove(stage.directory); err != nil && !os.IsNotExist(err) {
		result = err
	}
	return result
}

// Launch returns only once a separate helper has reverified and started the
// installer under a read-only sharing lock. No elevation or shell verb is used.
func (stage *StagedInstaller) Launch() error {
	if stage.file == nil || stage.launched {
		return fmt.Errorf("installer stage is not ready")
	}
	scriptPath := filepath.Join(stage.directory, "launch.ps1")
	if err := WritePrivateFileAtomic(scriptPath, func(writer io.Writer) error {
		_, err := io.WriteString(writer, verifiedInstallerScript)
		return err
	}); err != nil {
		return err
	}
	powershell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	if !filepath.IsAbs(powershell) {
		return fmt.Errorf("Windows PowerShell path unavailable")
	}
	command := exec.Command(powershell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
	command.Env = append(os.Environ(), "FOCUSD_UPDATE_DIR="+stage.directory, "FOCUSD_UPDATE_SHA256="+stage.expectedSHA)
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP}
	if err := command.Start(); err != nil {
		return fmt.Errorf("start installer launcher: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	deadline := time.NewTimer(15 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-deadline.C:
			if _, err := os.Stat(filepath.Join(stage.directory, "locked")); err == nil {
				stage.launched = true
				stage.file.Close()
				stage.file = nil
				return ErrInstallerStartupUncertain
			}
			_ = command.Process.Kill()
			<-done
			return fmt.Errorf("installer did not confirm startup")
		case err := <-done:
			if stage.started() {
				stage.launched = true
				stage.file.Close()
				stage.file = nil
				return nil
			}
			if failure, readErr := os.ReadFile(filepath.Join(stage.directory, "failed.txt")); readErr == nil {
				return fmt.Errorf("installer launcher: %s", strings.TrimSpace(string(failure)))
			}
			return fmt.Errorf("installer launcher exited before startup: %v", err)
		case <-tick.C:
			if stage.started() {
				stage.launched = true
				stage.file.Close()
				stage.file = nil
				return nil
			}
		}
	}
}

func (stage *StagedInstaller) started() bool {
	data, err := os.ReadFile(filepath.Join(stage.directory, "ready.json"))
	if err != nil || len(data) > 1024 {
		return false
	}
	var ready struct {
		PID int `json:"pid"`
	}
	if json.Unmarshal(data, &ready) != nil || ready.PID < 1 {
		return false
	}
	_ = os.WriteFile(filepath.Join(stage.directory, "acknowledged"), []byte("started"), 0600)
	return true
}

const verifiedInstallerScript = `
$ErrorActionPreference = 'Stop'
$directory = $env:FOCUSD_UPDATE_DIR
$image = Join-Path $directory 'focusd_setup.exe'
$ready = Join-Path $directory 'ready.json'
$stream = $null
$installer = $null
try {
    $stream = [IO.File]::Open($image, [IO.FileMode]::Open, [IO.FileAccess]::Read, [IO.FileShare]::Read)
    $hash = [Security.Cryptography.SHA256]::Create()
    try { $actual = [BitConverter]::ToString($hash.ComputeHash($stream)).Replace('-', '').ToLowerInvariant() }
    finally { $hash.Dispose() }
    if ($actual -ne $env:FOCUSD_UPDATE_SHA256) { throw 'Installer changed before launch' }
    'locked' | Set-Content -LiteralPath (Join-Path $directory 'locked') -Encoding ASCII
    $installer = Start-Process -FilePath $image -ArgumentList '/S' -PassThru -WindowStyle Hidden
    @{pid=$installer.Id} | ConvertTo-Json -Compress | Set-Content -LiteralPath $ready -Encoding ASCII
    $installer.WaitForExit()
} catch {
    $_.Exception.Message | Set-Content -LiteralPath (Join-Path $directory 'failed.txt') -Encoding UTF8
    if ($installer) { $installer.WaitForExit() }
    exit 1
} finally {
    if ($stream) { $stream.Dispose() }
    for ($attempt = 0; $attempt -lt 100; $attempt++) {
        try { Remove-Item -LiteralPath $image -Force -ErrorAction Stop; break }
        catch { if (-not (Test-Path -LiteralPath $image)) { break }; Start-Sleep -Milliseconds 100 }
    }
    if ($installer) {
        $acknowledged = Join-Path $directory 'acknowledged'
        $wait = [Diagnostics.Stopwatch]::StartNew()
        while (-not (Test-Path -LiteralPath $acknowledged) -and $wait.Elapsed.TotalSeconds -lt 15) { Start-Sleep -Milliseconds 100 }
        Remove-Item -LiteralPath $acknowledged -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath (Join-Path $directory 'locked') -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath $ready -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath (Join-Path $directory 'failed.txt') -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath (Join-Path $directory 'launch.ps1') -Force -ErrorAction SilentlyContinue
        try { [IO.Directory]::Delete($directory, $false) } catch { }
    }
}
`
