package coreapi

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"focusd/storage"
	"focusd/system"
)

func TestCSVLiteralTextNeutralizesFormulaPrefixes(t *testing.T) {
	for _, value := range []string{"=SUM(1,2)", "+1", "-1", "@SUM(A1)", "\tformula", "\rformula", "\nformula", "  =1", "\u2003@value", "\ufeff=1", "\u200b@value"} {
		if got := csvLiteralText(value); got != "'"+value {
			t.Errorf("csvLiteralText(%q) = %q; want literal prefix", value, got)
		}
	}
	for _, value := range []string{"", "Firefox", "Café", "'=already literal", "normal = sentence", "123"} {
		if got := csvLiteralText(value); got != value {
			t.Errorf("csvLiteralText(%q) changed safe text to %q", value, got)
		}
	}
}

func TestWriteUsageCSVKeepsNumericFieldsAndQuotesText(t *testing.T) {
	stats := []storage.AppDailyStat{{Date: "2026-11-03", AppName: "=formula,\n記事本", ExeName: " @app", TotalDurationSecs: 123, OpenCount: 2}}
	var output bytes.Buffer
	if err := writeUsage(&output, ExportCSV, stats); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&output).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || len(rows[1]) != 5 {
		t.Fatalf("unexpected CSV shape: %v", rows)
	}
	if rows[1][1] != "'=formula,\n記事本" || rows[1][2] != "' @app" || rows[1][3] != "123" || rows[1][4] != "2" {
		t.Fatalf("CSV values = %q", rows[1])
	}
}

func TestWriteUsageJSONPreservesLiteralValues(t *testing.T) {
	stats := []storage.AppDailyStat{{Date: "2026-11-03", AppName: "=literal", AppID: "app|hash", TotalDurationSecs: 123}}
	var output bytes.Buffer
	if err := writeUsage(&output, ExportJSON, stats); err != nil {
		t.Fatal(err)
	}
	var decoded []storage.AppDailyStat
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 1 || decoded[0].AppName != "=literal" || decoded[0].AppID != "app|hash" || decoded[0].TotalDurationSecs != 123 {
		t.Fatalf("JSON values = %#v", decoded)
	}
}

type failedExportWriter struct{}

func (failedExportWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestWriteUsagePropagatesOutputFailure(t *testing.T) {
	for _, format := range []ExportFormat{ExportCSV, ExportJSON} {
		if err := writeUsage(failedExportWriter{}, format, nil); !errors.Is(err, io.ErrClosedPipe) {
			t.Errorf("%s output error = %v", format, err)
		}
	}
}

func TestExportOptionsRejectInvalidRangeAndFormatBeforeStorage(t *testing.T) {
	for _, options := range []ExportOptions{
		{Format: "xml"},
		{Format: ExportCSV, StartDate: "2026-11-03"},
		{Format: ExportJSON, StartDate: "2026-11-04", EndDate: "2026-11-03"},
		{Format: ExportCSV, StartDate: "2026-02-30", EndDate: "2026-11-03"},
	} {
		if _, err := ExportUsage(options); err == nil {
			t.Errorf("ExportUsage(%+v) unexpectedly succeeded", options)
		}
	}
	if _, err := ExportUsage(ExportOptions{Format: ExportCSV, Destination: "report.json"}); err == nil || !strings.Contains(err.Error(), "extension") {
		t.Fatalf("format mismatch error = %v", err)
	}
}

func TestExportUsageSelectsRangeAndAppliesDisplayOverride(t *testing.T) {
	setupPomodoroEnv(t)
	for _, date := range []string{"2026-10-10", "2026-10-11"} {
		start, err := time.Parse("2006-01-02", date)
		if err != nil {
			t.Fatal(err)
		}
		if err := storage.InsertSessionWithDaily(&storage.Session{
			AppName: "Original", ExeName: "test.exe", AppID: "test|export",
			StartTime: start, EndTime: start.Add(time.Minute),
			DurationSecs: 60, Date: date,
		}, ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := system.SetDisplayOverride("test|export", "My App"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "nested", "range.json")
	result, err := ExportUsage(ExportOptions{
		StartDate: "2026-10-11", EndDate: "2026-10-11",
		Format: ExportJSON, Destination: path,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != path {
		t.Fatalf("returned path = %q; want %q", result, path)
	}
	content, err := os.ReadFile(result)
	if err != nil {
		t.Fatal(err)
	}
	var stats []storage.AppDailyStat
	if err := json.Unmarshal(content, &stats); err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 || stats[0].Date != "2026-10-11" || stats[0].AppName != "My App" || stats[0].AppID != "test|export" || stats[0].TotalDurationSecs != 60 {
		t.Fatalf("selected export rows = %#v", stats)
	}
}
