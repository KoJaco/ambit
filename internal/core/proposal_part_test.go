package core

import (
	"errors"
	"testing"
)

func stagePart(t *testing.T, idx *Index, p Part) string {
	t.Helper()
	id, err := idx.Stage(StageInput{Source: OpSetPart, Summary: "part", Ops: []StageOp{{Op: OpSetPart, Part: &p}}})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestSetPartProposalAccepts(t *testing.T) {
	idx, dir := newModel(t)
	id := stagePart(t, idx, Part{ID: "api", Name: "API", Include: []string{"api/**"}})
	if len(idx.Parts) != 0 {
		t.Fatal("staging must not change config")
	}
	diff, err := idx.ProposalDiff(id)
	if err != nil || len(diff.Operations) != 1 {
		t.Fatal(err, diff)
	}
	row := diff.Operations[0]
	if row.Op != OpSetPart || row.ProposedPart == nil || row.ProposedPart.ID != "api" || row.CurrentPart != nil || row.Current != nil {
		t.Fatalf("%+v", row)
	}
	res, err := idx.AcceptAll(id)
	if err != nil || len(res.Applied) != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	idx = reopen(t, dir)
	if len(idx.Parts) != 1 || idx.Parts[0].Include[0] != "api/**" {
		t.Fatalf("%+v", idx.Parts)
	}

	del, err := idx.Stage(StageInput{Source: OpDeletePart, Ops: []StageOp{{Op: OpDeletePart, Part: &Part{ID: "api"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := idx.AcceptAll(del); err != nil {
		t.Fatal(err)
	}
	if idx = reopen(t, dir); len(idx.Parts) != 0 {
		t.Fatalf("%+v", idx.Parts)
	}
}

func TestPartProposalValidationAndStaleness(t *testing.T) {
	idx, _ := newModel(t)
	bad := func(op StageOp) {
		t.Helper()
		if _, err := idx.Stage(StageInput{Source: op.Op, Ops: []StageOp{op}}); err == nil {
			t.Fatalf("%+v should fail", op)
		}
	}
	bad(StageOp{Op: OpSetPart})
	bad(StageOp{Op: OpSetPart, Part: &Part{ID: "Bad Id"}})
	bad(StageOp{Op: OpSetPart, Part: &Part{ID: "x", Include: []string{"[bad"}}})
	bad(StageOp{Op: OpDeletePart, Part: &Part{ID: "nope"}})
	if _, err := idx.Stage(StageInput{Source: OpSetPart, Ops: []StageOp{
		{Op: OpSetPart, Part: &Part{ID: "a"}}, {Op: OpSetPart, Part: &Part{ID: "b"}},
	}}); !errors.Is(err, ErrBadOperation) {
		t.Fatalf("one part op per proposal: %v", err)
	}

	first := stagePart(t, idx, Part{ID: "one"})
	second := stagePart(t, idx, Part{ID: "two"})
	if _, err := idx.AcceptAll(first); err != nil {
		t.Fatal(err)
	}
	res, err := idx.AcceptAll(second)
	if err != nil || len(res.Skipped) != 1 || len(res.Applied) != 0 {
		t.Fatalf("second should be stale: %+v %v", res, err)
	}
	// An unrelated config change (the stage) does not make a part op stale.
	third := stagePart(t, idx, Part{ID: "three"})
	if err := idx.SetConfigKeys(map[string]any{"stage": "build"}); err != nil {
		t.Fatal(err)
	}
	if res, err := idx.AcceptAll(third); err != nil || len(res.Applied) != 1 {
		t.Fatalf("%+v %v", res, err)
	}
}
