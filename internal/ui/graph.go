package ui

import (
	"fmt"
	"image"
	"image/color"
	"slices"
	"sort"
	"strings"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/hussaratkuro/hyprsvn/internal/model"
)

// The revision tree as a GitKraken-style graph: one row per commit (newest
// first), the trunk in lane 0, every branch and tag in a lane that is only
// occupied between its oldest and newest loaded commit, so lanes are reused
// and a repository with hundreds of branches still fits.

type graphRow struct {
	rev    int
	lane   int
	node   *model.RevisionBranchNode
	commit model.RevisionBranchCommit
	first  bool  // newest loaded commit of the node
	last   bool  // oldest loaded commit of the node
	from   int   // lane of the node this one was copied from at its oldest row, -1 = none
	merges []int // lanes merged into this node at this row
}

// laneSpan is one node's occupation of a lane: alive for lo ≤ rev ≤ hi.
type laneSpan struct {
	lo, hi int
	node   *model.RevisionBranchNode
}

type graphView struct {
	key      string
	rows     []graphRow
	lanes    [][]laneSpan
	cursor   int
	list     widget.List
	rowClick []widget.Clickable
}

func (a *App) graphRows() *graphView {
	m := &a.m
	g := &a.diff.graph
	key := fmt.Sprintf("%t|%d|%s", m.historyFull, len(m.historyEntries), m.historySearch)
	if g.key == key {
		return g
	}
	g.key, g.cursor = key, 0
	g.list.Position = layout.Position{}
	g.rows, g.lanes = nil, nil
	graph := buildRevisionBranchGraph(m.historyEntries)

	// Nodes with commits, their loaded revision span, trunk first then
	// newest first.
	type nodeSpan struct {
		node    *model.RevisionBranchNode
		lo, hi  int
		commits []model.RevisionBranchCommit
	}
	query := strings.ToLower(strings.TrimSpace(m.historySearch))
	var spans []*nodeSpan
	for _, n := range graph.Nodes {
		if n.Kind == "branches" || n.Kind == "tags" { // container directories
			continue
		}
		var commits []model.RevisionBranchCommit
		for _, c := range n.Commits {
			if query != "" && !strings.Contains(strings.ToLower(fmt.Sprintf("r%d %s %s %s %s", c.Revision, c.Author, c.Date, c.Msg, n.Name)), query) {
				continue
			}
			commits = append(commits, c)
		}
		if len(commits) == 0 {
			continue
		}
		sort.Slice(commits, func(i, j int) bool { return commits[i].Revision > commits[j].Revision })
		spans = append(spans, &nodeSpan{node: n, lo: commits[len(commits)-1].Revision, hi: commits[0].Revision, commits: commits})
	}
	// A merged-back node's lane stays alive up to the merge row, so the
	// branch line runs from its tip up into the merge commit.
	has := map[string]bool{}
	for _, sp := range spans {
		has[sp.node.Path] = true
	}
	for _, sp := range spans {
		for _, mb := range sp.node.MergeBacks {
			if has[mb.Target] && mb.Rev > sp.hi {
				sp.hi = mb.Rev
			}
		}
	}
	sort.Slice(spans, func(i, j int) bool {
		ti, tj := spans[i].node.Path == "/trunk", spans[j].node.Path == "/trunk"
		if ti != tj {
			return ti
		}
		if spans[i].hi != spans[j].hi {
			return spans[i].hi > spans[j].hi
		}
		return spans[i].node.Path < spans[j].node.Path
	})

	// Lane assignment: sweeping from the newest span, a lane is free when
	// its last occupant ends above this span (no overlap). The trunk keeps
	// lane 0 for the whole height.
	laneOf := map[string]int{}
	var laneLo []int // oldest revision still occupied per lane
	addLane := func(lo int) int {
		laneLo = append(laneLo, lo)
		g.lanes = append(g.lanes, nil)
		return len(laneLo) - 1
	}
	for _, sp := range spans {
		lane := -1
		if sp.node.Path == "/trunk" {
			if len(laneLo) == 0 {
				addLane(0)
			}
			lane, laneLo[0] = 0, 0
		} else {
			if len(laneLo) == 0 { // no trunk in the log: keep lane 0 empty
				addLane(0)
			}
			for i := 1; i < len(laneLo); i++ {
				if laneLo[i] > sp.hi {
					lane = i
					break
				}
			}
			if lane < 0 {
				lane = addLane(sp.lo)
			}
			laneLo[lane] = sp.lo
		}
		laneOf[sp.node.Path] = lane
		g.lanes[lane] = append(g.lanes[lane], laneSpan{lo: sp.lo, hi: sp.hi, node: sp.node})
	}

	// Merge-backs by target lane and revision.
	mergeAt := map[int]map[int][]int{}
	for _, sp := range spans {
		for _, mb := range sp.node.MergeBacks {
			tl, ok := laneOf[mb.Target]
			if !ok {
				continue
			}
			if mergeAt[tl] == nil {
				mergeAt[tl] = map[int][]int{}
			}
			mergeAt[tl][mb.Rev] = append(mergeAt[tl][mb.Rev], laneOf[sp.node.Path])
		}
	}
	for _, sp := range spans {
		lane := laneOf[sp.node.Path]
		for i, c := range sp.commits {
			row := graphRow{rev: c.Revision, lane: lane, node: sp.node, commit: c, first: i == 0, last: i == len(sp.commits)-1, from: -1}
			if row.last && sp.node.CreatedFrom != "" {
				if fl, ok := laneOf[sp.node.CreatedFrom]; ok {
					row.from = fl
				}
			}
			row.merges = mergeAt[lane][c.Revision]
			g.rows = append(g.rows, row)
		}
	}
	sort.SliceStable(g.rows, func(i, j int) bool { return g.rows[i].rev > g.rows[j].rev })
	return g
}

// laneAlive reports whether lane is occupied at rev (trunk always is).
func (g *graphView) laneAlive(lane, rev int) bool {
	return g.laneSpanAt(lane, rev) != nil
}

// laneAbove reports whether the lane occupant at rev continues past it,
// i.e. the lane line must run out of the top of rev's row.
func (g *graphView) laneAbove(lane, rev int) bool {
	sp := g.laneSpanAt(lane, rev)
	return sp != nil && sp.hi > rev
}

func (g *graphView) laneSpanAt(lane, rev int) *laneSpan {
	if lane == 0 {
		return &laneSpan{lo: 0, hi: int(^uint(0) >> 1)}
	}
	for i := range g.lanes[lane] {
		if sp := &g.lanes[lane][i]; rev >= sp.lo && rev <= sp.hi {
			return sp
		}
	}
	return nil
}

func (a *App) graphKey(name string) bool {
	g := a.graphRows()
	switch name {
	case "up", "k":
		g.cursor = max(0, g.cursor-1)
	case "down", "j":
		g.cursor = min(max(0, len(g.rows)-1), g.cursor+1)
	case "pgup":
		g.cursor = max(0, g.cursor-10)
	case "pgdown":
		g.cursor = min(max(0, len(g.rows)-1), g.cursor+10)
	case "home":
		g.cursor = 0
	case "end":
		g.cursor = max(0, len(g.rows)-1)
	case "d", "enter":
		if len(g.rows) > 0 {
			rev := g.rows[min(g.cursor, len(g.rows)-1)].rev
			a.m.screen, a.m.runningTitle = model.ScreenRunning, fmt.Sprintf("Loading diff for r%d...", rev)
			a.run(commitDiffCmd(a.m.activeRepo, rev))
		}
	default:
		return false
	}
	return true
}

// laneColor keeps its hues on a monochrome wallbash palette too: the graph
// is unreadable when every lane is grey.
func (a *App) laneColor(lane int) color.NRGBA {
	cols := []color.NRGBA{{0xcb, 0xa6, 0xf7, 0xff}, {0x74, 0xc7, 0xec, 0xff}, {0xa6, 0xe3, 0xa1, 0xff}, {0xfa, 0xb3, 0x87, 0xff}, {0xf5, 0xc2, 0xe7, 0xff}, {0x94, 0xe2, 0xd5, 0xff}, {0xf9, 0xe2, 0xaf, 0xff}, {0xb4, 0xbe, 0xfe, 0xff}, {0xf3, 0x8b, 0xa8, 0xff}, {0x89, 0xdc, 0xeb, 0xff}}
	return cols[lane%len(cols)]
}

func isTag(n *model.RevisionBranchNode) bool { return strings.HasPrefix(n.Kind, "tag") }

func (a *App) screenGraph(gtx C) D {
	m := &a.m
	g := a.graphRows()
	if len(g.rowClick) < len(g.rows) {
		g.rowClick = make([]widget.Clickable, len(g.rows))
	}
	for i := range g.rows {
		if click, ok := g.rowClick[i].Update(gtx); ok {
			g.cursor = i
			if click.NumClicks >= 2 {
				a.graphKey("d")
			}
		}
	}
	if a.hist.fullBtn.Clicked(gtx) && !m.historyFull {
		a.sendKey("a")
	}
	title := "Revision tree"
	mode := fmt.Sprintf("newest %d entries", len(m.historyEntries))
	if m.historyFull {
		mode = fmt.Sprintf("full history · %d entries", len(m.historyEntries))
	}
	var intro layout.Widget
	if m.screen == model.ScreenHistorySearch {
		intro = a.narrow(620, a.field("Search text", m.input.Editor(), "message, author, branch or revision", true))
	} else {
		items := []layout.Widget{a.small(fmt.Sprintf("%s · %d lanes · %d commits", mode, len(g.lanes), len(g.rows)), a.pal.Overlay0)}
		if !m.historyFull {
			items = append(items, a.btn(&a.hist.fullBtn, icoRefresh, "Load full history", btnPrimary))
		}
		intro = a.row(items...)
	}
	if m.err != nil {
		msg := stripANSI(m.historyContent)
		if strings.TrimSpace(msg) == "" {
			msg = m.err.Error()
		}
		return a.frame(gtx, title, func(gtx C) D { return a.status(msg, "")(gtx) }, hintDef{"Esc", "back"})
	}
	body := a.listBody(intro, func(gtx C) D {
		gtx.Constraints.Min = gtx.Constraints.Max
		if len(g.rows) == 0 {
			return layout.Center.Layout(gtx, a.text("No trunk/branch/tag activity in the loaded log.", a.pal.Overlay1))
		}
		rowH := a.lineH(gtx) + gtx.Dp(12)
		a.keepCursorVisible(gtx, &g.list, g.cursor, rowH)
		ls := material.List(a.th, &g.list)
		ls.Indicator.Color = a.pal.Surface2
		ls.Track.Color = alpha(a.pal.Surface0, 0)
		return ls.Layout(gtx, len(g.rows), func(gtx C, i int) D {
			return g.rowClick[i].Layout(gtx, func(gtx C) D { return a.graphRow(gtx, g, i, rowH) })
		})
	}, nil)
	hints := []hintDef{{"d", "commit diff"}, {"/", "search"}, {"i", "info"}, {"Esc", "back"}, {"q", "quit"}}
	if m.screen == model.ScreenHistorySearch {
		hints = []hintDef{{"Enter", "search"}, {"Esc", "cancel"}}
	}
	return a.frame(gtx, title, body, hints...)
}

// graphRow draws the lanes for one commit row and the commit's text.
func (a *App) graphRow(gtx C, g *graphView, i, rowH int) D {
	row := g.rows[i]
	gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, rowH)
	// Lanes get at most 40% of the row; with many lanes they get thinner.
	laneW := gtx.Dp(18)
	if maxW := gtx.Constraints.Max.X * 4 / 10; laneW*len(g.lanes) > maxW {
		laneW = max(gtx.Dp(5), maxW/max(1, len(g.lanes)))
	}
	graphW := laneW*len(g.lanes) + gtx.Dp(12)
	selected := i == g.cursor
	size := image.Pt(gtx.Constraints.Max.X, rowH)
	if selected {
		paint.FillShape(gtx.Ops, a.pal.Surface0, clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(6)).Op(gtx.Ops))
	} else if g.rowClick[i].Hovered() {
		paint.FillShape(gtx.Ops, alpha(a.pal.Surface0, 0x80), clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(6)).Op(gtx.Ops))
	}
	cx := func(lane int) float32 { return float32(gtx.Dp(6) + laneW*lane + laneW/2) }
	midY := float32(rowH) / 2
	stroke := float32(gtx.Dp(2))
	if laneW < gtx.Dp(10) {
		stroke = float32(gtx.Dp(1))
	}
	line := func(x0, y0, x1, y1 float32, c color.NRGBA) {
		var p clip.Path
		p.Begin(gtx.Ops)
		p.MoveTo(f32.Pt(x0, y0))
		p.LineTo(f32.Pt(x1, y1))
		paint.FillShape(gtx.Ops, c, clip.Stroke{Path: p.End(), Width: stroke}.Op())
	}
	// Vertical lane segments: whole height while the lane is occupied at
	// this revision; a node's newest row starts at the dot unless it is
	// merged back further up, its oldest row ends at the dot, and a lane
	// that ends by merging into this row only gets the diagonal.
	for lane := range g.lanes {
		if !g.laneAlive(lane, row.rev) {
			continue
		}
		top, bottom := float32(0), float32(rowH)
		if lane == row.lane && lane != 0 {
			if row.first && !g.laneAbove(lane, row.rev) {
				top = midY
			}
			if row.last {
				bottom = midY
			}
		} else if slices.Contains(row.merges, lane) && !g.laneAbove(lane, row.rev) {
			continue
		}
		line(cx(lane), top, cx(lane), bottom, alpha(a.laneColor(lane), 0xB0))
	}
	// Branch-out from the parent lane below this row into the new node.
	if row.last && row.from >= 0 && row.from != row.lane {
		line(cx(row.from), float32(rowH), cx(row.lane), midY, alpha(a.laneColor(row.lane), 0xB0))
	}
	// Merge-backs: the source lane is older, so it comes in from below.
	for _, src := range row.merges {
		if src == row.lane {
			continue
		}
		line(cx(src), float32(rowH), cx(row.lane), midY, alpha(a.laneColor(src), 0xB0))
	}
	c := a.laneColor(row.lane)
	r := gtx.Dp(5)
	if laneW < gtx.Dp(10) {
		r = gtx.Dp(3)
	}
	center := image.Pt(int(cx(row.lane)), int(midY))
	fillCircle(gtx, a.pal.Mantle, center, r+gtx.Dp(2))
	fillCircle(gtx, c, center, r)
	if isTag(row.node) {
		fillCircle(gtx, a.pal.Mantle, center, max(1, r-gtx.Dp(2)))
	}
	msg := firstLine(row.commit.Msg)
	if msg == "" {
		msg = "(no commit message)"
	}
	return layout.Inset{Left: unit.Dp(float32(graphW) / gtx.Metric.PxPerDp), Right: unit.Dp(10)}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.Y = rowH
		children := []layout.FlexChild{layout.Rigid(a.codeLine(fmt.Sprintf("r%d", row.rev), a.pal.Sapphire, true))}
		if row.first || row.last || isTag(row.node) {
			label := row.node.Name
			if isTag(row.node) {
				label = "tag " + label
			}
			children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout), layout.Rigid(a.pillLabel(label, c)))
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
			layout.Flexed(1, func(gtx C) D {
				defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
				return a.codeLine(msg, a.pal.Text, false)(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
			layout.Rigid(a.codeLine(padRight(row.commit.Author, 12), a.pal.Subtext0, false)),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			layout.Rigid(a.codeLine(formatLogDate(row.commit.Date), a.pal.Overlay1, false)),
		)
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}
