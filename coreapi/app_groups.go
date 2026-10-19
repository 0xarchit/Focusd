package coreapi

import (
	"focusd/storage"
	"regexp"
	"sort"
	"strings"
)

type groupedBrowserStat struct {
	Category   string
	TotalSecs  int
	SubEntries []subEntry
}

type subEntry struct {
	Title    string
	Duration int
}

var defaultBrowserTitles = map[string]bool{
	"new tab":         true,
	"new page":        true,
	"start page":      true,
	"home":            true,
	"blank page":      true,
	"speed dial":      true,
	"google chrome":   true,
	"mozilla firefox": true,
	"microsoft edge":  true,
	"zen browser":     true,
	"brave":           true,
	"opera":           true,
	"vivaldi":         true,
	"chrome":          true,
	"firefox":         true,
	"edge":            true,
}

// groupBrowserStats rolls site entries up into categories. An entry that is
// already a category name becomes its own group rather than a sub-entry.
func groupBrowserStats(stats []storage.AppDailyStat) []groupedBrowserStat {
	groups := make(map[string]*groupedBrowserStat)
	var order []string

	for _, stat := range stats {
		title := CollapseURLTitle(stat.AppName)
		duration := stat.TotalDurationSecs

		category := extractAppCategory(title)
		if category == "" {
			category = title
		}

		g, ok := groups[category]
		if !ok {
			g = &groupedBrowserStat{Category: category, TotalSecs: 0}
			groups[category] = g
			order = append(order, category)
		}
		g.TotalSecs += duration

		if category != title {
			g.SubEntries = append(g.SubEntries, subEntry{
				Title:    cleanTitleForDisplay(title),
				Duration: duration,
			})
		}
	}

	result := make([]groupedBrowserStat, 0, len(order))
	for _, cat := range order {
		result = append(result, *groups[cat])
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].TotalSecs != result[j].TotalSecs {
			return result[i].TotalSecs > result[j].TotalSecs
		}
		return result[i].Category < result[j].Category
	})

	return result
}

var separatorRegex = regexp.MustCompile(`\s*[-–—|·]\s*`)

func cleanTitleForDisplay(title string) string {
	parts := separatorRegex.Split(title, -1)
	if len(parts) <= 1 {
		return title
	}

	cleaned := strings.TrimSpace(parts[0])
	if cleaned == "" && len(parts) > 1 {
		cleaned = strings.TrimSpace(parts[1])
	}

	if cleaned == "" {
		return title
	}

	return cleaned
}
