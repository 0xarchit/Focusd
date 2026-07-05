package core

import (
	"focusd/storage"
	"log"
	"sort"
)

type DailySummary struct {
	Date         string
	TotalAppTime int
	AppCount     int
	TopApps      []storage.AppDailyStat
	TopSites     []storage.AppDailyStat
	GroupedSites []groupedBrowserStat
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

	summary := &DailySummary{
		Date:     date,
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

	sort.Slice(sites, func(i, j int) bool {
		return sites[i].TotalDurationSecs > sites[j].TotalDurationSecs
	})
	if len(sites) > 10 {
		summary.TopSites = sites[:10]
	} else {
		summary.TopSites = sites
	}

	summary.GroupedSites = groupBrowserStats(sites)

	return summary, nil
}
