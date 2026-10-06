package ui

import (
	"testing"

	"github.com/hussaratkuro/hyprsvn/internal/theme"
)

func TestAutoDiffThemeFallsBackForMonochromePalette(t *testing.T) {
	t.Setenv("TUI_DIFF_THEME", "")
	p := theme.CatppuccinMocha()
	g := theme.Hex("#B2B2B2")
	p.Green, p.Red, p.Yellow, p.Mauve = g, g, g, g

	got, appearance := resolveDiffPalette(p)
	want := theme.CatppuccinMocha()
	if appearance != diffAppearanceColor || got.added != lg(want.Green) || got.deleted != lg(want.Red) || got.modified != lg(want.Yellow) || got.conflict != lg(want.Mauve) {
		t.Fatalf("auto monochrome palette = %#v / %v, want semantic fallback", got, appearance)
	}
}

func TestWallbashAndMonoDiffOverrides(t *testing.T) {
	p := theme.CatppuccinMocha()
	g := theme.Hex("#AAAAAA")
	p.Green, p.Red, p.Yellow, p.Mauve, p.Text = g, g, g, g, theme.Hex("#EEEEEE")

	t.Setenv("TUI_DIFF_THEME", "wallbash")
	got, appearance := resolveDiffPalette(p)
	if appearance != diffAppearanceColor || got.added != lg(p.Green) || got.deleted != lg(p.Red) {
		t.Fatalf("wallbash override did not preserve theme colors: %#v / %v", got, appearance)
	}

	t.Setenv("TUI_DIFF_THEME", "mono")
	got, appearance = resolveDiffPalette(p)
	if appearance != diffAppearanceMono || got.added != lg(p.Text) || got.deleted != lg(p.Text) || got.modified != lg(p.Text) || got.conflict != lg(p.Text) {
		t.Fatalf("mono override = %#v / %v", got, appearance)
	}
}
