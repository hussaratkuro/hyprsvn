package ui

import (
	"fmt"
	"strconv"
	"strings"

	"gioui.org/io/event"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/hussaratkuro/hyprsvn/internal/model"
)

// Every screen mirrors the corresponding view in the TUI's views.go: same
// Model fields, same hints, drawn as widgets.

func (a *App) layoutScreen(gtx C) D {
	m := &a.m
	switch m.screen {
	case model.ScreenRepoSelect:
		return a.screenRepoSelect(gtx)
	case model.ScreenActionSelect:
		return a.screenActionSelect(gtx)
	case model.ScreenCreateBranchInput:
		return a.frame(gtx, "Create branch", a.inputBody("Branch parameter", nil, a.small("Final branch name format: YYYY-MM-DD_username_parameter", a.pal.Overlay0)),
			hintDef{"Enter", "create branch"}, hintDef{"i", "info"}, hintDef{"Esc", "back"})
	case model.ScreenCheckoutRevisionInput:
		return a.frame(gtx, "Checkout revision", a.inputBody("Search revisions",
			a.small("This runs: svn update -r REVISION", a.pal.Overlay0),
			a.lines(seg{"Searches the revision number, the author, the commit message and the changed file paths.", a.pal.Overlay0, false},
				seg{"An empty search lists every revision. Nothing is checked out until a revision is picked.", a.pal.Overlay0, false},
				seg{"Note: Pull will update the working copy back to HEAD.", a.pal.Yellow, true})),
			hintDef{"Enter", "search"}, hintDef{"i", "info"}, hintDef{"Esc", "back"})
	case model.ScreenCheckoutRevisionSelect:
		return a.screenCheckoutRevisionSelect(gtx)
	case model.ScreenFileHistorySearch:
		return a.frame(gtx, "File history search", a.inputBody("File path search", a.small("Search by local working-copy file path.", a.pal.Overlay0), nil),
			hintDef{"Enter", "search"}, hintDef{"i", "info"}, hintDef{"Esc", "back"})
	case model.ScreenFileHistorySelect:
		rows := make([]rowDef, len(m.fileHistoryItems))
		for i, p := range m.fileHistoryItems {
			rows[i] = rowDef{segs: []seg{{icoFile, a.pal.Overlay1, false}, {p, a.pal.Text, false}}}
		}
		return a.frame(gtx, "File history", a.listBody(a.small("Search: "+m.fileHistoryQuery, a.pal.Overlay0),
			func(gtx C) D {
				return a.rowsList(gtx, rows, m.fileHistoryCursor, func(i int) { a.m.fileHistoryCursor = i })
			}, nil),
			hintDef{"Enter", "show history"}, hintDef{"i", "info"}, hintDef{"Esc", "back"})
	case model.ScreenBranchSelect:
		return a.screenBranchSelect(gtx)
	case model.ScreenBranchMergeSelect:
		return a.screenBranchMergeSelect(gtx)
	case model.ScreenBranchDiffSelect:
		return a.screenBranchDiffSelect(gtx)
	case model.ScreenDeleteBranchConfirm:
		return a.screenDeleteBranchConfirm(gtx)
	case model.ScreenShelfSelect:
		return a.screenShelfSelect(gtx)
	case model.ScreenPullSelect:
		return a.screenPullSelect(gtx)
	case model.ScreenShelveSelect:
		return a.screenShelveSelect(gtx)
	case model.ScreenShelveNameInput:
		var below layout.Widget = a.small("Use a unique name without / or \\. Leading and trailing spaces are removed.", a.pal.Overlay0)
		if m.shelfNameError != "" {
			below = a.column2(below, a.status(m.shelfNameError, ""))
		}
		return a.frame(gtx, "Name shelf", a.inputBody("Shelf name", a.small(fmt.Sprintf("Selected files: %d", len(selectedCommitItems(m.commitItems))), a.pal.Overlay0), below),
			hintDef{"Enter", "create shelf"}, hintDef{"Esc", "back"})
	case model.ScreenCommitSelect:
		return a.screenCommitSelect(gtx)
	case model.ScreenCommitMessageInput:
		return a.screenCommitMessageInput(gtx)
	case model.ScreenPartialHunkSelect:
		return a.screenPartialHunkSelect(gtx)
	case model.ScreenRevertSelect:
		return a.screenRevertSelect(gtx)
	case model.ScreenConflictSelect:
		return a.screenConflictSelect(gtx)
	case model.ScreenPropertyBrowse:
		return a.screenPropertyBrowse(gtx)
	case model.ScreenPropertyTargetInput:
		return a.frame(gtx, "Properties", a.inputBody("Which path?", nil,
			a.lines(seg{"Empty search opens the working copy root (.), where a merge records svn:mergeinfo.", a.pal.Overlay0, false}, seg{"Otherwise type part of a path — directories are listed first.", a.pal.Overlay0, false})),
			hintDef{"Enter", "open"}, hintDef{"i", "info"}, hintDef{"Esc", "back"})
	case model.ScreenPropertyTargetSelect:
		rows := make([]rowDef, len(m.propertyTargets))
		for i, p := range m.propertyTargets {
			if p == "." {
				p = "./  (working copy root)"
			}
			rows[i] = rowDef{segs: []seg{{p, a.pal.Subtext0, false}}}
		}
		return a.frame(gtx, "Properties — pick a path", a.listBody(a.text("Matching paths:", a.pal.Text),
			func(gtx C) D {
				return a.rowsList(gtx, rows, m.propertyTargetCursor, func(i int) { a.m.propertyTargetCursor = i })
			}, nil),
			hintDef{"Enter", "properties"}, hintDef{"i", "info"}, hintDef{"Esc", "search again"})
	case model.ScreenPropertyList:
		return a.screenPropertyList(gtx)
	case model.ScreenPropertyNameSelect:
		return a.screenPropertyNameSelect(gtx)
	case model.ScreenPropertyNameInput:
		return a.frame(gtx, "Own property for "+m.propertyTarget, a.inputBody("Property name", nil, a.small("For a name of your own — the properties SVN documents are in the picker Esc goes back to.", a.pal.Overlay0)),
			hintDef{"Enter", "next: value"}, hintDef{"i", "info"}, hintDef{"Esc", "back"})
	case model.ScreenPropertyValueSelect:
		return a.screenPropertyValueSelect(gtx)
	case model.ScreenPropertyValueInput:
		title := "Set " + m.propertyName
		if m.propertyEditing {
			title = "Edit " + m.propertyName
		}
		return a.frame(gtx, title+" on "+m.propertyTarget, a.inputBody("Value", nil,
			a.lines(seg{`Type \n for a line break — svn:ignore and svn:mergeinfo are line-based.`, a.pal.Overlay0, false}, seg{"This runs: svn propset " + m.propertyName + " VALUE " + m.propertyTarget, a.pal.Overlay0, false})),
			hintDef{"Enter", "set property"}, hintDef{"i", "info"}, hintDef{"Esc", "back"})
	case model.ScreenHistory, model.ScreenHistorySearch:
		return a.screenHistory(gtx)
	case model.ScreenDiff:
		return a.screenDiff(gtx)
	case model.ScreenRunning:
		return a.screenRunning(gtx)
	case model.ScreenResult:
		return a.screenResult(gtx)
	}
	return D{}
}

func (a *App) column2(ws ...layout.Widget) layout.Widget {
	return func(gtx C) D { return a.column(gtx, ws...) }
}

// ---- repositories and actions ----

func (a *App) screenRepoSelect(gtx C) D {
	rows := make([]rowDef, len(a.m.repos))
	for i, r := range a.m.repos {
		rows[i] = rowDef{segs: []seg{{icoRepo, a.pal.Mauve, false}, {r.Path, a.pal.Text, false}, {r.CurrentLocation, a.pal.Overlay1, false}}}
	}
	return a.frame(gtx, "Select SVN repository", a.listBody(nil,
		func(gtx C) D { return a.rowsList(gtx, rows, a.m.repoCursor, func(i int) { a.m.repoCursor = i }) }, nil),
		hintDef{"Enter", "select"}, hintDef{"q", "quit"})
}

func (a *App) screenActionSelect(gtx C) D {
	m := &a.m
	filtered := m.filteredActionIndexes()
	rows := make([]rowDef, len(filtered))
	for i, idx := range filtered {
		rows[i] = rowDef{segs: []seg{{actionIcon(model.Action(idx)), a.pal.Teal, false}, {m.actions[idx], a.pal.Text, false}}}
	}
	var intro layout.Widget
	if m.actionFilterMode {
		intro = a.row(a.icon(icoSearch, a.pal.Yellow, unit.Sp(13)), a.mono(m.actionFilter+"_", a.pal.Rosewater, unit.Sp(13)), a.small(fmt.Sprintf("%d/%d · Enter runs · Esc cancels", len(filtered), len(m.actions)), a.pal.Overlay0))
	}
	var footer layout.Widget
	if len(filtered) == 0 {
		footer = a.small("no action matches "+m.actionFilter, a.pal.Overlay0)
	}
	hints := []hintDef{{"/", "search"}, {"Enter", "run"}, {"i", "info"}, {"Esc", "repos"}, {"q", "quit"}}
	if m.actionFilterMode {
		hints = []hintDef{{"Enter", "run"}, {"Esc", "cancel search"}}
	}
	return a.frame(gtx, "Select action", a.listBody(intro, func(gtx C) D {
		return a.rowsList(gtx, rows, m.actionCursor, func(i int) { a.m.actionCursor = i })
	}, footer), hints...)
}

func actionIcon(act model.Action) string {
	switch act {
	case model.ActionPull:
		return icoDown
	case model.ActionCommit, model.ActionCreateBranch:
		return icoUp
	case model.ActionSwitchBranch, model.ActionMergeBranch, model.ActionBranchDiffFromStart, model.ActionBranchDiffVsTrunk, model.ActionDeleteBranch, model.ActionSwitchTrunk, model.ActionRevisionTree:
		return icoBranch
	case model.ActionProperties, model.ActionCleanup:
		return icoCog
	case model.ActionCommitHistory, model.ActionFileHistory, model.ActionStatus:
		return icoList
	case model.ActionQuit:
		return icoTimes
	}
	return icoFile
}

// ---- revisions and branches ----

func (a *App) screenCheckoutRevisionSelect(gtx C) D {
	m := &a.m
	filtered := m.filteredCheckoutRevisions()
	rows := make([]rowDef, len(filtered))
	for i, rev := range filtered {
		msg := rev.Msg
		if msg == "" {
			msg = "(no commit message)"
		}
		segs := []seg{{fmt.Sprintf("r%d", rev.Revision), a.pal.Sapphire, true}, {rev.Author, a.pal.Subtext0, false}, {rev.Date, a.pal.Overlay1, false}}
		if p := checkoutRevisionMatchedPath(rev, m.checkoutRevQuery); p != "" {
			segs = append(segs, seg{"· " + p, a.pal.Overlay1, false})
		}
		segs = append(segs, seg{msg, a.pal.Text, false})
		rows[i] = rowDef{segs: segs}
	}
	var intro layout.Widget
	if m.checkoutRevFilterMode {
		intro = a.row(a.icon(icoSearch, a.pal.Yellow, unit.Sp(13)), a.mono(m.checkoutRevQuery+"_", a.pal.Rosewater, unit.Sp(13)), a.small(fmt.Sprintf("%d/%d · Enter checks out · Esc stops editing", len(filtered), len(m.checkoutRevisions)), a.pal.Overlay0))
	} else {
		q := strings.TrimSpace(m.checkoutRevQuery)
		if q == "" {
			q = "(every loaded revision)"
		}
		intro = a.row(a.small("Search:", a.pal.Overlay0), a.mono(q, a.pal.Rosewater, unit.Sp(12)), a.small(fmt.Sprintf("%d/%d · matches revision, author, message and changed paths", len(filtered), len(m.checkoutRevisions)), a.pal.Overlay0))
	}
	var footer layout.Widget
	if len(filtered) > 0 {
		rev := filtered[min(m.checkoutRevCursor, len(filtered)-1)]
		footer = func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return a.box(gtx, a.pal.Surface0, a.pal.Surface1, unit.Dp(8), layout.UniformInset(unit.Dp(10)), func(gtx C) D {
				ls := []seg{{fmt.Sprintf("r%d  ·  %s  ·  %s", rev.Revision, orDash(rev.Author), orDash(rev.Date)), a.pal.Yellow, true}, {orDash(rev.Msg), a.pal.Text, false}}
				if len(rev.Paths) == 0 {
					ls = append(ls, seg{"No changed paths in the log for this revision.", a.pal.Overlay0, false})
				} else {
					ls = append(ls, seg{fmt.Sprintf("Changed files (%d):", len(rev.Paths)), a.pal.Overlay0, false})
					for i, p := range rev.Paths {
						if i >= 6 {
							ls = append(ls, seg{fmt.Sprintf("  … and %d more", len(rev.Paths)-6), a.pal.Overlay0, false})
							break
						}
						ls = append(ls, seg{"  " + orDash(p.Action) + " " + p.Path, a.pal.Subtext0, false})
					}
				}
				return a.lines(ls...)(gtx)
			})
		}
	} else {
		footer = a.small("no revision matches "+m.checkoutRevQuery, a.pal.Overlay0)
	}
	hints := []hintDef{{"/", "edit search"}, {"Enter", "checkout selected"}, {"i", "info"}, {"Esc", "back"}}
	if m.checkoutRevFilterMode {
		hints = []hintDef{{"Enter", "checkout selected"}, {"Esc", "stop editing"}}
	}
	return a.frame(gtx, "Checkout revision", a.listBody(intro, func(gtx C) D {
		return a.rowsList(gtx, rows, m.checkoutRevCursor, func(i int) { a.m.checkoutRevCursor = i })
	}, footer), hints...)
}

func (a *App) screenBranchSelect(gtx C) D {
	m := &a.m
	title, enterAction := "Switch to branch", "switch"
	switch m.selectedAction {
	case model.ActionMergeBranch:
		title, enterAction = "Merge branch", "choose revision"
	case model.ActionBranchDiffFromStart:
		title, enterAction = "Branch diff since branch point", "compare"
	case model.ActionBranchDiffVsTrunk:
		title, enterAction = "Branch diff vs trunk HEAD", "compare"
	case model.ActionDeleteBranch:
		title, enterAction = "Delete branch", "confirm delete"
	}
	filtered := m.filteredBranches()
	current := currentBranchIndex(m.branches, m.activeRepo.CurrentLocation)
	rows := make([]rowDef, len(filtered))
	for i, br := range filtered {
		segs := []seg{{fmt.Sprintf("%3d", i+1), a.pal.Overlay1, false}, {icoBranch, a.pal.Teal, false}, {br.Name, a.pal.Text, false}}
		if m.branchFilter == "" && i == current && strings.HasPrefix(strings.Trim(m.activeRepo.CurrentLocation, "/"), "branches/") {
			segs = append(segs, seg{"checked out", a.pal.Green, false})
		}
		rows[i] = rowDef{segs: segs}
	}
	var intro layout.Widget
	switch {
	case m.branchFilterMode:
		intro = a.row(a.icon(icoSearch, a.pal.Yellow, unit.Sp(13)), a.mono(m.branchFilter+"_", a.pal.Rosewater, unit.Sp(13)), a.small(fmt.Sprintf("%d/%d · Enter: %s · Esc cancels", len(filtered), len(m.branches), enterAction), a.pal.Overlay0))
	case strings.TrimSpace(m.branchNumberInput) != "":
		intro = a.row(a.small("Branch number:", a.pal.Yellow), a.mono(m.branchNumberInput, a.pal.Rosewater, unit.Sp(13)), a.small("Enter: "+enterAction+" · Backspace edits", a.pal.Overlay0))
	default:
		intro = a.small(fmt.Sprintf("%d branches · type a number to jump", len(m.branches)), a.pal.Overlay0)
	}
	hints := []hintDef{{"/", "search"}, {"0-9", "jump to number"}, {"Enter", enterAction}, {"Esc", "back"}}
	if m.branchFilterMode {
		hints = []hintDef{{"Enter", enterAction}, {"Esc", "cancel search"}}
	}
	return a.frame(gtx, title, a.listBody(intro, func(gtx C) D {
		return a.rowsList(gtx, rows, m.branchCursor, func(i int) { a.m.branchCursor = i; a.m.branchNumberInput = "" })
	}, nil), hints...)
}

func (a *App) screenBranchMergeSelect(gtx C) D {
	m := &a.m
	rows := make([]rowDef, 0, len(m.mergeRevisions)+1)
	rows = append(rows, rowDef{segs: []seg{{icoBolt, a.pal.Teal, false}, {"Merge latest branch state through HEAD", a.pal.Teal, true}, {"apply every unmerged branch revision", a.pal.Overlay1, false}}})
	for _, rev := range m.mergeRevisions {
		msg := rev.Msg
		if msg == "" {
			msg = "(no commit message)"
		}
		rows = append(rows, rowDef{segs: []seg{{fmt.Sprintf("r%d", rev.Revision), a.pal.Sapphire, true}, {rev.Author, a.pal.Subtext0, false}, {rev.Date, a.pal.Overlay1, false}, {msg, a.pal.Text, false}}})
	}
	intro := a.row(a.small("Source branch:", a.pal.Overlay0), a.mono(m.mergeBranch.Name, a.pal.Yellow, unit.Sp(13)), a.small("· choose the latest branch state or one commit", a.pal.Overlay0))
	return a.frame(gtx, "Merge branch revision", a.listBody(intro, func(gtx C) D {
		return a.rowsList(gtx, rows, m.mergeCursor, func(i int) { a.m.mergeCursor = i })
	}, nil), hintDef{"Enter", "merge selected"}, hintDef{"i", "info"}, hintDef{"Esc", "branches"})
}

func (a *App) screenBranchDiffSelect(gtx C) D {
	m := &a.m
	ctx := m.branchDiffCtx
	rows := make([]rowDef, len(m.branchDiffItems))
	for i, it := range m.branchDiffItems {
		label := it.Path
		if it.IsDir && !strings.HasSuffix(label, "/") {
			label += "/"
		}
		rows[i] = rowDef{segs: []seg{{padRight(it.Status, 3), a.statusColor(it.Status), true}, {label, a.pal.Text, false}}}
	}
	ls := []seg{{"Old  " + ctx.Old.Label, a.pal.Sapphire, false}, {"New  " + ctx.New.Label, a.pal.Yellow, false}}
	if strings.TrimSpace(ctx.Summary) != "" {
		ls = append(ls, seg{ctx.Summary, a.pal.Overlay0, false})
	}
	ls = append(ls, seg{fmt.Sprintf("Changed paths: %d", len(m.branchDiffItems)), a.pal.Text, false})
	return a.frame(gtx, ctx.Title(), a.listBody(a.lines(ls...), func(gtx C) D {
		return a.rowsList(gtx, rows, m.branchDiffCursor, func(i int) { a.m.branchDiffCursor = i })
	}, nil), hintDef{"Enter/m", "hyprmerge"}, hintDef{"d", "built-in diff"}, hintDef{"u", "full diff"}, hintDef{"i", "info"}, hintDef{"Esc", "branches"}, hintDef{"q", "menu"})
}

func (a *App) statusColor(status string) color4 {
	t := strings.TrimSpace(status)
	if t == "" {
		return a.pal.Subtext0
	}
	switch t[:1] {
	case "A", "?":
		return diffGreen
	case "D", "!":
		return diffRed
	case "M", "R", "U", "G":
		return diffYellow
	case "C":
		return diffMauve
	}
	return a.pal.Subtext0
}

func (a *App) screenDeleteBranchConfirm(gtx C) D {
	info := a.m.branchDelete
	ls := []seg{{"This deletes the branch from the repository, not just locally.", a.pal.Yellow, true},
		{"It runs: svn delete <branch URL> -m \"Deleting branch ...\"", a.pal.Overlay0, false}, {"", a.pal.Text, false},
		{"Branch       " + info.Name, a.pal.Yellow, true}, {"URL          " + info.URL, a.pal.Rosewater, false}}
	if info.LastRev > 0 {
		ls = append(ls, seg{fmt.Sprintf("Last commit  r%d  by %s  %s", info.LastRev, orDash(info.Author), orDash(info.Date)), a.pal.Green, false})
		if strings.TrimSpace(info.Msg) != "" {
			ls = append(ls, seg{"Message      " + info.Msg, a.pal.Text, false})
		}
	} else {
		ls = append(ls, seg{"Last commit  unknown", a.pal.Overlay0, false})
	}
	if info.IsCheckout {
		ls = append(ls, seg{"The working copy is on this branch. After the delete, switch to trunk.", a.pal.Red, true})
	}
	return a.frame(gtx, "Delete branch", a.inputBody("Type "+branchDeleteConfirmWord+" and press Enter to delete it", a.lines(ls...),
		a.small("Anything else cancels. The branch history stays in the repository, so it can be restored with svn copy from an earlier revision.", a.pal.Overlay0)),
		hintDef{"Enter", "delete branch"}, hintDef{"i", "info"}, hintDef{"Esc", "cancel"})
}

// ---- shelves ----

func (a *App) screenShelfSelect(gtx C) D {
	m := &a.m
	rows := make([]rowDef, len(m.shelves))
	for i, name := range m.shelves {
		r := rowDef{segs: []seg{{icoFolder, a.pal.Teal, false}, {name, a.pal.Text, false}}}
		if i == m.shelfDeleteIdx {
			r.segs = append(r.segs, seg{"DELETE!", a.pal.Red, true})
			r.tint = a.pal.Red
		}
		rows[i] = r
	}
	hints := []hintDef{{"Enter", "unshelve"}, {"Del", "delete shelf"}, {"i", "info"}, {"Esc", "back"}}
	var footer layout.Widget
	if m.shelfDeleteIdx >= 0 {
		footer = a.text("Del again: discard this shelf and its saved changes — or move cursor to cancel", a.pal.Yellow)
	}
	return a.frame(gtx, "Unshelve changes", a.listBody(a.text("Available shelves:", a.pal.Text), func(gtx C) D {
		return a.rowsList(gtx, rows, m.shelfCursor, func(i int) { a.m.shelfCursor = i })
	}, footer), hints...)
}

// ---- pull / commit / revert / shelve lists ----

func (a *App) screenPullSelect(gtx C) D {
	m := &a.m
	selectedFiles, totalFiles := 0, 0
	for _, it := range m.commitItems {
		if !it.IsDir {
			totalFiles++
			if it.Selected {
				selectedFiles++
			}
		}
	}
	rows := make([]rowDef, len(m.commitItems))
	for i, it := range m.commitItems {
		if it.IsDir {
			allSel, anySel, count := true, false, 0
			for j := i + 1; j < len(m.commitItems) && !m.commitItems[j].IsDir; j++ {
				count++
				if m.commitItems[j].Selected {
					anySel = true
				} else {
					allSel = false
				}
			}
			chk := a.check(count > 0 && allSel)
			if anySel && !allSel {
				chk = seg{icoSquare, a.pal.Yellow, false}
			}
			rows[i] = rowDef{segs: []seg{chk, {icoFolder, a.pal.Mauve, false}, {it.Path, a.pal.Mauve, true}}}
			continue
		}
		rows[i] = rowDef{indent: 1, segs: []seg{a.check(it.Selected), {padRight(it.Status, 2), a.statusColor(it.Status), true}, {it.Path, a.pal.Rosewater, false}}}
	}
	intro := a.lines(seg{fmt.Sprintf("Selected: %d of %d files", selectedFiles, totalFiles), a.pal.Overlay0, false}, seg{"Incoming repository changes:", a.pal.Text, false})
	return a.frame(gtx, "Pull incoming changes", a.listBody(intro, func(gtx C) D {
		return a.rowsList(gtx, rows, m.commitCursor, func(i int) { a.m.commitCursor = i })
	}, nil), hintDef{"Space", "select/dir"}, hintDef{"a", "all"}, hintDef{"n", "none"}, hintDef{"d", "diff"}, hintDef{"m", "hyprmerge"}, hintDef{"Enter", "pull selected"}, hintDef{"i", "info"}, hintDef{"Esc", "back"})
}

func (a *App) screenShelveSelect(gtx C) D {
	m := &a.m
	rows := make([]rowDef, len(m.commitItems))
	for i, it := range m.commitItems {
		status := it.Status
		if it.Unversioned {
			status = "? copy"
		}
		rows[i] = rowDef{segs: []seg{a.check(it.Selected), {padRight(status, 8), a.statusColor(status), true}, {commitItemLabel(it), a.pal.Text, false}}}
	}
	intro := a.lines(seg{fmt.Sprintf("Selected files: %d | Unversioned (will be copied): %d", len(selectedCommitItems(m.commitItems)), len(selectedUnversionedCommitPaths(m.commitItems))), a.pal.Overlay0, false}, seg{"Working copy changes:", a.pal.Text, false})
	return a.frame(gtx, "Shelve local changes", a.listBody(intro, func(gtx C) D {
		return a.rowsList(gtx, rows, m.commitCursor, func(i int) { a.m.commitCursor = i })
	}, nil), hintDef{"Space", "select"}, hintDef{"a", "all"}, hintDef{"n", "none"}, hintDef{"d", "diff"}, hintDef{"m", "hyprmerge"}, hintDef{"Enter", "shelve"}, hintDef{"i", "info"}, hintDef{"Esc", "back"})
}

func (a *App) screenCommitSelect(gtx C) D {
	m := &a.m
	rows := make([]rowDef, len(m.commitItems))
	canDelete := func(ci model.CommitItem) bool {
		return ci.Unversioned || (len(ci.Status) > 0 && ci.Status[0] == 'A')
	}
	hasDeletable := false
	for i, it := range m.commitItems {
		armed := m.deleteConfirmIdx == i
		status := it.Status
		if it.Unversioned {
			status = "? add"
		}
		if armed {
			status = strings.TrimSpace(status[:1]) + " DEL!"
		}
		if canDelete(it) {
			hasDeletable = true
		}
		indent := 0
		label := commitItemLabel(it)
		if it.IncludedByParent != "" {
			rel := strings.TrimPrefix(it.Path, strings.TrimSuffix(it.IncludedByParent, "/")+"/")
			indent = strings.Count(rel, "/") + 1
			label = strings.TrimLeft(commitTreeItemLabel(it), " ")
		}
		r := rowDef{indent: indent, segs: []seg{a.check(it.Selected), {padRight(status, 8), a.statusColor(status), true}, {label, a.pal.Text, false}}}
		if armed {
			r.tint = a.pal.Red
		}
		rows[i] = r
	}
	ls := []seg{{fmt.Sprintf("Selected items: %d | Unversioned (will be svn add-ed): %d", len(selectedCommitItems(m.commitItems)), len(selectedUnversionedCommitPaths(m.commitItems))), a.pal.Overlay0, false}}
	if n := len(m.commitConflicts); n > 0 {
		ls = append(ls, seg{fmt.Sprintf("%d path(s) left out: still in conflict, SVN refuses to commit them — see Resolve conflicts", n), a.pal.Yellow, true})
		for _, p := range m.commitConflicts {
			ls = append(ls, seg{"  " + p, a.pal.Yellow, false})
		}
	}
	ls = append(ls, seg{"Working copy changes · drag selected files onto a hyprcommander pane to copy them there", a.pal.Text, false})
	var footer layout.Widget
	hints := []hintDef{{"Space", "select/dir"}, {"a", "all"}, {"n", "none"}, {"h", "hide root"}, {"d", "file diff"}, {"m", "hyprmerge"}, {"p", "partial hunks"}, {"Enter", "commit message"}, {"i", "info"}, {"Esc", "back"}}
	if hasDeletable {
		hints = append(hints, hintDef{"Del", "delete"})
	}
	if m.deleteConfirmIdx >= 0 {
		footer = a.text("Del again: confirm permanent deletion — or move cursor to cancel", a.pal.Yellow)
		hints = []hintDef{{"Del", "confirm delete"}}
	}
	if a.drag.status != "" {
		if a.drag.statusErr {
			footer = a.status(a.drag.status, "")
		} else {
			footer = a.status("", a.drag.status)
		}
	}
	return a.frame(gtx, "Commit", a.listBody(a.lines(ls...), func(gtx C) D {
		return a.dragRows(gtx, rows, m.commitCursor, func(i int) { a.m.commitCursor = i; a.m.deleteConfirmIdx = -1 })
	}, footer), hints...)
}

func (a *App) screenRevertSelect(gtx C) D {
	m := &a.m
	rows := make([]rowDef, len(m.commitItems))
	for i, it := range m.commitItems {
		rows[i] = rowDef{segs: []seg{a.check(it.Selected), {padRight(it.Status, 8), a.statusColor(it.Status), true}, {it.Path, a.pal.Text, false}}}
	}
	intro := a.lines(seg{"Warning: SVN revert discards local changes for selected versioned files.", a.pal.Yellow, true}, seg{fmt.Sprintf("Selected files: %d", len(selectedCommitPaths(m.commitItems))), a.pal.Overlay0, false}, seg{"Working copy changes:", a.pal.Text, false})
	return a.frame(gtx, "Revert files", a.listBody(intro, func(gtx C) D {
		return a.rowsList(gtx, rows, m.commitCursor, func(i int) { a.m.commitCursor = i })
	}, nil), hintDef{"Space", "select"}, hintDef{"a", "all"}, hintDef{"n", "none"}, hintDef{"d", "diff"}, hintDef{"m", "hyprmerge"}, hintDef{"Enter", "revert selected"}, hintDef{"i", "info"}, hintDef{"Esc", "back"})
}

func (a *App) screenCommitMessageInput(gtx C) D {
	m := &a.m
	if m.partialCommit {
		var sel []model.PartialHunk
		for _, h := range m.partialHunks {
			if h.Selected {
				sel = append(sel, h)
			}
		}
		ls := []seg{{"File: " + m.partialItem.Path, a.pal.Overlay0, false}, {fmt.Sprintf("Selected hunks: %d", len(sel)), a.pal.Overlay0, false}}
		for i := 0; i < min(6, len(sel)); i++ {
			ls = append(ls, seg{fmt.Sprintf("  hunk %d: %s", i+1, sel[i].Header), a.pal.Overlay1, false})
		}
		if len(sel) > 6 {
			ls = append(ls, seg{fmt.Sprintf("  … and %d more", len(sel)-6), a.pal.Overlay1, false})
		}
		return a.frame(gtx, "Partial commit message", a.inputBody("Commit message", a.lines(ls...), nil),
			hintDef{"Enter", "commit selected hunks"}, hintDef{"i", "info"}, hintDef{"Esc", "back"})
	}
	items := selectedCommitItems(m.commitItems)
	ls := []seg{{fmt.Sprintf("Files selected: %d", len(items)), a.pal.Overlay0, false}}
	for i := 0; i < min(8, len(items)); i++ {
		prefix := "  "
		if items[i].Unversioned {
			prefix = "  + "
		}
		ls = append(ls, seg{prefix + items[i].Path, a.pal.Overlay1, false})
	}
	if len(items) > 8 {
		ls = append(ls, seg{fmt.Sprintf("  … and %d more", len(items)-8), a.pal.Overlay1, false})
	}
	if len(selectedUnversionedCommitPaths(m.commitItems)) > 0 {
		ls = append(ls, seg{"Selected unversioned files will be added with svn add before commit.", a.pal.Yellow, true})
	}
	return a.frame(gtx, "Commit message", a.inputBody("Commit message", a.lines(ls...), nil),
		hintDef{"Enter", "commit"}, hintDef{"i", "info"}, hintDef{"Esc", "back"})
}

func (a *App) screenPartialHunkSelect(gtx C) D {
	m := &a.m
	rows := make([]rowDef, len(m.partialHunks))
	for i, h := range m.partialHunks {
		var extra []seg
		shown := 0
		for _, line := range h.Lines {
			if line == "" || line[0] == ' ' {
				continue
			}
			if shown >= 8 {
				extra = append(extra, seg{"      …", a.pal.Overlay0, false})
				break
			}
			c, _ := a.classifySVN(line)
			extra = append(extra, seg{"      " + line, c, false})
			shown++
		}
		rows[i] = rowDef{
			segs:  []seg{a.check(h.Selected), {fmt.Sprintf("hunk %d", i+1), a.pal.Text, true}, {h.Header, a.pal.Yellow, false}, {fmt.Sprintf("+%d", h.Added), a.pal.Green, false}, {fmt.Sprintf("-%d", h.Removed), a.pal.Red, false}},
			extra: extra,
		}
	}
	intro := a.lines(seg{"File: " + m.partialItem.Path, a.pal.Overlay0, false}, seg{fmt.Sprintf("Selected hunks: %d of %d", len(m.partialHunks)-countUnselected(m.partialHunks), len(m.partialHunks)), a.pal.Overlay0, false}, seg{"Choose hunks to commit:", a.pal.Text, false})
	return a.frame(gtx, "Partial commit hunks", a.listBody(intro, func(gtx C) D {
		return a.rowsList(gtx, rows, m.partialHunkCursor, func(i int) { a.m.partialHunkCursor = i })
	}, nil), hintDef{"Space", "select"}, hintDef{"a", "all"}, hintDef{"n", "none"}, hintDef{"Enter", "commit message"}, hintDef{"i", "info"}, hintDef{"Esc", "back"})
}

// ---- conflicts ----

func (a *App) screenConflictSelect(gtx C) D {
	m := &a.m
	tree := m.treeConflicts()
	rows := make([]rowDef, len(m.conflictItems))
	for i, it := range m.conflictItems {
		r := rowDef{segs: []seg{{icoWarn, a.pal.Red, false}, {padRight(it.Status, 10), a.pal.Mauve, true}, {it.Path, a.pal.Text, false}}}
		if it.IsTree && m.resolveConfirm == resolveConfirmAllTree {
			r.tint = a.pal.Yellow
		}
		rows[i] = r
	}
	intro := a.lines(seg{fmt.Sprintf("%d conflict(s), %d of them tree conflicts", len(m.conflictItems), len(tree)), a.pal.Overlay0, false}, seg{"Conflicted files:", a.pal.Text, false})
	fs := []seg{{"File conflicts → Merger (default, Enter), Meld (M), keep current (m), take incoming (t). Tree conflicts → --accept=working (r).", a.pal.Yellow, false}}
	if strings.TrimSpace(m.conflictNotice) != "" {
		fs = append(fs, seg{m.conflictNotice, a.pal.Red, true})
	}
	switch m.resolveConfirm {
	case resolveConfirmAllTree:
		fs = append(fs, seg{fmt.Sprintf("r again: resolve all %d tree conflict(s) with --accept=working — or move cursor to cancel", len(tree)), a.pal.Yellow, true})
	case resolveConfirmMine:
		fs = append(fs, seg{"m again: keep the current file and discard the incoming changes — or move cursor to cancel", a.pal.Yellow, true})
	case resolveConfirmTheirs:
		fs = append(fs, seg{"t again: take the incoming file and discard your changes — or move cursor to cancel", a.pal.Yellow, true})
	}
	hints := []hintDef{}
	if len(tree) > 0 {
		hints = append(hints, hintDef{"r", fmt.Sprintf("resolve all %d tree", len(tree))})
	}
	if len(m.conflictItems) > 0 && !m.conflictItems[min(m.conflictCursor, len(m.conflictItems)-1)].IsTree {
		hints = append(hints, hintDef{"m", "keep current"}, hintDef{"t", "take incoming"})
	}
	hints = append(hints, hintDef{"Enter", "Merger (default)"}, hintDef{"M", "Meld"}, hintDef{"i", "info"}, hintDef{"Esc", "back"})
	return a.frame(gtx, "Resolve conflicts", a.listBody(intro, func(gtx C) D {
		return a.rowsList(gtx, rows, m.conflictCursor, func(i int) { a.m.conflictCursor = i; a.m.resolveConfirm = resolveConfirmNone; a.m.conflictNotice = "" })
	}, a.lines(fs...)), hints...)
}

// ---- properties ----

func (a *App) screenPropertyBrowse(gtx C) D {
	m := &a.m
	rows := make([]rowDef, len(m.propertyBrowseEntries))
	for i, e := range m.propertyBrowseEntries {
		ico, col := icoFile, a.pal.Subtext0
		switch {
		case e.Kind == model.PropertyBrowseParent:
			ico, col = icoUp, a.pal.Overlay0
		case e.Kind == model.PropertyBrowseSelf:
			ico, col = icoDot, a.pal.Sapphire
		case e.Kind == model.PropertyBrowseDir:
			ico, col = icoFolder, a.pal.Teal
		}
		if e.Unversioned {
			col = a.pal.Overlay0
		}
		segs := []seg{{ico, col, false}, {e.Label, col, e.Kind != model.PropertyBrowseFile}}
		switch {
		case e.Unversioned:
			segs = append(segs, seg{"?", a.pal.Overlay0, false})
		case len(e.Props) > 0:
			segs = append(segs, seg{fmt.Sprintf("%s %d", icoCog, len(e.Props)), a.pal.Yellow, false})
		}
		rows[i] = rowDef{segs: segs}
	}
	return a.frame(gtx, "Properties — "+m.propertyBrowseDir, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Flexed(1, a.listBody(a.row(a.small("Path:", a.pal.Overlay0), a.mono(m.propertyBrowseDir, a.pal.Text, unit.Sp(13))), func(gtx C) D {
				return a.rowsList(gtx, rows, m.propertyBrowseCursor, func(i int) { a.m.propertyBrowseCursor = i; a.m.propertyBrowseNotice = "" })
			}, nil)),
			layout.Rigid(layout.Spacer{Width: unit.Dp(16)}.Layout),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Max.X = gtx.Dp(360)
				gtx.Constraints.Min = gtx.Constraints.Max
				return a.box(gtx, a.pal.Surface0, a.pal.Surface1, unit.Dp(8), layout.UniformInset(unit.Dp(12)), func(gtx C) D {
					gtx.Constraints.Min = gtx.Constraints.Max
					return a.propertyPane(gtx)
				})
			}),
		)
	}, hintDef{"→/Enter", "open"}, hintDef{"←", "up"}, hintDef{"p", "properties"}, hintDef{"a", "add"}, hintDef{"/", "search path"}, hintDef{"Esc", "back"})
}

// propertyPane shows what the highlighted path carries and what can be done.
func (a *App) propertyPane(gtx C) D {
	m := &a.m
	entry, ok := m.propertyBrowseEntry()
	if !ok {
		return a.small("Empty directory.", a.pal.Overlay0)(gtx)
	}
	ls := []seg{}
	if m.propertyBrowseNotice != "" {
		ls = append(ls, seg{m.propertyBrowseNotice, a.pal.Yellow, true})
	}
	if entry.Kind == model.PropertyBrowseParent {
		ls = append(ls, seg{"Parent directory", a.pal.Yellow, true}, seg{"Enter or ← steps one level up.", a.pal.Overlay0, false})
		return a.lines(ls...)(gtx)
	}
	ls = append(ls, seg{"Properties of " + entry.Path, a.pal.Yellow, true})
	switch {
	case entry.Unversioned:
		ls = append(ls, seg{"not versioned", a.pal.Yellow, false}, seg{"svn add it first — SVN keeps properties on versioned paths only.", a.pal.Overlay0, false})
	case len(entry.Props) == 0:
		ls = append(ls, seg{"no properties on this path", a.pal.Overlay0, false})
	default:
		for _, prop := range entry.Props {
			ls = append(ls, seg{prop.Name, a.pal.Yellow, false})
			for _, line := range propertyValuePreview(prop.Value, 2) {
				ls = append(ls, seg{"  " + line, a.pal.Overlay1, false})
			}
		}
	}
	ls = append(ls, seg{"", a.pal.Text, false}, seg{"Options", a.pal.Yellow, true})
	for _, opt := range propertyBrowseOptions(entry) {
		ls = append(ls, seg{"  " + opt, a.pal.Overlay1, false})
	}
	return a.lines(ls...)(gtx)
}

func (a *App) screenPropertyList(gtx C) D {
	m := &a.m
	target := m.propertyTarget
	if target == "." {
		target = "./  (working copy root)"
	}
	rows := make([]rowDef, len(m.propertyItems))
	for i, it := range m.propertyItems {
		r := rowDef{segs: []seg{{icoCog, a.pal.Yellow, false}, {it.Name, a.pal.Text, true}}}
		if i == m.propertyDeleteIdx {
			r.segs = append(r.segs, seg{"DELETE!", a.pal.Red, true})
			r.tint = a.pal.Red
		}
		for _, l := range propertyValuePreview(it.Value, 3) {
			r.extra = append(r.extra, seg{"      " + l, a.pal.Overlay1, false})
		}
		rows[i] = r
	}
	ls := []seg{}
	if m.propertyNotice != "" {
		ls = append(ls, seg{m.propertyNotice, a.pal.Green, true})
	}
	ls = append(ls, seg{fmt.Sprintf("%d propert(y/ies):", len(m.propertyItems)), a.pal.Text, false})
	var footer layout.Widget
	hints := []hintDef{{"a", "add"}, {"Enter/e", "edit value"}, {"Del", "delete"}, {"i", "info"}, {"Esc", "back"}}
	if len(m.propertyItems) == 0 {
		footer = a.small("no properties on this path — press a to add one", a.pal.Overlay0)
	}
	if m.propertyDeleteIdx >= 0 {
		footer = a.text("Del again: remove this property — or move cursor to cancel", a.pal.Yellow)
	}
	return a.frame(gtx, "Properties of "+target, a.listBody(a.lines(ls...), func(gtx C) D {
		return a.rowsList(gtx, rows, m.propertyCursor, func(i int) { a.m.propertyCursor = i; a.m.propertyDeleteIdx = -1 })
	}, footer), hints...)
}

func (a *App) screenPropertyNameSelect(gtx C) D {
	m := &a.m
	kind := "file"
	if m.propertyTargetIsDir {
		kind = "directory"
	}
	rows := make([]rowDef, len(m.propertyNameChoices))
	for i, c := range m.propertyNameChoices {
		var segs []seg
		switch {
		case c.Custom:
			segs = []seg{{icoBolt, a.pal.Teal, false}, {c.Name, a.pal.Teal, true}, {c.Summary, a.pal.Overlay1, false}}
		case c.IsSet:
			segs = []seg{{icoDot, a.pal.Yellow, false}, {c.Name, a.pal.Yellow, true}, {c.Summary, a.pal.Overlay1, false}, {"= " + compactPropertyValue(c.Current), a.pal.Subtext0, false}}
		default:
			segs = []seg{{" ", a.pal.Text, false}, {c.Name, a.pal.Text, false}, {c.Summary, a.pal.Overlay1, false}}
		}
		r := rowDef{segs: segs}
		if i == m.propertyDeleteIdx {
			r.segs = append(r.segs, seg{"DELETE!", a.pal.Red, true})
			r.tint = a.pal.Red
		}
		rows[i] = r
	}
	var footer layout.Widget
	if m.propertyDeleteIdx >= 0 {
		footer = a.text("Del again: remove this property — or move cursor to cancel", a.pal.Yellow)
	}
	return a.frame(gtx, "Pick a property for "+m.propertyTarget, a.listBody(a.text("Pick a property — the list covers what SVN documents for a "+kind+":", a.pal.Text), func(gtx C) D {
		return a.rowsList(gtx, rows, m.propertyNameCursor, func(i int) { a.m.propertyNameCursor = i; a.m.propertyDeleteIdx = -1 })
	}, footer), hintDef{"Enter", "choose"}, hintDef{"Del", "remove set property"}, hintDef{"Esc", "back"})
}

func (a *App) screenPropertyValueSelect(gtx C) D {
	m := &a.m
	rows := make([]rowDef, 0, len(m.propertyValueChoices)+1)
	for _, c := range m.propertyValueChoices {
		label := c.Label
		if label == "" {
			label = c.Value
		}
		rows = append(rows, rowDef{segs: []seg{{label, a.pal.Text, true}, {c.Summary, a.pal.Overlay1, false}}})
	}
	rows = append(rows, rowDef{segs: []seg{{icoBolt, a.pal.Teal, false}, {"Type a value of your own", a.pal.Teal, true}}})
	var footer layout.Widget
	if cur := m.propertyCurrentValue(); cur != "" {
		footer = a.small("Current value: "+compactPropertyValue(cur), a.pal.Overlay0)
	}
	return a.frame(gtx, "Set "+m.propertyName+" on "+m.propertyTarget, a.listBody(a.text("Pick a value — it is set as soon as you press Enter:", a.pal.Text), func(gtx C) D {
		return a.rowsList(gtx, rows, m.propertyValueCursor, func(i int) { a.m.propertyValueCursor = i })
	}, footer), hintDef{"Enter", "set value"}, hintDef{"Esc", "back"})
}

// ---- text screens ----

func (a *App) screenRunning(gtx C) D {
	m := &a.m
	return a.frame(gtx, m.runningTitle, func(gtx C) D {
		gtx.Constraints.Min = gtx.Constraints.Max
		if len(m.runningLines) == 0 {
			return layout.Center.Layout(gtx, a.row(a.spinner, a.text("SVN is working. It may grumble a little.", a.pal.Overlay1)))
		}
		a.list.ScrollToEnd = m.runningPinTail
		d := a.textLines(gtx, &a.list, m.runningLines, a.classifySVN)
		a.list.ScrollToEnd = false
		return d
	})
}

func (a *App) spinner(gtx C) D {
	gtx.Constraints.Max.X = gtx.Dp(18)
	gtx.Constraints.Max.Y = gtx.Dp(18)
	return material.Loader(a.th).Layout(gtx)
}

func (a *App) screenResult(gtx C) D {
	m := &a.m
	if m.err == nil && m.isCompactResultAction() && !m.resultExpanded {
		label := map[model.Action]string{
			model.ActionPull: "Pull", model.ActionSwitchBranch: "Switch branch", model.ActionSwitchTrunk: "Switch to trunk",
			model.ActionMergeBranch: "Merge branch", model.ActionCreateBranch: "Create branch", model.ActionCheckoutRevision: "Checkout revision",
			model.ActionDeleteBranch: "Delete branch",
		}[m.selectedAction]
		r := m.activeRepo
		return a.frame(gtx, "Done", func(gtx C) D {
			ws := []layout.Widget{a.row(a.iconDisc(icoCheck, a.pal.Green, 34), a.heading(label+" completed"))}
			if s := lastMeaningfulLine(stripANSI(m.result)); s != "" {
				ws = append(ws, a.text(s, a.pal.Text))
			}
			if r.CurrentLocation != "" && r.CurrentLocation != "unknown" {
				ws = append(ws, a.row(a.small("Location:", a.pal.Overlay0), a.mono(r.CurrentLocation, a.pal.Yellow, unit.Sp(13))))
			}
			if r.CurrentRevision != "" && r.CurrentRevision != "unknown" {
				ws = append(ws, a.row(a.small("Revision:", a.pal.Overlay0), a.mono("r"+r.CurrentRevision, a.pal.Green, unit.Sp(13))))
			}
			return a.column(gtx, ws...)
		}, hintDef{"Enter", "show log"}, hintDef{"q", "quit"})
	}
	title, ico, col := "Done", icoCheck, a.pal.Green
	if m.err != nil {
		title, ico, col = "Error", icoWarn, a.pal.Red
	}
	lines := m.viewport.Lines()
	return a.frame(gtx, title, a.listBody(a.row(a.icon(ico, col, unit.Sp(14)), a.small(strconv.Itoa(len(lines))+" lines", a.pal.Overlay0)), func(gtx C) D {
		return a.textLines(gtx, &a.list, lines, a.classifySVN)
	}, nil), hintDef{"Enter", "back to menu"}, hintDef{"q", "quit"})
}

// ---- command palette ----

func (a *App) layoutPalette(gtx C) D {
	m := &a.m
	paint.FillShape(gtx.Ops, alpha(a.pal.Crust, 0xA0), clip.Rect{Max: gtx.Constraints.Max}.Op())
	scrim := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, &scrimTag)
	scrim.Pop()
	items := m.filteredCommands()
	rows := make([]rowDef, len(items))
	for i, it := range items {
		rows[i] = rowDef{segs: []seg{{it.label, a.pal.Text, false}, {it.hint, a.pal.Overlay1, false}}}
	}
	return layout.Center.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(640))
		gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(480))
		gtx.Constraints.Min = gtx.Constraints.Max
		return a.panel(func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(a.row(a.iconDisc(icoBolt, a.pal.Mauve, 30), a.heading("Command palette"), a.small("fuzzy-search repositories and SVN actions", a.pal.Overlay0))),
				layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
				layout.Rigid(func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return a.box(gtx, a.pal.Surface0, a.pal.Mauve, unit.Dp(8), layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(12), Right: unit.Dp(12)},
						a.row(a.icon(icoSearch, a.pal.Overlay1, unit.Sp(14)), a.mono(m.commands.query+"_", a.pal.Text, unit.Sp(14))))
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
				layout.Flexed(1, func(gtx C) D {
					if len(items) == 0 {
						return a.small("No matching commands", a.pal.Overlay0)(gtx)
					}
					// the screen under the scrim already laid out a.list this frame;
					// use the palette's own list, clickables and cursor memory.
					a.list, a.palList = a.palList, a.list
					a.rowClicks, a.palClicks = a.palClicks, a.rowClicks
					a.lastCursor, a.palCursor = a.palCursor, a.lastCursor
					defer func() {
						a.list, a.palList = a.palList, a.list
						a.rowClicks, a.palClicks = a.palClicks, a.rowClicks
						a.lastCursor, a.palCursor = a.palCursor, a.lastCursor
					}()
					return a.rowsList(gtx, rows, m.commands.cursor, func(i int) { a.m.commands.cursor = i })
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(a.small("type to filter · Enter runs · Esc closes", a.pal.Overlay0)),
			)
		})(gtx)
	})
}

var scrimTag int
