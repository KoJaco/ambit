package core

import "testing"

func TestStageUpdateRejectsStatus(t *testing.T) {
	idx, _ := newModel(t)
	id, err := idx.Create(CreateInput{Name: "Payments", Type: "service", Spec: "p"})
	if err != nil {
		t.Fatal(err)
	}
	st := StatusDone
	_, err = idx.Stage(StageInput{
		Source: OpUpdateNode,
		Ops: []StageOp{{
			Op:     OpUpdateNode,
			NodeID: id,
			Update: UpdateInput{Status: &st},
		}},
	})
	if err == nil {
		t.Fatal("expected status rejection")
	}
}
