package cli

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{"equal", "2.13.0", "2.13.0", 0},
		{"equal with patch shorthand", "2.13", "2.13.0", 0},
		{"equal ignoring v prefix", "v2.13.0", "2.13.0", 0},
		{"patch greater", "2.13.1", "2.13.0", 1},
		{"minor greater", "2.14.0", "2.13.9", 1},
		{"major greater", "3.0.0", "2.99.99", 1},
		{"patch lesser", "2.13.0", "2.13.1", -1},
		{"minor lesser", "2.12.9", "2.13.0", -1},
		{"newer than published", "2.14.0", "2.13.0", 1},
		{"prerelease suffix ignored", "2.13.0-rc1", "2.13.0", 0},
		{"build metadata ignored", "2.13.0+abc", "2.13.0", 0},
		{"two part versus three part", "2.13", "2.13.0", 0},
		{"empty equals empty", "", "", 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := compareVersions(tc.a, tc.b); got != tc.want {
				t.Errorf("compareVersions(%q, %q) = %d; want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// TestCompareVersionsNoUpdateLoop is the regression guard for the original
// string-equality check, which treated 2.13 and 2.13.0 as different releases.
func TestCompareVersionsNoUpdateLoop(t *testing.T) {
	if got := compareVersions("2.13", "2.13.0"); got != 0 {
		t.Errorf("expected 2.13 and 2.13.0 to be equal, got %d", got)
	}
}

func TestParseChecksumFor(t *testing.T) {
	valid := "a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90"

	tests := []struct {
		name     string
		manifest string
		asset    string
		want     string
		wantErr  bool
	}{
		{
			name:     "single entry",
			manifest: valid + "  focusd_setup.exe\n",
			asset:    installerAssetName,
			want:     valid,
		},
		{
			name:     "picks the requested asset",
			manifest: "1111111111111111111111111111111111111111111111111111111111111111  focusd-v3.zip\n" + valid + "  focusd_setup.exe\n",
			asset:    installerAssetName,
			want:     valid,
		},
		{
			name:     "tolerates spaces and CRLF",
			manifest: valid + "   focusd_setup.exe\r\n",
			asset:    installerAssetName,
			want:     valid,
		},
		{
			name:     "uppercase hex is normalized",
			manifest: "A1B2C3D4E5F60718293A4B5C6D7E8F90A1B2C3D4E5F60718293A4B5C6D7E8F90  focusd_setup.exe\n",
			asset:    installerAssetName,
			want:     valid,
		},
		{
			name:     "missing asset is an error",
			manifest: valid + "  something_else.exe\n",
			asset:    installerAssetName,
			wantErr:  true,
		},
		{
			name:     "empty manifest is an error",
			manifest: "",
			asset:    installerAssetName,
			wantErr:  true,
		},
		{
			name:     "short hash is rejected",
			manifest: "deadbeef  focusd_setup.exe\n",
			asset:    installerAssetName,
			wantErr:  true,
		},
		{
			name:     "non hex is rejected",
			manifest: "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz  focusd_setup.exe\n",
			asset:    installerAssetName,
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseChecksumFor(tc.manifest, tc.asset)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("parseChecksumFor() = %q; want %q", got, tc.want)
			}
		})
	}
}

// TestParseChecksumForRefusesMissing guards the updater: without a verifiable
// checksum it must refuse rather than run an unverified installer.
func TestParseChecksumForRefusesMissing(t *testing.T) {
	if _, err := parseChecksumFor("", installerAssetName); err == nil {
		t.Error("expected an error for an empty manifest")
	}
}

func TestIsHex(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"0123456789abcdef", true},
		{"ABCDEF", true},
		{"", true},
		{"abcg", false},
		{"abc def", false},
		{"abc-", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if got := isHex(tc.input); got != tc.want {
				t.Errorf("isHex(%q) = %v; want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []int
	}{
		{"simple", "1.2.3", []int{1, 2, 3}},
		{"with v prefix", "v1.2.3", []int{1, 2, 3}},
		{"two parts", "2.13", []int{2, 13}},
		{"prerelease stripped", "1.2.3-rc.1", []int{1, 2, 3}},
		{"build metadata stripped", "1.2.3+deadbeef", []int{1, 2, 3}},
		{"non numeric stops parse", "1.2.beta", []int{1, 2}},
		{"empty", "", nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseVersion(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("parseVersion(%q) = %v; want %v", tc.input, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("parseVersion(%q)[%d] = %d; want %d", tc.input, i, got[i], tc.want[i])
				}
			}
		})
	}
}
