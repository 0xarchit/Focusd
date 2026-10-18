package tui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// These tests pin the mouse hit regions to the frame that is actually drawn:
// every clickable region must cover the label or panel it claims to, using
// bubbletea's zero-based screen coordinates. They are the regression guard for
// the hand-approximated offsets the regions used to carry.

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

func indexAll(s, sub string) []int {
	var out []int
	for i := 0; ; {
		j := strings.Index(s[i:], sub)
		if j < 0 {
			return out
		}
		out = append(out, i+j)
		i += j + len(sub)
	}
}

// renderFrame renders one tab at a fixed size with a sandboxed config dir and
// returns the model plus the ANSI-stripped frame lines.
func renderFrame(t *testing.T, tab int) (*Model, []string) {
	t.Helper()
	t.Setenv("APPDATA", t.TempDir())
	m := NewModel()
	m.splash = false
	m.activeTab = tab
	m.width = 100
	m.height = 40
	lines := strings.Split(m.View(), "\n")
	clean := make([]string, len(lines))
	for i, l := range lines {
		clean[i] = stripANSI(l)
	}
	return &m, clean
}

// labelPos returns the frame row and display column of the first occurrence
// of label, or (-1, -1) when it is absent.
func labelPos(lines []string, label string) (row, col int) {
	for i, l := range lines {
		if j := strings.Index(l, label); j >= 0 {
			return i, lipgloss.Width(l[:j])
		}
	}
	return -1, -1
}

func regionByID(m *Model, id string) *clickableRegion {
	for i := range m.clickableRegions {
		if m.clickableRegions[i].ID == id {
			return &m.clickableRegions[i]
		}
	}
	return nil
}

// panelBorderRows lists the rows carrying a panel's top-left corner.
func panelBorderRows(lines []string, minRow int) []int {
	var rows []int
	for i, l := range lines {
		if i >= minRow && strings.Contains(l, "╭") {
			rows = append(rows, i)
		}
	}
	return rows
}

// lastPanelBottom returns the last row carrying a panel's bottom-left corner.
func lastPanelBottom(lines []string, minRow int) int {
	last := -1
	for i, l := range lines {
		if i >= minRow && strings.Contains(l, "╰") {
			last = i
		}
	}
	return last
}

// panelRight returns the display column just past the right border drawn on a
// panel's top border row.
func panelRight(line string) int {
	if j := strings.LastIndex(line, "╮"); j >= 0 {
		return lipgloss.Width(line[:j]) + 1
	}
	return -1
}

func checkRegionCoversLabel(t *testing.T, m *Model, id string, lines []string, label string) {
	t.Helper()
	row, col := labelPos(lines, label)
	if row < 0 {
		t.Fatalf("label %q not found in the frame", label)
	}
	reg := regionByID(m, id)
	if reg == nil {
		t.Fatalf("hit region %s is missing", id)
	}
	wantX2 := col + lipgloss.Width(label)
	if reg.Y1 != row || reg.Y2 != row+1 {
		t.Errorf("%s covers rows [%d,%d); label %q renders on row %d", id, reg.Y1, reg.Y2, label, row)
	}
	if reg.X1 != col || reg.X2 != wantX2 {
		t.Errorf("%s covers columns [%d,%d); label %q renders at [%d,%d)", id, reg.X1, reg.X2, label, col, wantX2)
	}
}

func TestTabHitRegionsMatchTheFrame(t *testing.T) {
	m, lines := renderFrame(t, tabDashboard)
	for i, name := range tabNames {
		label := fmt.Sprintf("[%d] %s", i+1, name)
		row, col := labelPos(lines, label)
		if row < 0 {
			t.Fatalf("label %q not found in the frame", label)
		}
		reg := regionByID(m, fmt.Sprintf("tab-%d", i))
		if reg == nil {
			t.Fatalf("hit region tab-%d is missing", i)
		}
		// The region deliberately includes the tab bar's top border row, so
		// the label only needs to fall inside it.
		if row < reg.Y1 || row >= reg.Y2 {
			t.Errorf("tab %s label renders on row %d; the hit region covers rows [%d,%d)", label, row, reg.Y1, reg.Y2)
		}
		if col != reg.X1 || reg.X2 != col+lipgloss.Width(label) {
			t.Errorf("tab %s hit columns [%d,%d); label renders at [%d,%d)", label, reg.X1, reg.X2, col, col+lipgloss.Width(label))
		}
	}
}

func TestFocusHitRegionsMatchTheFrame(t *testing.T) {
	m, lines := renderFrame(t, tabFocus)
	for i, label := range []string{"[ ▶ START ]", "[ ■ STOP ]", "[ ⟳ RESET ]"} {
		checkRegionCoversLabel(t, m, fmt.Sprintf("focus-btn-%d", i), lines, label)
	}

	top := -1
	for i, l := range lines {
		if strings.Contains(l, "╭─ FOCUS TIMER") {
			top = i
			break
		}
	}
	if top < 0 {
		t.Fatal("focus panel top border not found")
	}
	bottom := lastPanelBottom(lines, m.contentTop)
	reg := regionByID(m, "panel-0")
	if reg == nil {
		t.Fatal("focus panel region is missing")
	}
	if reg.Y1 != top || reg.Y2 != bottom+1 {
		t.Errorf("panel-0 covers rows [%d,%d); the panel is drawn on rows [%d,%d]", reg.Y1, reg.Y2, top, bottom)
	}
	if right := panelRight(lines[top]); right >= 0 && reg.X2 != right {
		t.Errorf("panel-0 ends at column %d; the panel's right border is drawn at column %d", reg.X2, right)
	}
}

func TestStatsHitRegionsMatchTheFrame(t *testing.T) {
	m, lines := renderFrame(t, tabStats)
	for i, label := range []string{"[ TODAY ]", "[ LAST 7 DAYS ]", "[ LAST 30 DAYS ]", "[ CUSTOM RANGE ]"} {
		checkRegionCoversLabel(t, m, fmt.Sprintf("range-%d", i), lines, label)
	}

	borders := panelBorderRows(lines, m.contentTop)
	if len(borders) < 3 {
		t.Fatalf("expected three stacked panels in the stats frame, found %d", len(borders))
	}
	reg := regionByID(m, "panel-0")
	if reg.Y1 != borders[0] || reg.Y2 != borders[1] {
		t.Errorf("panel-0 covers rows [%d,%d); its panel is drawn on rows [%d,%d]", reg.Y1, reg.Y2, borders[0], borders[1]-1)
	}
	if reg = regionByID(m, "panel-1"); reg.Y1 != borders[1] || reg.Y2 != borders[2] {
		t.Errorf("panel-1 covers rows [%d,%d); its panel is drawn on rows [%d,%d]", reg.Y1, reg.Y2, borders[1], borders[2]-1)
	}
	bottom := lastPanelBottom(lines, m.contentTop)
	if reg = regionByID(m, "panel-3"); reg.Y1 != borders[2] || reg.Y2 != bottom+1 {
		t.Errorf("panel-3 covers rows [%d,%d); its panel is drawn on rows [%d,%d]", reg.Y1, reg.Y2, borders[2], bottom)
	}
}

func TestDashboardHitRegionsMatchTheFrame(t *testing.T) {
	m, lines := renderFrame(t, tabDashboard)
	borders := panelBorderRows(lines, m.contentTop)
	if len(borders) < 3 {
		t.Fatalf("expected three rows of panels in the dashboard frame, found %d", len(borders))
	}

	var topCols []int
	for _, j := range indexAll(lines[borders[0]], "╭") {
		topCols = append(topCols, lipgloss.Width(lines[borders[0]][:j]))
	}
	if len(topCols) != 3 {
		t.Fatalf("expected three side-by-side top panels, found %d", len(topCols))
	}
	for i, want := range topCols {
		reg := regionByID(m, fmt.Sprintf("panel-%d", i))
		if reg == nil {
			t.Fatalf("panel-%d region is missing", i)
		}
		if reg.X1 != want {
			t.Errorf("panel-%d starts at column %d; its border is drawn at column %d", i, reg.X1, want)
		}
		if reg.Y1 != borders[0] {
			t.Errorf("panel-%d covers rows from %d; the panels are drawn from row %d", i, reg.Y1, borders[0])
		}
	}

	reg := regionByID(m, "panel-0")
	if reg.Y2 != borders[1]-1 {
		t.Errorf("panel-0 covers rows through %d; the next panel row starts at %d", reg.Y2-1, borders[1])
	}
	if reg = regionByID(m, "panel-3"); reg.Y1 != borders[1] {
		t.Errorf("panel-3 covers rows from %d; its panel is drawn from row %d", reg.Y1, borders[1])
	}
	p4 := regionByID(m, "panel-4")
	if p4 == nil {
		t.Fatal("panel-4 region is missing")
	}
	if p4.Y1 != borders[2] {
		t.Errorf("panel-4 covers rows from %d; its panel is drawn from row %d", p4.Y1, borders[2])
	}
	if reg.Y2 != p4.Y1-1 {
		t.Errorf("panel-3 ends at row %d; panel-4 starts at row %d", reg.Y2, p4.Y1)
	}
	// The dashboard fills the body at this size; the last panel may be
	// clipped, but its region must stay inside the visible body area.
	bodyEnd := m.contentTop + m.contentHeight()
	if p4.Y1 >= p4.Y2 || p4.Y2 > bodyEnd {
		t.Errorf("panel-4 covers rows [%d,%d); the visible body ends at row %d", p4.Y1, p4.Y2, bodyEnd)
	}
}

func TestLimitsHitRegionsMatchTheFrame(t *testing.T) {
	m, lines := renderFrame(t, tabLimits)
	borders := panelBorderRows(lines, m.contentTop)
	if len(borders) == 0 {
		t.Fatal("limits panel not found")
	}
	reg := regionByID(m, "panel-0")
	if reg == nil {
		t.Fatal("panel-0 region is missing")
	}
	bottom := lastPanelBottom(lines, m.contentTop)
	if reg.Y1 != borders[0] || reg.Y2 != bottom+1 {
		t.Errorf("panel-0 covers rows [%d,%d); the panel is drawn on rows [%d,%d]", reg.Y1, reg.Y2, borders[0], bottom)
	}
	if right := panelRight(lines[borders[0]]); right >= 0 && reg.X2 != right {
		t.Errorf("panel-0 ends at column %d; the panel's right border is drawn at column %d", reg.X2, right)
	}
}

func TestSettingsHitRegionsMatchTheFrame(t *testing.T) {
	m, lines := renderFrame(t, tabSettings)
	borders := panelBorderRows(lines, m.contentTop)
	if len(borders) == 0 {
		t.Fatal("settings panel not found")
	}
	reg := regionByID(m, "panel-0")
	if reg == nil {
		t.Fatal("panel-0 region is missing")
	}
	if reg.Y1 != borders[0] {
		t.Errorf("panel-0 covers rows from %d; the panel is drawn from row %d", reg.Y1, borders[0])
	}
	if right := panelRight(lines[borders[0]]); right >= 0 && reg.X2 != right {
		t.Errorf("panel-0 ends at column %d; the panel's right border is drawn at column %d", reg.X2, right)
	}
}
