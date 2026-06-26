package core

import (
	"focusd/storage"
	"regexp"
	"sort"
	"strings"
)

type appPattern struct {
	Name     string
	Patterns []string
}

var appPatterns = []appPattern{
	{Name: "YouTube", Patterns: []string{"youtube.com", "youtu.be", "- youtube", "| youtube"}},
	{Name: "GitHub", Patterns: []string{"github.com", "· github", "- github", "| github", "/github"}},
	{Name: "GitLab", Patterns: []string{"gitlab.com", "- gitlab", "| gitlab"}},
	{Name: "Instagram", Patterns: []string{"instagram.com", "- instagram", "| instagram"}},
	{Name: "Twitter/X", Patterns: []string{"twitter.com", "/ x", "| x", "- x"}},
	{Name: "Facebook", Patterns: []string{"facebook.com", "- facebook", "| facebook"}},
	{Name: "Reddit", Patterns: []string{"reddit.com", "- reddit", "| reddit"}},
	{Name: "LinkedIn", Patterns: []string{"linkedin.com", "- linkedin", "| linkedin"}},
	{Name: "TikTok", Patterns: []string{"tiktok.com", "- tiktok", "| tiktok"}},
	{Name: "Netflix", Patterns: []string{"netflix.com", "- netflix", "| netflix"}},
	{Name: "Prime Video", Patterns: []string{"primevideo.com", "amazon.com/gp/video", "- prime video"}},
	{Name: "Hotstar", Patterns: []string{"hotstar.com", "- hotstar", "disney+ hotstar"}},
	{Name: "JioCinema", Patterns: []string{"jiocinema.com", "- jiocinema"}},
	{Name: "Spotify", Patterns: []string{"spotify.com", "- spotify", "| spotify"}},
	{Name: "Twitch", Patterns: []string{"twitch.tv", "- twitch", "| twitch"}},
	{Name: "Discord", Patterns: []string{"discord.com", "- discord", "| discord"}},
	{Name: "Slack", Patterns: []string{"slack.com", "- slack", "| slack"}},
	{Name: "WhatsApp", Patterns: []string{"web.whatsapp.com", "- whatsapp"}},
	{Name: "Telegram", Patterns: []string{"web.telegram.org", "- telegram"}},
	{Name: "Gmail", Patterns: []string{"mail.google.com", "- gmail", "| gmail", "inbox -"}},
	{Name: "Outlook", Patterns: []string{"outlook.live.com", "outlook.office.com", "- outlook"}},
	{Name: "Google Drive", Patterns: []string{"drive.google.com", "- google drive"}},
	{Name: "Google Docs", Patterns: []string{"docs.google.com", "- google docs"}},
	{Name: "Google Sheets", Patterns: []string{"sheets.google.com", "- google sheets"}},
	{Name: "Google Meet", Patterns: []string{"meet.google.com", "- google meet"}},
	{Name: "Zoom", Patterns: []string{"zoom.us", "- zoom", "| zoom"}},
	{Name: "Microsoft Teams", Patterns: []string{"teams.microsoft.com", "- microsoft teams", "| teams"}},
	{Name: "Notion", Patterns: []string{"notion.so", "- notion", "| notion"}},
	{Name: "Figma", Patterns: []string{"figma.com", "- figma", "| figma"}},
	{Name: "Canva", Patterns: []string{"canva.com", "- canva", "| canva"}},
	{Name: "Trello", Patterns: []string{"trello.com", "- trello", "| trello"}},
	{Name: "Asana", Patterns: []string{"asana.com", "- asana", "| asana"}},
	{Name: "Jira", Patterns: []string{"atlassian.net", "- jira", "| jira"}},
	{Name: "StackOverflow", Patterns: []string{"stackoverflow.com", "- stack overflow"}},
	{Name: "LeetCode", Patterns: []string{"leetcode.com", "- leetcode", "| leetcode"}},
	{Name: "HackerRank", Patterns: []string{"hackerrank.com", "- hackerrank"}},
	{Name: "Codeforces", Patterns: []string{"codeforces.com", "- codeforces"}},
	{Name: "Medium", Patterns: []string{"medium.com", "- medium", "| medium"}},
	{Name: "Dev.to", Patterns: []string{"dev.to", "- dev community"}},
	{Name: "Quora", Patterns: []string{"quora.com", "- quora", "| quora"}},
	{Name: "Pinterest", Patterns: []string{"pinterest.com", "- pinterest", "| pinterest"}},
	{Name: "Snapchat", Patterns: []string{"snapchat.com", "- snapchat"}},
	{Name: "Amazon", Patterns: []string{"amazon.in", "amazon.com", "- amazon"}},
	{Name: "Flipkart", Patterns: []string{"flipkart.com", "- flipkart"}},
	{Name: "Myntra", Patterns: []string{"myntra.com", "- myntra"}},
	{Name: "Swiggy", Patterns: []string{"swiggy.com", "- swiggy"}},
	{Name: "Zomato", Patterns: []string{"zomato.com", "- zomato"}},
	{Name: "Uber", Patterns: []string{"uber.com", "- uber"}},
	{Name: "Ola", Patterns: []string{"olacabs.com", "- ola"}},
	{Name: "ChatGPT", Patterns: []string{"chat.openai.com", "chatgpt.com", "- chatgpt"}},
	{Name: "Claude", Patterns: []string{"claude.ai", "- claude"}},
	{Name: "Google Gemini", Patterns: []string{"gemini.google.com", "- gemini"}},
	{Name: "Perplexity", Patterns: []string{"perplexity.ai", "- perplexity"}},
	{Name: "Unstop", Patterns: []string{"unstop.com", "// unstop"}},
	{Name: "Internshala", Patterns: []string{"internshala.com", "- internshala"}},
	{Name: "Naukri", Patterns: []string{"naukri.com", "- naukri"}},
	{Name: "GeeksforGeeks", Patterns: []string{"geeksforgeeks.org", "- geeksforgeeks"}},
	{Name: "W3Schools", Patterns: []string{"w3schools.com", "- w3schools"}},
	{Name: "MDN", Patterns: []string{"developer.mozilla.org", "- mdn"}},
	{Name: "VS Code", Patterns: []string{"- visual studio code", "vscode"}},
	{Name: "CodePen", Patterns: []string{"codepen.io", "- codepen"}},
	{Name: "Replit", Patterns: []string{"replit.com", "- replit"}},
	{Name: "Vercel", Patterns: []string{"vercel.com", "- vercel"}},
	{Name: "Netlify", Patterns: []string{"netlify.com", "- netlify"}},
	{Name: "AWS", Patterns: []string{"aws.amazon.com", "console.aws", "- aws"}},
	{Name: "Google Cloud", Patterns: []string{"console.cloud.google", "- google cloud"}},
	{Name: "Azure", Patterns: []string{"portal.azure.com", "- azure"}},
	{Name: "Coursera", Patterns: []string{"coursera.org", "- coursera"}},
	{Name: "Udemy", Patterns: []string{"udemy.com", "- udemy"}},
	{Name: "Khan Academy", Patterns: []string{"khanacademy.org", "- khan academy"}},
	{Name: "Wikipedia", Patterns: []string{"wikipedia.org", "- wikipedia"}},
	{Name: "Google Search", Patterns: []string{"google.com/search", "- google search"}},
	{Name: "Bing", Patterns: []string{"bing.com/search", "- bing"}},
	{Name: "DuckDuckGo", Patterns: []string{"duckduckgo.com", "- duckduckgo"}},
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
}

func ExtractAppCategory(title string) string {
	if title == "" {
		return ""
	}

	titleLower := strings.ToLower(title)

	if defaultBrowserTitles[titleLower] {
		return "Browser (Idle)"
	}

	// x.com exact-host check before first-match loop to avoid catching
	// unrelated domains that happen to contain the substring "x.com/".
	if titleLower == "x.com" ||
		strings.HasPrefix(titleLower, "x.com/") ||
		strings.Contains(titleLower, "://x.com/") {
		return "Twitter/X"
	}

	for i := range appPatterns {
		app := &appPatterns[i]
		for _, pattern := range app.Patterns {
			if strings.Contains(titleLower, strings.ToLower(pattern)) {
				return app.Name
			}
		}
	}

	return ""
}

type GroupedBrowserStat struct {
	Category   string
	TotalSecs  int
	SubEntries []SubEntry
}

type SubEntry struct {
	Title    string
	Duration int
}

func GroupBrowserStats(stats []storage.AppDailyStat) []GroupedBrowserStat {
	groups := make(map[string]*GroupedBrowserStat)
	var order []string

	for _, stat := range stats {
		title := stat.AppName
		duration := stat.TotalDurationSecs

		category := ExtractAppCategory(title)

		if category == "" {
			category = title
		}

		if g, ok := groups[category]; ok {
			g.TotalSecs += duration
			if category != title {
				cleanTitle := cleanTitleForDisplay(title)
				g.SubEntries = append(g.SubEntries, SubEntry{
					Title:    cleanTitle,
					Duration: duration,
				})
			}
		} else {
			order = append(order, category)
			g := &GroupedBrowserStat{
				Category:  category,
				TotalSecs: duration,
			}
			if category != title {
				cleanTitle := cleanTitleForDisplay(title)
				g.SubEntries = []SubEntry{{
					Title:    cleanTitle,
					Duration: duration,
				}}
			}
			groups[category] = g
		}
	}

	result := make([]GroupedBrowserStat, 0, len(order))
	for _, cat := range order {
		result = append(result, *groups[cat])
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalSecs > result[j].TotalSecs
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
