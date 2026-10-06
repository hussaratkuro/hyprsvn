package ui

import (
	"reflect"
	"testing"

	"github.com/hussaratkuro/hyprsvn/internal/model"
)

func TestBranchMergeRevisionsFromLog(t *testing.T) {
	entries := []model.SVNLogEntryXML{
		{Revision: 21, Author: "alice", Date: "2026-09-07T10:30:00Z", Msg: "Fix the thing\nwith detail"},
		{Revision: 10, Author: "creator", Msg: "Create branch"},
		{Revision: 18, Author: "bob", Msg: "Add tests"},
	}

	got := branchMergeRevisionsFromLog(entries)
	if len(got) != 2 {
		t.Fatalf("branchMergeRevisionsFromLog() returned %d rows, want 2", len(got))
	}
	if got[0].Revision != 21 || got[0].Msg != "Fix the thing with detail" {
		t.Fatalf("first revision = %#v, want compacted r21", got[0])
	}
	if got[1].Revision != 18 || got[1].Msg != "Add tests" {
		t.Fatalf("second revision = %#v, want r18", got[1])
	}
}

func TestBranchMergeArgs(t *testing.T) {
	const branchURL = "https://svn.example.test/repo/branches/feature"

	t.Run("single revision", func(t *testing.T) {
		want := []string{"merge", "-c", "42", branchURL + "@HEAD", "."}
		if got := branchMergeArgs(branchURL, "42", 0); !reflect.DeepEqual(got, want) {
			t.Fatalf("branchMergeArgs() = %#v, want %#v", got, want)
		}
	})

	t.Run("whole branch", func(t *testing.T) {
		want := []string{"merge", "-r", "17:HEAD", branchURL + "@HEAD", "."}
		if got := branchMergeArgs(branchURL, "", 17); !reflect.DeepEqual(got, want) {
			t.Fatalf("branchMergeArgs() = %#v, want %#v", got, want)
		}
	})
}
