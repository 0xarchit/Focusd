package coreapi

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"focusd/storage"
	"focusd/system"
)

// ExportFormat identifies the supported usage file representation.
type ExportFormat string

const (
	ExportCSV  ExportFormat = "csv"
	ExportJSON ExportFormat = "json"
)

// ExportOptions exports all history when both dates are empty. Destination is
// an optional file path; the default is focusd_export.<format> in Downloads.
type ExportOptions struct {
	StartDate   string
	EndDate     string
	Format      ExportFormat
	Destination string
}

// ExportUsage writes usage atomically without exposing a partially written file.
func ExportUsage(options ExportOptions) (string, error) {
	path, err := exportPath(options)
	if err != nil {
		return "", err
	}
	stats, err := storage.GetAppStatsInRange(options.StartDate, options.EndDate)
	if err != nil {
		return "", fmt.Errorf("read export usage: %w", err)
	}
	stats = ApplyDisplayOverrides(stats)
	if err := system.WritePrivateFileAtomic(path, func(w io.Writer) error {
		return writeUsage(w, options.Format, stats)
	}); err != nil {
		return "", fmt.Errorf("write export: %w", err)
	}
	return path, nil
}

func exportPath(options ExportOptions) (string, error) {
	if options.Format != ExportCSV && options.Format != ExportJSON {
		return "", fmt.Errorf("unsupported export format %q", options.Format)
	}
	if (options.StartDate == "") != (options.EndDate == "") {
		return "", fmt.Errorf("provide both export range dates or neither")
	}
	if options.StartDate != "" {
		for _, value := range []string{options.StartDate, options.EndDate} {
			date, err := time.Parse("2006-01-02", value)
			if err != nil || date.Format("2006-01-02") != value {
				return "", fmt.Errorf("invalid export date %q; use YYYY-MM-DD", value)
			}
		}
		if options.StartDate > options.EndDate {
			return "", fmt.Errorf("export start date must not be after end date")
		}
	}
	path := options.Destination
	if path == "" {
		profile := os.Getenv("USERPROFILE")
		if profile == "" {
			return "", fmt.Errorf("USERPROFILE not set")
		}
		path = filepath.Join(profile, "Downloads", "focusd_export."+string(options.Format))
	}
	if !strings.EqualFold(filepath.Ext(path), "."+string(options.Format)) {
		return "", fmt.Errorf("export extension must match %s format", options.Format)
	}
	return filepath.Abs(path)
}

func writeUsage(w io.Writer, format ExportFormat, stats []storage.AppDailyStat) error {
	if format == ExportJSON {
		if stats == nil {
			stats = []storage.AppDailyStat{}
		}
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(stats)
	}
	if format != ExportCSV {
		return fmt.Errorf("unsupported export format %q", format)
	}
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"Date", "App Name", "Executable", "Duration (seconds)", "Open Count"}); err != nil {
		return err
	}
	for _, app := range stats {
		if err := writer.Write([]string{
			csvLiteralText(app.Date),
			csvLiteralText(app.AppName),
			csvLiteralText(app.ExeName),
			strconv.Itoa(app.TotalDurationSecs),
			strconv.Itoa(app.OpenCount),
		}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func csvLiteralText(value string) string {
	for _, r := range value {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "'" + value
		}
		if unicode.IsSpace(r) {
			continue
		}
		if strings.ContainsRune("=+-@", r) {
			return "'" + value
		}
		break
	}
	return value
}
