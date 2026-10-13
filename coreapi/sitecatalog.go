package coreapi

import (
	"regexp"
	"strings"
)

// siteRule describes one catalogue entry.
type siteRule struct {
	Name string
	// Domains match a host suffix, so "news.ycombinator.com" matches
	// "ycombinator.com" but "notycombinator.com" does not.
	Domains []string
	// Title patterns match the cleaned page title at token boundaries.
	Titles []*regexp.Regexp
	// Hosts match the full title as a bare domain, e.g. "x.com/foo".
	Hosts []string
	// Exact restricts title matching to the bare site name, for names that are
	// also ordinary English where a substring match gives nonsense.
	Exact bool
}

func titlePattern(expr string) *regexp.Regexp {
	return regexp.MustCompile(expr)
}

// siteCatalog is ordered from most specific to least, because the first match
// wins and a specific rule must beat a generic one.
var siteCatalog = []siteRule{
	{Name: "YouTube", Domains: []string{"youtube.com", "youtu.be"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\byoutube\b`),
	}},
	{Name: "GitHub", Domains: []string{"github.com", "githubusercontent.com", "github.io"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bgithub\b`),
		titlePattern(`(?i)/github/`),
	}},
	{Name: "GitLab", Domains: []string{"gitlab.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bgitlab\b`),
	}},
	{Name: "Stack Overflow", Domains: []string{"stackoverflow.com", "stackexchange.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bstack\s*overflow\b`),
		titlePattern(`(?i)\bstackoverflow\b`),
	}},
	{Name: "LeetCode", Domains: []string{"leetcode.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bleetcode\b`),
	}},
	{Name: "HackerRank", Domains: []string{"hackerrank.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bhackerrank\b`),
	}},
	{Name: "Codeforces", Domains: []string{"codeforces.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bcodeforces\b`),
	}},
	{Name: "Twitter/X", Domains: []string{"x.com", "twitter.com"}, Hosts: []string{"x.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\btwitter\b`),
	}},
	{Name: "Instagram", Domains: []string{"instagram.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\binstagram\b`),
	}},
	{Name: "Facebook", Domains: []string{"facebook.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bfacebook\b`),
	}},
	{Name: "Reddit", Domains: []string{"reddit.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\breddit\b`),
	}},
	{Name: "LinkedIn", Domains: []string{"linkedin.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\blinked\s*in\b`),
	}},
	{Name: "TikTok", Domains: []string{"tiktok.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\btik\s*tok\b`),
	}},
	{Name: "Netflix", Domains: []string{"netflix.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bnetflix\b`),
	}},
	{Name: "Prime Video", Domains: []string{"primevideo.com", "amazon.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bprime\s*video\b`),
	}},
	{Name: "Hotstar", Exact: true, Domains: []string{"hotstar.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bhotstar\b`),
	}},
	{Name: "JioCinema", Domains: []string{"jiocinema.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bjiocinema\b`),
	}},
	{Name: "Spotify", Domains: []string{"spotify.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bspotify\b`),
	}},
	{Name: "Twitch", Domains: []string{"twitch.tv"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\btwitch\b`),
	}},
	{Name: "Discord", Domains: []string{"discord.com", "discord.gg"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bdiscord\b`),
	}},
	{Name: "Slack", Domains: []string{"slack.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bslack\b`),
	}},
	{Name: "WhatsApp", Domains: []string{"web.whatsapp.com", "whatsapp.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bwhats\s*app\b`),
	}},
	{Name: "Telegram", Domains: []string{"web.telegram.org", "telegram.org"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\btelegram\b`),
	}},
	{Name: "Gmail", Domains: []string{"mail.google.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bgmail\b`),
	}},
	{Name: "Outlook", Domains: []string{"outlook.live.com", "outlook.office.com", "outlook.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\boutlook\b`),
	}},
	{Name: "Google Drive", Domains: []string{"drive.google.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bgoogle\s*drive\b`),
	}},
	{Name: "Google Docs", Domains: []string{"docs.google.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bgoogle\s*docs\b`),
	}},
	{Name: "Google Sheets", Domains: []string{"sheets.google.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bgoogle\s*sheets\b`),
	}},
	{Name: "Google Meet", Domains: []string{"meet.google.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bgoogle\s*meet\b`),
	}},
	{Name: "Microsoft Teams", Domains: []string{"teams.microsoft.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bmicrosoft\s*teams\b`),
	}},
	{Name: "Zoom", Exact: true, Domains: []string{"zoom.us"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bzoom\b`),
	}},
	{Name: "Notion", Exact: true, Domains: []string{"notion.so", "notion.site"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bnotion\b`),
	}},
	{Name: "Figma", Exact: true, Domains: []string{"figma.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bfigma\b`),
	}},
	{Name: "Canva", Exact: true, Domains: []string{"canva.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bcanva\b`),
	}},
	{Name: "Trello", Exact: true, Domains: []string{"trello.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\btrello\b`),
	}},
	{Name: "Asana", Exact: true, Domains: []string{"asana.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\basana\b`),
	}},
	{Name: "Jira", Exact: true, Domains: []string{"atlassian.net"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bjira\b`),
	}},
	{Name: "Medium", Exact: true, Domains: []string{"medium.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bmedium\b`),
	}},
	{Name: "Dev.to", Domains: []string{"dev.to"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bdev\.to\b`),
		titlePattern(`(?i)\bdev community\b`),
	}},
	{Name: "Quora", Exact: true, Domains: []string{"quora.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bquora\b`),
	}},
	{Name: "Pinterest", Domains: []string{"pinterest.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bpinterest\b`),
	}},
	{Name: "Snapchat", Domains: []string{"snapchat.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bsnapchat\b`),
	}},
	{Name: "Amazon", Domains: []string{"amazon.com", "amazon.in", "amazon.co.uk"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bamazon\b`),
	}},
	{Name: "Flipkart", Exact: true, Domains: []string{"flipkart.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bflipkart\b`),
	}},
	{Name: "Myntra", Exact: true, Domains: []string{"myntra.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bmyntra\b`),
	}},
	{Name: "Swiggy", Exact: true, Domains: []string{"swiggy.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bswiggy\b`),
	}},
	{Name: "Zomato", Exact: true, Domains: []string{"zomato.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bzomato\b`),
	}},
	{Name: "Uber", Exact: true, Domains: []string{"uber.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\buber\b`),
	}},
	{Name: "ChatGPT", Domains: []string{"chat.openai.com", "chatgpt.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bchat\s*gpt\b`),
	}},
	{Name: "Claude", Domains: []string{"claude.ai"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bclaude\b`),
	}},
	{Name: "Google Gemini", Domains: []string{"gemini.google.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bgemini\b`),
	}},
	{Name: "Perplexity", Exact: true, Domains: []string{"perplexity.ai"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bperplexity\b`),
	}},
	{Name: "MDN", Domains: []string{"developer.mozilla.org"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bmdn\b`),
		titlePattern(`(?i)\bdeveloper\.mozilla\b`),
	}},
	{Name: "VS Code", Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bvisual studio code\b`),
		titlePattern(`(?i)\bvscode\b`),
	}},
	{Name: "CodePen", Exact: true, Domains: []string{"codepen.io"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bcodepen\b`),
	}},
	{Name: "Replit", Exact: true, Domains: []string{"replit.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\breplit\b`),
	}},
	{Name: "Vercel", Exact: true, Domains: []string{"vercel.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bvercel\b`),
	}},
	{Name: "Netlify", Exact: true, Domains: []string{"netlify.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bnetlify\b`),
	}},
	{Name: "AWS", Exact: true, Domains: []string{"aws.amazon.com", "console.aws"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\baws\b`),
	}},
	{Name: "Google Cloud", Domains: []string{"console.cloud.google"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bgoogle cloud\b`),
	}},
	{Name: "Azure", Exact: true, Domains: []string{"portal.azure.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bazure\b`),
	}},
	{Name: "Coursera", Exact: true, Domains: []string{"coursera.org"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bcoursera\b`),
	}},
	{Name: "Udemy", Exact: true, Domains: []string{"udemy.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\budemy\b`),
	}},
	{Name: "Khan Academy", Domains: []string{"khanacademy.org"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bkhan academy\b`),
	}},
	{Name: "Wikipedia", Domains: []string{"wikipedia.org"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bwikipedia\b`),
	}},
	{Name: "DuckDuckGo", Domains: []string{"duckduckgo.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bduckduckgo\b`),
	}},
	{Name: "Bing", Exact: true, Domains: []string{"bing.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bbing\b`),
	}},
	{Name: "Google Search", Domains: []string{"google.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bgoogle search\b`),
	}},
	{Name: "Internal", Titles: []*regexp.Regexp{
		titlePattern(`(?i)\binternal (docs|wiki|tools)\b`),
	}},
	{Name: "Unstop", Exact: true, Domains: []string{"unstop.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bunstop\b`),
	}},
	{Name: "Naukri", Exact: true, Domains: []string{"naukri.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bnaukri\b`),
	}},
	{Name: "GeeksforGeeks", Domains: []string{"geeksforgeeks.org"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bgeeksforgeeks\b`),
	}},
	{Name: "W3Schools", Domains: []string{"w3schools.com"}, Titles: []*regexp.Regexp{
		titlePattern(`(?i)\bw3schools\b`),
	}},
}

// siteOverrides lets a user rename or add entries without touching the code.
var siteOverrides = map[string]string{}

// SetSiteOverride records a user label for a site key. An empty name removes it.
func SetSiteOverride(key, name string) {
	if strings.TrimSpace(name) == "" {
		delete(siteOverrides, key)
		return
	}
	siteOverrides[key] = strings.TrimSpace(name)
}

// ClearSiteOverrides resets the override table. Used by tests.
func ClearSiteOverrides() {
	siteOverrides = map[string]string{}
}

// domainOf extracts a bare host from text that may contain a scheme, path, or
// port.
func domainOf(text string) string {
	candidate := strings.TrimSpace(strings.ToLower(text))
	if i := strings.Index(candidate, "://"); i >= 0 {
		candidate = candidate[i+3:]
	}
	if i := strings.IndexAny(candidate, "/?#"); i >= 0 {
		candidate = candidate[:i]
	}
	if i := strings.Index(candidate, ":"); i >= 0 {
		candidate = candidate[:i]
	}
	return strings.Trim(candidate, ".")
}

// domainMatchesHost reports whether host is the rule domain or a subdomain of
// it. A plain suffix test would let "notyoutube.com" match "youtube.com".
func domainMatchesHost(host, domain string) bool {
	host = strings.Trim(host, ".")
	domain = strings.Trim(domain, ".")
	if host == domain {
		return true
	}
	return strings.HasSuffix(host, "."+domain)
}

// SiteNameFor returns the friendly name for a cleaned title or domain, or an
// empty string when nothing matches.
func SiteNameFor(titleOrDomain string) string {
	text := strings.TrimSpace(titleOrDomain)
	if text == "" {
		return ""
	}

	// A user override wins over everything.
	if override, ok := siteOverrides[strings.ToLower(text)]; ok {
		return override
	}
	if override, ok := siteOverrides[text]; ok {
		return override
	}

	host := domainOf(text)

	for i := range siteCatalog {
		rule := &siteCatalog[i]

		if host != "" {
			for _, d := range rule.Domains {
				if domainMatchesHost(host, d) {
					return rule.Name
				}
			}
			for _, h := range rule.Hosts {
				if host == h {
					return rule.Name
				}
			}
		}

		if rule.Exact {
			if sameSiteName(text, rule.Name) {
				return rule.Name
			}
			continue
		}

		for _, re := range rule.Titles {
			if re.MatchString(text) {
				return rule.Name
			}
		}
	}
	return ""
}

// siteSeparators are stripped before comparing a title to a site name, so
// "Slack |", "slack", and "Slack" all compare equal.
var siteSeparators = "-|·—_"

// sameSiteName compares a title to a site name ignoring case, spacing, and
// separators, so "Slack", "slack", and "Slack |" all compare equal.
func sameSiteName(title, name string) bool {
	normalize := func(s string) string {
		var b strings.Builder
		for _, r := range strings.ToLower(strings.TrimSpace(s)) {
			if r == ' ' || strings.ContainsRune(siteSeparators, r) {
				continue
			}
			b.WriteRune(r)
		}
		return b.String()
	}
	return normalize(title) == normalize(name)
}

// extractAppCategory returns the category for a stored site name, or an empty
// string when the name is already specific enough to display as-is.
func extractAppCategory(title string) string {
	if title == "" {
		return ""
	}

	trimmed := strings.TrimSpace(strings.ToLower(title))
	if defaultBrowserTitles[trimmed] {
		return "Browser (Idle)"
	}

	if name := SiteNameFor(title); name != "" {
		return name
	}
	return ""
}
