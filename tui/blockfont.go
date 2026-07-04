package tui

import "strings"

func RenderBlockTimer(mins, secs int) string {
	var minDigits []int
	if mins >= 100 {
		minDigits = []int{mins / 100, (mins / 10) % 10, mins % 10}
	} else {
		minDigits = []int{mins / 10, mins % 10}
	}

	s1, s2 := secs/10, secs%10

	var digitLines [][3]string
	for _, d := range minDigits {
		digitLines = append(digitLines, getDigitLines(d))
	}
	d3 := getDigitLines(s1)
	d4 := getDigitLines(s2)

	colon := [3]string{
		" ▄ ",
		"   ",
		" ▀ ",
	}

	var result []string
	for i := 0; i < 3; i++ {
		var rowParts []string
		for _, dl := range digitLines {
			rowParts = append(rowParts, dl[i])
		}
		row := strings.Join(rowParts, " ") + colon[i] + d3[i] + " " + d4[i]
		result = append(result, row)
	}

	return strings.Join(result, "\n")
}

func getDigitLines(d int) [3]string {
	switch d {
	case 0:
		return [3]string{
			"▄▄▄▄▄",
			"█   █",
			"▀▀▀▀▀",
		}
	case 1:
		return [3]string{
			"  ▄█ ",
			"   █ ",
			"  ▀▀▀",
		}
	case 2:
		return [3]string{
			"▄▄▄▄▄",
			" ▄▄▀ ",
			"▀▀▀▀▀",
		}
	case 3:
		return [3]string{
			"▀▀▀▀▄",
			" ▀▀▀█",
			"▄▄▄▄▀",
		}
	case 4:
		return [3]string{
			"█   █",
			"▀▀▀▀█",
			"    ▀",
		}
	case 5:
		return [3]string{
			"█▀▀▀▀",
			"▀▀▀▀█",
			"▄▄▄▄▀",
		}
	case 6:
		return [3]string{
			"▄▀▀▀▀",
			"█▀▀▀█",
			"▀▀▀▀▀",
		}
	case 7:
		return [3]string{
			"▀▀▀▀█",
			"  ▄▀ ",
			" █   ",
		}
	case 8:
		return [3]string{
			"█▀▀▀█",
			"█▀▀▀█",
			"█▄▄▄█",
		}
	case 9:
		return [3]string{
			"█▀▀▀█",
			"▀▀▀▀█",
			"▄▄▄▄▀",
		}
	default:
		return [3]string{
			" ▄▄  ",
			" ▀▀  ",
			"     ",
		}
	}
}
