package ui

import (
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/hussaratkuro/hyprsvn/internal/model"
)

// The diff viewer: side-by-side rows built from the unwrapped sources the
// state machine keeps, or a unified diff drawn line by line; a change map on
// the right, and Alt+Up/Down jumping between changes.

type diffView struct {
	source *model.SideBySideDiffSource
	rows   []model.DiffRow
	starts []int
	list   widget.List
	graph  graphView
	change int
}

func (a *App) diffRows() []model.DiffRow {
	src := a.m.diffSource
	if src == nil {
		return nil
	}
	if a.diff.source == src {
		return a.diff.rows
	}
	oldLines, oldCRLF := splitLinesWithCRLF(src.OldText)
	newLines, newCRLF := splitLinesWithCRLF(src.NewText)
	a.diff.source, a.diff.rows = src, sideBySideRows(oldLines, newLines, oldCRLF, newCRLF)
	a.diff.starts, a.diff.change = nil, -1
	prev := "="
	for i, r := range a.diff.rows {
		if r.Marker != "=" && prev == "=" {
			a.diff.starts = append(a.diff.starts, i)
		}
		prev = r.Marker
	}
	a.diff.list.Position = layout.Position{}
	return a.diff.rows
}

func (a *App) diffKey(name string) bool {
	d := &a.diff
	total := len(a.diffRows())
	if a.m.diffSource == nil {
		total = len(a.m.viewport.Lines())
	}
	page := 20
	switch name {
	case "alt+down", "alt+up":
		if len(d.starts) == 0 {
			return true
		}
		if name == "alt+down" {
			d.change = min(len(d.starts)-1, d.change+1)
		} else {
			d.change = max(0, d.change-1)
		}
		d.list.Position.First, d.list.Position.Offset = max(0, d.starts[d.change]-3), 0
	case "up", "k":
		d.list.Position.First = max(0, d.list.Position.First-1)
	case "down", "j":
		d.list.Position.First = min(max(0, total-1), d.list.Position.First+1)
	case "pgup":
		d.list.Position.First = max(0, d.list.Position.First-page)
	case "pgdown":
		d.list.Position.First = min(max(0, total-1), d.list.Position.First+page)
	case "home":
		d.list.Position.First, d.list.Position.Offset = 0, 0
	case "end":
		d.list.Position.First, d.list.Position.Offset = max(0, total-1), 0
	default:
		return false
	}
	return true
}

// Diff semantics keep fixed hues: on a monochrome wallbash palette the
// theme's green / red / yellow collapse to grey and the diff turns unreadable.
var (
	diffGreen  = color.NRGBA{R: 0xa6, G: 0xe3, B: 0xa1, A: 0xff}
	diffRed    = color.NRGBA{R: 0xf3, G: 0x8b, B: 0xa8, A: 0xff}
	diffYellow = color.NRGBA{R: 0xf9, G: 0xe2, B: 0xaf, A: 0xff}
	diffMauve  = color.NRGBA{R: 0xcb, G: 0xa6, B: 0xf7, A: 0xff}
)

func (a *App) markerColors(marker string) (bg, fg color.NRGBA) {
	switch marker {
	case "+":
		return alpha(diffGreen, 0x26), diffGreen
	case "-":
		return alpha(diffRed, 0x26), diffRed
	case "~":
		return alpha(diffYellow, 0x22), diffYellow
	}
	return color.NRGBA{}, a.pal.Subtext0
}

func (a *App) screenDiff(gtx C) D {
	m := &a.m
	title := "Side-by-side diff"
	if m.isBranchDiffAction() {
		title = m.branchDiffCtx.Title()
	}
	if m.err != nil {
		lines := m.viewport.Lines()
		return a.frame(gtx, "Diff failed", func(gtx C) D { return a.textLines(gtx, &a.diff.list, lines, a.classifySVN) }, hintDef{"Esc", "back"})
	}
	rows := a.diffRows()
	if rows == nil {
		lines := m.viewport.Lines()
		return a.frame(gtx, "Unified diff", a.listBody(a.small(fmt.Sprintf("%d lines", len(lines)), a.pal.Overlay0), func(gtx C) D {
			return a.textLines(gtx, &a.diff.list, lines, a.classifySVN)
		}, nil), hintDef{"PgUp", "page up"}, hintDef{"PgDn", "page down"}, hintDef{"Home", "top"}, hintDef{"End", "bottom"}, hintDef{"Esc", "back"})
	}
	src := m.diffSource
	numW := len(strconv.Itoa(max(len(rows), 1))) + 1
	var intro layout.Widget = func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Flexed(1, a.row(a.pillLabel(src.OldLabel, a.pal.Red), a.small(fmt.Sprintf("%d changes", len(a.diff.starts)), a.pal.Overlay0))),
			layout.Flexed(1, a.row(a.pillLabel(src.NewLabel, a.pal.Green), a.small(firstLine(src.Prefix), a.pal.Overlay0))),
		)
	}
	body := a.listBody(intro, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				gtx.Constraints.Min = gtx.Constraints.Max
				ls := material.List(a.th, &a.diff.list)
				ls.Indicator.Color = a.pal.Surface2
				ls.Track.Color = alpha(a.pal.Surface0, 0)
				return ls.Layout(gtx, len(rows), func(gtx C, i int) D { return a.diffRow(gtx, rows[i], numW) })
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx C) D { return a.changeMap(gtx, rows) }),
		)
	}, nil)
	return a.frame(gtx, title, body, hintDef{"Alt+↑", "prev change"}, hintDef{"Alt+↓", "next change"}, hintDef{"PgUp", "page up"}, hintDef{"PgDn", "page down"}, hintDef{"Home", "top"}, hintDef{"End", "bottom"}, hintDef{"Esc", "back"})
}

// diffRow draws one aligned pair of lines with number gutters and a marker.
func (a *App) diffRow(gtx C, r model.DiffRow, numW int) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	h := a.lineH(gtx)
	bg, fg := a.markerColors(r.Marker)
	if bg.A != 0 {
		paint.FillShape(gtx.Ops, bg, clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, h)}.Op())
	}
	num := func(n int, crlf bool) layout.Widget {
		s := strings.Repeat(" ", numW)
		if n > 0 {
			s = fmt.Sprintf("%*d", numW, n)
		}
		if !crlf {
			return a.codeLine(s+"   ", a.pal.Overlay0, false)
		}
		return a.row(a.codeLine(s, a.pal.Overlay0, false), a.small("CR", a.pal.Peach))
	}
	cell := func(text string, c color.NRGBA) layout.Widget {
		return func(gtx C) D {
			defer clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, h)}.Push(gtx.Ops).Pop()
			// Keep the code away from the number / CR gutter.
			return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, a.codeLine(text, c, false))
		}
	}
	left, right := a.pal.Text, a.pal.Text
	switch r.Marker {
	case "-":
		left, right = diffRed, a.pal.Overlay0
	case "+":
		left, right = a.pal.Overlay0, diffGreen
	case "~":
		left, right = diffYellow, diffYellow
	}
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(num(r.LeftNum, r.LeftCRLF)),
		layout.Flexed(1, cell(r.Left, left)),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Dp(24)
			return layout.Center.Layout(gtx, a.codeLine(r.Marker, fg, true))
		}),
		layout.Rigid(num(r.RightNum, r.RightCRLF)),
		layout.Flexed(1, cell(r.Right, right)),
	)
}

// changeMap is the whole-file overview strip with the viewport marked.
func (a *App) changeMap(gtx C, rows []model.DiffRow) D {
	w, h := gtx.Dp(10), gtx.Constraints.Max.Y
	paint.FillShape(gtx.Ops, a.pal.Surface0, clip.UniformRRect(image.Rectangle{Max: image.Pt(w, h)}, gtx.Dp(3)).Op(gtx.Ops))
	total := max(1, len(rows))
	for i, r := range rows {
		if r.Marker == "=" {
			continue
		}
		bg, _ := a.markerColors(r.Marker)
		bg.A = 0xFF
		y0 := i * h / total
		y1 := max(y0+1, (i+1)*h/total)
		paint.FillShape(gtx.Ops, bg, clip.Rect{Min: image.Pt(0, y0), Max: image.Pt(w, y1)}.Op())
	}
	first := a.diff.list.Position.First
	visible := max(1, gtx.Constraints.Max.Y/a.lineH(gtx))
	y0 := first * h / total
	y1 := min(h, (first+visible)*h/total)
	paint.FillShape(gtx.Ops, alpha(a.pal.Text, 0x40), clip.UniformRRect(image.Rectangle{Min: image.Pt(0, y0), Max: image.Pt(w, max(y0+gtx.Dp(4), y1))}, gtx.Dp(3)).Op(gtx.Ops))
	return D{Size: image.Pt(w, h)}
}
