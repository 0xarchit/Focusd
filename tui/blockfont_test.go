package tui

import (
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestBlockTimerDisplaysEveryMinuteDigit(t *testing.T) {
	for _, minutes := range []int{99, 100, 999, 1000, 1440} {
		lines := strings.Split(renderBlockTimer(minutes, 7), "\n")
		wantDigits := strconv.Itoa(minutes)
		if len(wantDigits) < 2 {
			wantDigits = "0" + wantDigits
		}
		var firstRow []string
		for _, digit := range wantDigits {
			firstRow = append(firstRow, getDigitLines(int(digit - '0'))[0])
		}
		want := strings.Join(firstRow, " ") + " ▄ " + getDigitLines(0)[0] + " " + getDigitLines(7)[0]
		if lines[0] != want {
			t.Errorf("%d minutes first row = %q; want all digits %q", minutes, lines[0], want)
		}
		for _, line := range lines {
			if lipgloss.Width(line) > 76 {
				t.Errorf("%d minutes exceeds the 80-column focus panel: %d cells", minutes, lipgloss.Width(line))
			}
		}
	}
}
