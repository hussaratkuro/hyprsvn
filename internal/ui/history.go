package ui

import (
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/hussaratkuro/hyprsvn/internal/model"
)

// The commit history and file history screens: a commit list built from the
// structured log the loaders now return. The revision tree is in graph.go.

type historyView struct {
	key      string
	rows     []model.SVNLogEntryXML
	cursor   int
	expanded map[int]bool
	list     widget.List
	fullBtn  widget.Clickable
	diffBtn  widget.Clickable
	rowClick []widget.Clickable
}

// historyRows filters the loaded entries with the active search text.
func (a *App) historyRows() []model.SVNLogEntryXML {
	m := &a.m
	key := fmt.Sprintf("%d|%s|%t|%s|%d", m.selectedAction, m.historyTarget, m.historyFull, m.historySearch, len(m.historyEntries))
	if a.hist.key == key {
		return a.hist.rows
	}
	a.hist.key = key
	a.hist.cursor = 0
	a.hist.expanded = map[int]bool{}
	a.hist.list.Position = layout.Position{}
	query := strings.ToLower(strings.TrimSpace(m.historySearch))
	if query == "" {
		a.hist.rows = m.historyEntries
		return a.hist.rows
	}
	var rows []model.SVNLogEntryXML
	for _, e := range m.historyEntries {
		hay := strings.ToLower("r" + strconv.Itoa(e.Revision) + " " + e.Author + " " + e.Date + " " + e.Msg)
		for _, p := range e.Paths {
			hay += " " + strings.ToLower(p.Path)
		}
		if strings.Contains(hay, query) {
			rows = append(rows, e)
		}
	}
	a.hist.rows = rows
	return rows
}

func (a *App) historyKey(name string) bool {
	m := &a.m
	if m.selectedAction == model.ActionRevisionTree {
		return a.graphKey(name)
	}
	rows := a.historyRows()
	h := &a.hist
	switch name {
	case "up", "k":
		h.cursor = max(0, h.cursor-1)
	case "down", "j":
		h.cursor = min(max(0, len(rows)-1), h.cursor+1)
	case "pgup":
		h.cursor = max(0, h.cursor-10)
	case "pgdown":
		h.cursor = min(max(0, len(rows)-1), h.cursor+10)
	case "home":
		h.cursor = 0
	case "end":
		h.cursor = max(0, len(rows)-1)
	case "enter", " ":
		h.expanded[h.cursor] = !h.expanded[h.cursor]
	case "d":
		a.openCommitDiff()
	default:
		return false
	}
	return true
}

func (a *App) openCommitDiff() {
	rows := a.historyRows()
	if len(rows) == 0 {
		return
	}
	rev := rows[min(a.hist.cursor, len(rows)-1)].Revision
	a.m.screen, a.m.runningTitle = model.ScreenRunning, fmt.Sprintf("Loading diff for r%d...", rev)
	a.run(commitDiffCmd(a.m.activeRepo, rev))
}

func formatLogDate(d string) string {
	if len(d) >= 16 {
		return d[:10] + " " + d[11:16]
	}
	return d
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func (a *App) screenHistory(gtx C) D {
	m := &a.m
	if m.selectedAction == model.ActionRevisionTree {
		return a.screenGraph(gtx)
	}
	title := strings.TrimSpace(m.historyTitle)
	if title == "" {
		title = "Commit history"
	}
	if m.historyTarget != "" && m.historyTarget != "." {
		title += " · " + m.historyTarget
	}
	if m.err != nil {
		return a.frame(gtx, title, func(gtx C) D { return a.status(stripANSI(m.historyContent), "")(gtx) }, hintDef{"Esc", "back"})
	}
	rows := a.historyRows()
	h := &a.hist
	if len(h.rowClick) < len(rows) {
		h.rowClick = make([]widget.Clickable, len(rows))
	}
	for i := range rows {
		if click, ok := h.rowClick[i].Update(gtx); ok {
			h.cursor = i
			if click.NumClicks >= 2 {
				h.expanded[i] = !h.expanded[i]
			}
		}
	}
	if h.diffBtn.Clicked(gtx) {
		a.openCommitDiff()
	}
	var intro layout.Widget
	if m.screen == model.ScreenHistorySearch {
		intro = a.narrow(620, a.field("Search text", m.input.Editor(), "message, author, path or revision", true))
	} else {
		info := fmt.Sprintf("%d commits", len(rows))
		if strings.TrimSpace(m.historySearch) != "" {
			info = fmt.Sprintf("%d of %d commits match %q", len(rows), len(m.historyEntries), m.historySearch)
		}
		intro = a.row(a.small(info, a.pal.Overlay0), a.small("· enter or double click expands the changed paths", a.pal.Overlay0))
	}
	body := a.listBody(intro, func(gtx C) D {
		gtx.Constraints.Min = gtx.Constraints.Max
		if len(rows) == 0 {
			return layout.Center.Layout(gtx, a.text("No commits to show.", a.pal.Overlay1))
		}
		rowH := a.lineH(gtx) + gtx.Dp(10)
		a.keepCursorVisible(gtx, &h.list, h.cursor, rowH)
		ls := material.List(a.th, &h.list)
		ls.Indicator.Color = a.pal.Surface2
		ls.Track.Color = alpha(a.pal.Surface0, 0)
		return ls.Layout(gtx, len(rows), func(gtx C, i int) D {
			e := rows[i]
			return h.rowClick[i].Layout(gtx, func(gtx C) D {
				return a.commitRow(gtx, e, rowH, i == h.cursor, h.rowClick[i].Hovered(), h.expanded[i])
			})
		})
	}, nil)
	hints := []hintDef{{"d", "commit diff"}, {"/", "search"}, {"i", "info"}, {"Esc", "back"}, {"q", "quit"}}
	if m.screen == model.ScreenHistorySearch {
		hints = []hintDef{{"Enter", "search"}, {"Esc", "cancel"}}
	}
	return a.frame(gtx, title, body, hints...)
}

// commitRow is one commit: revision pill, author, date, message, and the
// changed paths when expanded.
func (a *App) commitRow(gtx C, e model.SVNLogEntryXML, rowH int, selected, hovered, expanded bool) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	r := rowDef{}
	if expanded {
		for _, p := range e.Paths {
			c := a.statusColor(p.Action)
			line := "      " + padRight(p.Action, 2) + p.Path
			if p.CopyFromPath != "" {
				line += fmt.Sprintf("  (from %s@%d)", p.CopyFromPath, p.CopyFromRev)
			}
			r.extra = append(r.extra, seg{line, c, false})
		}
		if len(e.Paths) == 0 {
			r.extra = append(r.extra, seg{"      (no changed paths recorded)", a.pal.Overlay0, false})
		}
		for _, l := range strings.Split(strings.TrimSpace(e.Msg), "\n")[1:] {
			r.extra = append(r.extra, seg{"      " + l, a.pal.Subtext0, false})
		}
	}
	msg := firstLine(e.Msg)
	if msg == "" {
		msg = "(no commit message)"
	}
	segs := []seg{{fmt.Sprintf("r%d", e.Revision), a.pal.Sapphire, true}, {padRight(e.Author, 12), a.pal.Subtext0, false}, {formatLogDate(e.Date), a.pal.Overlay1, false}, {msg, a.pal.Text, false}}
	if len(e.Paths) > 0 {
		segs = append(segs, seg{fmt.Sprintf("%d paths", len(e.Paths)), a.pal.Overlay0, false})
	}
	r.segs = segs
	return a.rowBox(gtx, r, rowH, selected, hovered)
}

// pill draws a small rounded label.
func (a *App) pillLabel(s string, c color.NRGBA) layout.Widget {
	return func(gtx C) D {
		return a.box(gtx, alpha(c, 0x2A), color.NRGBA{}, unit.Dp(99), layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2), Left: unit.Dp(8), Right: unit.Dp(8)}, func(gtx C) D {
			l := material.Label(a.th, unit.Sp(11), s)
			l.Color = c
			l.Font.Weight = font.Medium
			l.MaxLines = 1
			return l.Layout(gtx)
		})
	}
}

func fillCircle(gtx C, c color.NRGBA, center image.Point, r int) {
	paint.FillShape(gtx.Ops, c, clip.Ellipse{Min: image.Pt(center.X-r, center.Y-r), Max: image.Pt(center.X+r, center.Y+r)}.Op(gtx.Ops))
}
