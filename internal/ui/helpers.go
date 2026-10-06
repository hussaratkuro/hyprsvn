package ui

import (
	"fmt"
	"path"
	"strings"

	"github.com/hussaratkuro/hyprsvn/internal/model"
)

// Helpers kept from the TUI views: the state machine and the Gio screens
// share them.

func (m Model) listInnerHeight() int {
	// total - header - border(2) - status bar(2)
	return max(3, m.height-m.headerLines()-4)
}

// listBox wraps content in a border that fills the available list area.
func orDash(value string) string {
	if v := strings.TrimSpace(value); v != "" {
		return v
	}
	return "unknown"
}

func countUnselected(hunks []model.PartialHunk) int {
	n := 0
	for _, h := range hunks {
		if !h.Selected {
			n++
		}
	}
	return n
}

func (m Model) isCompactResultAction() bool {
	switch m.selectedAction {
	case model.ActionPull, model.ActionSwitchBranch, model.ActionSwitchTrunk,
		model.ActionMergeBranch, model.ActionCreateBranch, model.ActionCheckoutRevision,
		model.ActionDeleteBranch:
		return true
	}
	return false
}

func lastMeaningfulLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			return l
		}
	}
	return ""
}

// commitItemLabel renders a directory with a trailing slash: a property-only or
// copied directory is a change in its own right and should not read as a file.
func commitItemLabel(item model.CommitItem) string {
	label := item.Path
	if label == "." {
		label = "./  (working copy root)"
	} else if item.IsDir && !strings.HasSuffix(label, "/") {
		label += "/"
	}
	// A property-only directory commits its own properties and nothing below
	// it — say so, or it reads as if the whole subtree went along.
	if item.IsDir && item.PropsChanged && !item.Unversioned && !isScheduledDirChange(item) {
		label += "  (properties only, e.g. merge info)"
	}
	return label
}

// commitTreeItemLabel renders descendants of an unversioned directory like a
// nested HTML list while keeping item.Path untouched for SVN and diff commands.
func commitTreeItemLabel(item model.CommitItem) string {
	if item.IncludedByParent == "" {
		return commitItemLabel(item)
	}
	rel := strings.TrimPrefix(item.Path, strings.TrimSuffix(item.IncludedByParent, "/")+"/")
	depth := strings.Count(rel, "/") + 1
	name := path.Base(item.Path)
	if item.IsDir {
		name += "/"
	}
	return strings.Repeat("  ", depth) + "• " + name
}

// ── Properties ────────────────────────────────────────────────────────────────

func propertyValuePreview(value string, limit int) []string {
	trimmed := strings.TrimRight(value, "\n")
	if strings.TrimSpace(trimmed) == "" {
		return []string{"(empty)"}
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) <= limit {
		return lines
	}
	shown := append([]string(nil), lines[:limit]...)
	return append(shown, fmt.Sprintf("… %d more line(s)", len(lines)-limit))
}
