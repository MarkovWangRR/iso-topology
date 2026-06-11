package iso25d

import "testing"

func TestCJKWidowControl(t *testing.T) {
	lines := wrapLine("实时动态信息流", 10, 52)
	for _, l := range lines {
		if len([]rune(l)) == 1 {
			t.Fatalf("single-rune orphan line %q in %v", l, lines)
		}
	}
}
