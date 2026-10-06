package ui

import "testing"

func TestWordJump(t *testing.T) {
	r := []rune("hello   brave world")
	for _, tc := range []struct{ pos, dir, want int }{
		{19, -1, 14}, {14, -1, 8}, {8, -1, 0}, {0, -1, 0},
		{0, 1, 5}, {5, 1, 13}, {13, 1, 19}, {19, 1, 19},
		{10, -1, 8}, {10, 1, 13},
	} {
		if got := wordJump(r, tc.pos, tc.dir); got != tc.want {
			t.Errorf("wordJump(%d, %d) = %d, want %d", tc.pos, tc.dir, got, tc.want)
		}
	}
}
