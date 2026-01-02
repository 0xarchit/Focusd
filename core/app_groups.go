package core

import (
	"regexp"
	"sort"
	"strings"
)

type appPattern struct {
	Name     string
	Patterns []string
	Priority int
}

var appPatterns = []appPattern{
	{Name: "YouTube", Patterns: []string{"youtube.com", "youtu.be", "- youtube", "| youtube"}, Priority: 100},
	{Name: "GitHub", Patterns: []string{"github.com", "· github", "- github", "| github", "/github"}, Priority: 100},
	{Name: "GitLab", Patterns: []string{"gitlab.com", "- gitlab", "| gitlab"}, Priority: 100},
	{Name: "Instagram", Patterns: []string{"instagram.com", "- instagram", "| instagram"}, Priority: 100},
	{Name: "Twitter/X", Patterns: []string{"twitter.com", "x.com/", "/ x", "| x", "- x"}, Priority: 100},
	{Name: "Facebook", Patterns: []string{"facebook.com", "- facebook", "| facebook"}, Priority: 100},
	{Name: "Reddit", Patterns: []string{"reddit.com", "- reddit", "| reddit"}, Priority: 100},
	{Name: "LinkedIn", Patterns: []string{"linkedin.com", "- linkedin", "| linkedin"}, Priority: 100},
	{Name: "TikTok", Patterns: []string{"tiktok.com", "- tiktok", "| tiktok"}, Priority: 100},
	{Name: "Netflix", Patterns: []string{"netflix.com", "- netflix", "| netflix"}, Priority: 100},
	{Name: "Prime Video", Patterns: []string{"primevideo.com", "amazon.com/gp/video", "- prime video"}, Priority: 100},
	{Name: "Hotstar", Patterns: []string{"hotstar.com", "- hotstar", "disney+ hotstar"}, Priority: 100},
	{Name: "JioCinema", Patterns: []string{"jiocinema.com", "- jiocinema"}, Priority: 100},
	{Name: "Spotify", Patterns: []string{"spotify.com", "- spotify", "| spotify"}, Priority: 100},
	{Name: "Twitch", Patterns: []string{"twitch.tv", "- twitch", "| twitch"}, Priority: 100},
	{Name: "Discord", Patterns: []string{"discord.com", "- discord", "| discord"}, Priority: 100},
	{Name: "Slack", Patterns: []string{"slack.com", "- slack", "| slack"}, Priority: 100},
	{Name: "WhatsApp", Patterns: []string{"web.whatsapp.com", "- whatsapp"}, Priority: 100},
	{Name: "Telegram", Patterns: []string{"web.telegram.org", "- telegram"}, Priority: 100},
	{Name: "Gmail", Patterns: []string{"mail.google.com", "- gmail", "| gmail", "inbox -"}, Priority: 100},
	{Name: "Outlook", Patterns: []string{"outlook.live.com", "outlook.office.com", "- outlook"}, Priority: 100},
	{Name: "Google Drive", Patterns: []string{"drive.google.com", "- google drive"}, Priority: 100},
	{Name: "Google Docs", Patterns: []string{"docs.google.com", "- google docs"}, Priority: 100},
	{Name: "Google Sheets", Patterns: []string{"sheets.google.com", "- google sheets"}, Priority: 100},
	{Name: "Google Meet", Patterns: []string{"meet.google.com", "- google meet"}, Priority: 100},
	{Name: "Zoom", Patterns: []string{"zoom.us", "- zoom", "| zoom"}, Priority: 100},
	{Name: "Microsoft Teams", Patterns: []string{"teams.microsoft.com", "- microsoft teams", "| teams"}, Priority: 100},
	{Name: "Notion", Patterns: []string{"notion.so", "- notion", "| notion"}, Priority: 100},
	{Name: "Figma", Patterns: []string{"figma.com", "- figma", "| figma"}, Priority: 100},
	{Name: "Canva", Patterns: []string{"canva.com", "- canva", "| canva"}, Priority: 100},
	{Name: "Trello", Patterns: []string{"trello.com", "- trello", "| trello"}, Priority: 100},
	{Name: "Asana", Patterns: []string{"asana.com", "- asana", "| asana"}, Priority: 100},
	{Name: "Jira", Patterns: []string{"atlassian.net", "- jira", "| jira"}, Priority: 100},
	{Name: "StackOverflow", Patterns: []string{"stackoverflow.com", "- stack overflow"}, Priority: 100},
	{Name: "LeetCode", Patterns: []string{"leetcode.com", "- leetcode", "| leetcode"}, Priority: 100},
	{Name: "HackerRank", Patterns: []string{"hackerrank.com", "- hackerrank"}, Priority: 100},
	{Name: "Codeforces", Patterns: []string{"codeforces.com", "- codeforces"}, Priority: 100},
	{Name: "Medium", Patterns: []string{"medium.com", "- medium", "| medium"}, Priority: 100},
	{Name: "Dev.to", Patterns: []string{"dev.to", "- dev community"}, Priority: 100},
	{Name: "Quora", Patterns: []string{"quora.com", "- quora", "| quora"}, Priority: 100},
	{Name: "Pinterest", Patterns: []string{"pinterest.com", "- pinterest", "| pinterest"}, Priority: 100},
	{Name: "Snapchat", Patterns: []string{"snapchat.com", "- snapchat"}, Priority: 100},
	{Name: "Amazon", Patterns: []string{"amazon.in", "amazon.com", "- amazon"}, Priority: 90},
	{Name: "Flipkart", Patterns: []string{"flipkart.com", "- flipkart"}, Priority: 100},
	{Name: "Myntra", Patterns: []string{"myntra.com", "- myntra"}, Priority: 100},
	{Name: "Swiggy", Patterns: []string{"swiggy.com", "- swiggy"}, Priority: 100},
	{Name: "Zomato", Patterns: []string{"zomato.com", "- zomato"}, Priority: 100},
	{Name: "Uber", Patterns: []string{"uber.com", "- uber"}, Priority: 100},
	{Name: "Ola", Patterns: []string{"olacabs.com", "- ola"}, Priority: 100},
	{Name: "ChatGPT", Patterns: []string{"chat.openai.com", "chatgpt.com", "- chatgpt"}, Priority: 100},
	{Name: "Claude", Patterns: []string{"claude.ai", "- claude"}, Priority: 100},
	{Name: "Google Gemini", Patterns: []string{"gemini.google.com", "- gemini"}, Priority: 100},
	{Name: "Perplexity", Patterns: []string{"perplexity.ai", "- perplexity"}, Priority: 100},
	{Name: "Unstop", Patterns: []string{"unstop.com", "// unstop"}, Priority: 100},
	{Name: "Internshala", Patterns: []string{"internshala.com", "- internshala"}, Priority: 100},
	{Name: "Naukri", Patterns: []string{"naukri.com", "- naukri"}, Priority: 100},
	{Name: "GeeksforGeeks", Patterns: []string{"geeksforgeeks.org", "- geeksforgeeks"}, Priority: 100},
	{Name: "W3Schools", Patterns: []string{"w3schools.com", "- w3schools"}, Priority: 100},
	{Name: "MDN", Patterns: []string{"developer.mozilla.org", "- mdn"}, Priority: 100},
	{Name: "VS Code", Patterns: []string{"- visual studio code", "vscode"}, Priority: 100},
	{Name: "CodePen", Patterns: []string{"codepen.io", "- codepen"}, Priority: 100},
	{Name: "Replit", Patterns: []string{"replit.com", "- replit"}, Priority: 100},
	{Name: "Vercel", Patterns: []string{"vercel.com", "- vercel"}, Priority: 100},
	{Name: "Netlify", Patterns: []string{"netlify.com", "- netlify"}, Priority: 100},
	{Name: "AWS", Patterns: []string{"aws.amazon.com", "console.aws", "- aws"}, Priority: 100},
	{Name: "Google Cloud", Patterns: []string{"console.cloud.google", "- google cloud"}, Priority: 100},
	{Name: "Azure", Patterns: []string{"portal.azure.com", "- azure"}, Priority: 100},
	{Name: "Coursera", Patterns: []string{"coursera.org", "- coursera"}, Priority: 100},
	{Name: "Udemy", Patterns: []string{"udemy.com", "- udemy"}, Priority: 100},
	{Name: "Khan Academy", Patterns: []string{"khanacademy.org", "- khan academy"}, Priority: 100},
	{Name: "Wikipedia", Patterns: []string{"wikipedia.org", "- wikipedia"}, Priority: 100},
	{Name: "Google Search", Patterns: []string{"google.com/search", "- google search"}, Priority: 90},
	{Name: "Bing", Patterns: []string{"bing.com/search", "- bing"}, Priority: 90},
	{Name: "DuckDuckGo", Patterns: []string{"duckduckgo.com", "- duckduckgo"}, Priority: 100},
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

	var bestMatch *appPattern

	for i := range appPatterns {
		app := &appPatterns[i]
		for _, pattern := range app.Patterns {
			if strings.Contains(titleLower, strings.ToLower(pattern)) {
				if bestMatch == nil || app.Priority > bestMatch.Priority {
					bestMatch = app
				}
				break
			}
		}
	}

	if bestMatch != nil {
		return bestMatch.Name
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

func GroupBrowserStats(stats []struct {
	Title    string
	Duration int
}) []GroupedBrowserStat {
	groups := make(map[string]*GroupedBrowserStat)
	var order []string

	for _, stat := range stats {
		category := ExtractAppCategory(stat.Title)

		if category == "" {
			category = stat.Title
		}

		if g, ok := groups[category]; ok {
			g.TotalSecs += stat.Duration
			if category != stat.Title {
				cleanTitle := cleanTitleForDisplay(stat.Title)
				g.SubEntries = append(g.SubEntries, SubEntry{
					Title:    cleanTitle,
					Duration: stat.Duration,
				})
			}
		} else {
			order = append(order, category)
			g := &GroupedBrowserStat{
				Category:  category,
				TotalSecs: stat.Duration,
			}
			if category != stat.Title {
				cleanTitle := cleanTitleForDisplay(stat.Title)
				g.SubEntries = []SubEntry{{
					Title:    cleanTitle,
					Duration: stat.Duration,
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
