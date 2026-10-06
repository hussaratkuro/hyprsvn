package ui

import (
	"testing"

	"github.com/hussaratkuro/hyprsvn/internal/model"
)

// A branch created at r2, committed at r3, merged back to trunk at r5: its
// lane must stay alive through r4 and end in the merge row, so the line
// runs tip → merge instead of a floating diagonal from above.
func TestGraphRowsMergeBackKeepsLaneAlive(t *testing.T) {
	a := &App{}
	a.m.historyEntries = []model.SVNLogEntryXML{
		{Revision: 1, Msg: "init", Paths: []model.SVNLogPathXML{{Action: "A", Path: "/trunk"}}},
		{Revision: 2, Msg: "branch", Paths: []model.SVNLogPathXML{{Action: "A", Path: "/branches/feat", CopyFromPath: "/trunk", CopyFromRev: 1}}},
		{Revision: 3, Msg: "work", Paths: []model.SVNLogPathXML{{Action: "M", Path: "/branches/feat/a.go"}}},
		{Revision: 4, Msg: "trunk work", Paths: []model.SVNLogPathXML{{Action: "M", Path: "/trunk/b.go"}}},
		{Revision: 5, Msg: "merge feat", Paths: []model.SVNLogPathXML{{Action: "M", Path: "/trunk/a.go"}}},
	}
	g := a.graphRows()
	if len(g.lanes) != 2 {
		t.Fatalf("lanes = %d, want 2", len(g.lanes))
	}
	if !g.laneAlive(1, 4) || !g.laneAlive(1, 5) {
		t.Fatalf("branch lane must stay alive up to the merge row")
	}
	if !g.laneAbove(1, 3) || g.laneAbove(1, 5) {
		t.Fatalf("branch tip must continue upward, merge row must not")
	}
	for _, r := range g.rows {
		if r.rev == 5 && (r.lane != 0 || len(r.merges) != 1 || r.merges[0] != 1) {
			t.Fatalf("r5 = %+v, want trunk row merging lane 1", r)
		}
	}
}
