package core

import (
	"fmt"
	"focusd/storage"
	"sort"
	"time"
)

type DailySummary struct {
	Date         string
	TotalAppTime int
	AppCount     int
	TopApps      []storage.AppDailyStat
	RangeMessage string
	RangeStart   string
	RangeEnd     string
}

func GetDailySummary(date string) (*DailySummary, error) {
	apps, err := storage.GetAppStatsForDate(date)
	if err != nil {
		return nil, err
	}

	return createSummary(date, apps), nil
}

func GetPeriodSummary(days int) (*DailySummary, error) {
	if days == 1 {
		return GetDailySummary(storage.Today())
	}

	allApps, err := storage.GetAllAppStats()
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -days+1).Format("2006-01-02")

	var minDate, maxDate string
	appMap := make(map[string]storage.AppDailyStat)

	for _, s := range allApps {
		if s.Date >= cutoff {
			if minDate == "" || s.Date < minDate {
				minDate = s.Date
			}
			if maxDate == "" || s.Date > maxDate {
				maxDate = s.Date
			}

			existing, exists := appMap[s.ExeName]
			if !exists {
				existing = storage.AppDailyStat{ExeName: s.ExeName, AppName: s.AppName}
			}
			existing.TotalDurationSecs += s.TotalDurationSecs
			existing.OpenCount += s.OpenCount
			appMap[s.ExeName] = existing
		}
	}

	var apps []storage.AppDailyStat
	for _, s := range appMap {
		apps = append(apps, s)
	}

	label := "Last 30 Days"
	if days == 7 {
		label = "Last 7 Days"
	}

	summary := createSummary(label, apps)
	summary.RangeStart = minDate
	summary.RangeEnd = maxDate

	if minDate != "" && minDate > cutoff {
		summary.RangeMessage = fmt.Sprintf("Note: You are a new user. Displaying available data from %s to %s.", minDate, maxDate)
	} else if minDate != "" {
		summary.RangeMessage = fmt.Sprintf("Data period: %s to %s", minDate, maxDate)
	}

	return summary, nil
}

func createSummary(label string, apps []storage.AppDailyStat) *DailySummary {
	summary := &DailySummary{
		Date:     label,
		AppCount: len(apps),
	}

	for _, app := range apps {
		summary.TotalAppTime += app.TotalDurationSecs
	}

	sort.Slice(apps, func(i, j int) bool {
		return apps[i].TotalDurationSecs > apps[j].TotalDurationSecs
	})

	if len(apps) > 10 {
		summary.TopApps = apps[:10]
	} else {
		summary.TopApps = apps
	}

	return summary
}

func GetWeeklySummary() ([]DailySummary, error) {
	var summaries []DailySummary
	for i := 0; i < 7; i++ {
		date := getDateNDaysAgo(i)
		summary, err := GetDailySummary(date)
		if err != nil {
			continue
		}
		if summary.AppCount > 0 {
			summaries = append(summaries, *summary)
		}
	}
	return summaries, nil
}

func getDateNDaysAgo(n int) string {
	return time.Now().AddDate(0, 0, -n).Format("2006-01-02")
}
