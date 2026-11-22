//go:build windows

package system

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestPrivateFileWriteFailurePreservesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.csv")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	injected := errors.New("injected write failure")
	err := WritePrivateFileAtomic(path, func(w io.Writer) error {
		if _, err := io.WriteString(w, "partial"); err != nil {
			t.Fatal(err)
		}
		return injected
	})
	if !errors.Is(err, injected) {
		t.Fatalf("write error = %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "original" {
		t.Fatalf("original contents = %q, err=%v", content, err)
	}
	files, err := filepath.Glob(filepath.Join(dir, ".focusd-export-*"))
	if err != nil || len(files) != 0 {
		t.Fatalf("staged files remain: %v, err=%v", files, err)
	}
}

func TestPrivateFilePublishesProtectedACLOnNewAndReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	for _, value := range []string{"first", "replacement"} {
		if err := WritePrivateFileAtomic(path, func(w io.Writer) error {
			_, err := io.WriteString(w, value)
			return err
		}); err != nil {
			t.Fatalf("publish %q: %v", value, err)
		}
		content, err := os.ReadFile(path)
		if err != nil || string(content) != value {
			t.Fatalf("contents = %q, err=%v", content, err)
		}
		descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.OWNER_SECURITY_INFORMATION)
		if err != nil {
			t.Fatal(err)
		}
		control, _, err := descriptor.Control()
		if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
			t.Fatalf("DACL protection = %x, err=%v", control, err)
		}
		user, err := windows.GetCurrentProcessToken().GetTokenUser()
		if err != nil {
			t.Fatal(err)
		}
		owner, _, err := descriptor.Owner()
		if err != nil || owner == nil || !owner.Equals(user.User.Sid) {
			t.Fatalf("file owner does not match current user: %v", err)
		}
		sddl := descriptor.String()
		for _, denied := range []string{";;;WD)", ";;;BU)", ";;;AU)"} {
			if strings.Contains(sddl, denied) {
				t.Fatalf("export ACL contains broad grant %q: %s", denied, sddl)
			}
		}
		if !strings.Contains(sddl, ";;;SY)") || !strings.Contains(sddl, ";;;BA)") {
			t.Fatalf("expected SYSTEM/admin grants: %s", sddl)
		}
	}
}

func TestPrivateFileRejectsDirectoryAndAlternateDataStream(t *testing.T) {
	dir := t.TempDir()
	for _, path := range []string{dir, filepath.Join(dir, "report.csv:secret")} {
		called := false
		err := WritePrivateFileAtomic(path, func(io.Writer) error {
			called = true
			return nil
		})
		if err == nil || called {
			t.Fatalf("invalid destination %q: err=%v, writer_called=%v", path, err, called)
		}
	}
}

func TestPrivateFileCreationUsesPrivateStagingDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	if err := WritePrivateFileAtomic(path, func(w io.Writer) error {
		file, ok := w.(*os.File)
		if !ok {
			t.Fatalf("staged writer type = %T", w)
		}
		stageDir := filepath.Dir(file.Name())
		if filepath.Dir(stageDir) != dir {
			t.Fatalf("staging directory escaped target parent: %q", stageDir)
		}
		descriptor, err := windows.GetNamedSecurityInfo(stageDir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			return err
		}
		control, _, err := descriptor.Control()
		if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
			t.Fatalf("staging DACL protection = %x, err=%v", control, err)
		}
		_, err = io.WriteString(w, "content")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(dir, ".focusd-export-*"))
	if err != nil || len(files) != 0 {
		t.Fatalf("staging directory remains: %v, err=%v", files, err)
	}
}

func TestPrivateFilePublicationFailurePreservesLockedDestination(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.csv")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(ptr, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	if err := WritePrivateFileAtomic(path, func(w io.Writer) error {
		_, err := io.WriteString(w, "replacement")
		return err
	}); err == nil {
		t.Fatal("expected publication failure for delete-locked destination")
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "original" {
		t.Fatalf("locked original contents = %q, err=%v", content, err)
	}
	files, err := filepath.Glob(filepath.Join(dir, ".focusd-export-*"))
	if err != nil || len(files) != 0 {
		t.Fatalf("staged files remain after publication failure: %v, err=%v", files, err)
	}
}
