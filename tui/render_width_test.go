package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// truncate must never return more than the requested number of cells: the
// ellipsis is part of the budget, not an extra cell. Every caller (padRight,
// padLeft, center, fitLine) pads to an exact width afterwards, and a
// one-cell overflow misaligns whole table columns.
func TestTruncateKeepsWithinLimit(t *testing.T) {
	cases := []struct {
		s     string
		limit int
	}{
		{"abcdef", 3},
		{"hello world", 5},
		{"aaa", 3},
		{"日本語テキスト", 3},
		{"abc", 0},
		{"", 3},
	}
	for _, c := range cases {
		if got := truncate(c.s, c.limit); lipgloss.Width(got) > c.limit {
			t.Errorf("truncate(%q, %d) = %q (width %d), exceeds limit", c.s, c.limit, got, lipgloss.Width(got))
		}
	}
	if got := truncate("aaa", 3); got != "aaa" {
		t.Errorf("exact fit was rewritten: truncate(%q, 3) = %q", "aaa", got)
	}
}

// A long tab title must not shift that row's Time column. Every line of the
// browser usage table is expected to be exactly width-4 cells wide with the
// durations flush right underneath the "Time" header.
func TestBrowserUsageRowsStayUniform(t *testing.T) {
	m := Model{stats: tuiData{Browsers: []appUsage{
		{Name: "focusd v3.0.0: app identity, audit remediation, notification fixes by 0xarchit · Pull Request #19 · 0xarchit/Focusd", Duration: 2904},
		{Name: "0x", Duration: 204},
		{Name: "0xarchit (Archit Jain)", Duration: 169},
		{Name: "App Router", Duration: 154},
		{Name: "ireze — One Edge for Every Frontier Model", Duration: 55},
		{Name: "hey admins team can we get some model updates like cheaper models like deepseek v4 fla", Duration: 39},
		{Name: "Github", Duration: 35},
		{Name: "Commits · 0xarchit/Focusd", Duration: 34},
		{Name: "0xarchit/Focusd at feature/v3-stability", Duration: 29},
		{Name: "LLM Coding Leaderboard — AI Coding Daily", Duration: 28},
	}}}

	for _, width := range []int{104, 118, 160} {
		out := m.renderBrowserUsage(width, 10)
		lines := strings.Split(out, "\n")
		for i, line := range lines {
			if got := lipgloss.Width(line); got != width-4 {
				t.Errorf("width=%d line %d: width %d, want %d: %q", width, i, got, width-4, stripANSI(line))
			}
		}
		if hdr := stripANSI(lines[0]); !strings.HasSuffix(hdr, "Time") {
			t.Errorf("width=%d header does not end with \"Time\": %q", width, hdr)
		}
		if t.Failed() {
			continue
		}
		for i, line := range lines {
			t.Logf("[%3d] %s", lipgloss.Width(line), stripANSI(line))
			_ = i
		}
	}
}
