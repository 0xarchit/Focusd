package core

import (
	"fmt"
	"focusd/storage"
	"log"
	"sort"
	"time"
)

type DailySummary struct {
	Date         string
	TotalAppTime int
	AppCount     int
	TopApps      []storage.AppDailyStat
	TopSites     []storage.AppDailyStat
	GroupedSites []GroupedBrowserStat
	RangeMessage string
	RangeStart   string
	RangeEnd     string
}

func GetDailySummary(date string) (*DailySummary, error) {
	apps, err := storage.GetAppStatsForDate(date)
	if err != nil {
		return nil, err
	}

	sites, err := storage.GetBrowserStatsForDate(date)
	if err != nil {
		log.Printf("WARN: failed to load browser stats for %s: %v", date, err)
	}

	summary := createSummary(date, apps)
	summary.TopSites = limitStats(sites, 10)
	summary.GroupedSites = GroupBrowserStats(sites)

	return summary, nil
}

func GetPeriodSummary(days int) (*DailySummary, error) {
	if days == 1 {
		return GetDailySummary(storage.Today())
	}

	allApps, err := storage.GetAllAppStats()
	if err != nil {
		return nil, err
	}

	allSites, err := storage.GetAllBrowserStats()
	if err != nil {
		log.Printf("WARN: failed to load all browser stats: %v", err)
	}

	cutoff := time.Now().AddDate(0, 0, -days+1).Format("2006-01-02")

	var minDate, maxDate string
	appMap := make(map[string]storage.AppDailyStat)
	siteMap := make(map[string]storage.AppDailyStat)

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

	for _, s := range allSites {
		if s.Date >= cutoff {
			existing, exists := siteMap[s.AppName]
			if !exists {
				existing = storage.AppDailyStat{AppName: s.AppName}
			}
			existing.TotalDurationSecs += s.TotalDurationSecs
			existing.OpenCount += s.OpenCount
			siteMap[s.AppName] = existing
		}
	}

	apps := mapToSlice(appMap)
	sites := mapToSlice(siteMap)

	label := "Last 30 Days"
	if days == 7 {
		label = "Last 7 Days"
	}

	summary := createSummary(label, apps)
	summary.TopSites = limitStats(sites, 10)
	summary.GroupedSites = GroupBrowserStats(sites)
	summary.RangeStart = minDate
	summary.RangeEnd = maxDate

	if minDate != "" && minDate > cutoff {
		summary.RangeMessage = fmt.Sprintf("Note: You are a new user. Displaying available data from %s to %s.", minDate, maxDate)
	} else if minDate != "" {
		summary.RangeMessage = fmt.Sprintf("Data period: %s to %s", minDate, maxDate)
	}

	return summary, nil
}

func mapToSlice(m map[string]storage.AppDailyStat) []storage.AppDailyStat {
	var s []storage.AppDailyStat
	for _, v := range m {
		s = append(s, v)
	}
	return s
}

func limitStats(stats []storage.AppDailyStat, limit int) []storage.AppDailyStat {
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].TotalDurationSecs > stats[j].TotalDurationSecs
	})
	if len(stats) > limit {
		return stats[:limit]
	}
	return stats
}

func createSummary(label string, apps []storage.AppDailyStat) *DailySummary {
	summary := &DailySummary{
		Date:     label,
		AppCount: len(apps),
	}

	for _, app := range apps {
		summary.TotalAppTime += app.TotalDurationSecs
	}

	summary.TopApps = limitStats(apps, 10)

	return summary
}
