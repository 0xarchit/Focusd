package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseNotesHandlesEveryOutputPath(t *testing.T) {
	tests := []struct {
		name    string
		commits []string
		apiKey  string
		notes   string
		aiErr   error
		want    string
	}{
		{"no key", []string{"Commit: 1\nMessage: fix: preserve history\nChanges: test"}, "", "", nil, "## Commits\nMessage: fix: preserve history"},
		{"no commits", nil, "key", "", nil, "System optimizations and stability improvements."},
		{"AI failure", []string{"Commit: 1\nMessage: fix: preserve history\nChanges: test"}, "key", "", errors.New("offline"), "## Commits\nMessage: fix: preserve history"},
		{"AI success", []string{"Commit: 1\nMessage: fix: preserve history\nChanges: test"}, "key", "Generated notes", nil, "Generated notes"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			generate := func([]string, string) (string, error) { return tc.notes, tc.aiErr }
			if err := runReleaseNotes(tc.commits, tc.apiKey, generate); err != nil {
				t.Fatal(err)
			}
			contents, err := os.ReadFile(OutputFile)
			if err != nil || string(contents) != tc.want {
				t.Fatalf("output = %q, err %v; want %q", contents, err, tc.want)
			}
			if err := os.Remove(OutputFile); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(OutputFile, 0700); err != nil {
				t.Fatal(err)
			}
			if err := runReleaseNotes(tc.commits, tc.apiKey, generate); err == nil {
				t.Fatal("directory output path did not report failure")
			}
		})
	}
}

func TestReleaseOutputHelper(t *testing.T) {
	if os.Getenv("FOCUSD_RELEASE_OUTPUT_HELPER") == "1" {
		main()
	}
}

func TestFallbackOutputFailureExitsNonzero(t *testing.T) {
	for _, apiKey := range []string{"", "test-key"} {
		name := "no-api-key"
		if apiKey != "" {
			name = "no-commits"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, OutputFile), 0700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestReleaseOutputHelper$")
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "FOCUSD_RELEASE_OUTPUT_HELPER=1",
				"GH_MODELS_API_KEY="+apiKey, "OPENROUTER_API_KEY=")
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("failed fallback write exited successfully: %s", output)
			}
		})
	}
}

func TestCommitDataIncludesCoreAPIChanges(t *testing.T) {
	dir := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2026-11-19T10:00:00+05:30",
			"GIT_COMMITTER_DATE=2026-11-19T10:00:00+05:30")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	runGit("init", "--quiet")
	if err := os.Mkdir(filepath.Join(dir, "coreapi"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "coreapi", "example.go"), []byte("package coreapi\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runGit("add", "--", "coreapi/example.go")
	runGit("-c", "user.name=Focusd Test", "-c", "user.email=focusd-test@example.invalid",
		"commit", "--quiet", "-m", "fix(coreapi): retain shared timer behavior")
	t.Chdir(dir)
	data := getCommitData()
	if len(data) != 1 || !strings.Contains(data[0], "coreapi/example.go") {
		t.Fatalf("coreapi-only commit missing from release data: %v", data)
	}
}
