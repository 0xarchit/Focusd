//go:build windows

package system

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func installerHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func TestVerifiedInstallerStageRejectsBadHashAndOversize(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	for _, test := range []struct {
		content string
		hash    string
		limit   int64
	}{{"wrong", installerHash("wanted"), 20}, {"too large", installerHash("too large"), 3}} {
		if stage, err := StageVerifiedInstaller(strings.NewReader(test.content), test.hash, test.limit); err == nil || stage != nil {
			t.Fatalf("invalid installer accepted: stage=%v err=%v", stage, err)
		}
	}
	stages, err := filepath.Glob(filepath.Join(GetInstallDir(), "updates", "stage-*"))
	if err != nil || len(stages) != 0 {
		t.Fatalf("failed stages remain=%v err=%v", stages, err)
	}
}

func TestVerifiedInstallerStageDeniesWriteAndDeleteUntilClosed(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	stage, err := StageVerifiedInstaller(strings.NewReader("installer"), installerHash("installer"), 20)
	if err != nil {
		t.Fatal(err)
	}
	defer stage.Close()
	if file, err := os.OpenFile(stage.Path, os.O_WRONLY|os.O_TRUNC, 0); err == nil {
		file.Close()
		t.Fatal("verified image permitted writing")
	}
	if err := os.Remove(stage.Path); err == nil {
		t.Fatal("verified image permitted deletion")
	}
	if err := CleanupUpdateStages(filepath.Dir(stage.directory)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stage.Path); err != nil {
		t.Fatalf("cleanup deleted active locked stage: %v", err)
	}
	if err := stage.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stage.directory); !os.IsNotExist(err) {
		t.Fatalf("aborted stage still exists: %v", err)
	}
}

func TestCompletedUpdateStageCleanupLeavesUnknownContents(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	stage, err := StageVerifiedInstaller(strings.NewReader("installer"), installerHash("installer"), 20)
	if err != nil {
		t.Fatal(err)
	}
	stage.file.Close()
	stage.file = nil
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stage.directory, old, old); err != nil {
		t.Fatal(err)
	}
	unknown := filepath.Join(stage.directory, "unrelated.txt")
	if err := os.WriteFile(unknown, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := CleanupUpdateStages(filepath.Dir(stage.directory)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(unknown); err != nil {
		t.Fatalf("cleanup removed unknown contents: %v", err)
	}
	if err := os.Remove(unknown); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(stage.directory, old, old); err != nil {
		t.Fatal(err)
	}
	if err := CleanupUpdateStages(filepath.Dir(stage.directory)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stage.directory); !os.IsNotExist(err) {
		t.Fatalf("completed stage remains: %v", err)
	}
}

func TestVerifiedInstallerLauncherKeepsLockUntilExitAndCleans(t *testing.T) {
	for _, required := range []string{"[IO.FileShare]::Read", "ComputeHash($stream)", "$installer.WaitForExit()", "finally", "Remove-Item -LiteralPath $image"} {
		if !strings.Contains(verifiedInstallerScript, required) {
			t.Fatalf("launcher missing %q", required)
		}
	}
	if strings.Contains(strings.ToLower(verifiedInstallerScript), "runas") {
		t.Fatal("launcher must not request elevation")
	}
}
