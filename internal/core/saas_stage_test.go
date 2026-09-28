package core

import (
	"os"
	"testing"
)

// Stage tiny SaaS (billing + app) slices into AMBIT_STAGE_DIR for visual review.
// Refresh an existing model: clear .arch/nodes/* and reset index nodes/relationships, then
// from the repo root (tests run with cwd = package dir, so pass the model path explicitly):
//
//	AMBIT_STAGE_DIR=$PWD go test ./internal/core/ -run TestStageSaaSIteration1 -count=1 -v
func TestStageSaaSIteration1(t *testing.T) {
	dir := os.Getenv("AMBIT_STAGE_DIR")
	if dir == "" {
		t.Skip("set AMBIT_STAGE_DIR to the model directory")
	}
	idx, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	id, err := idx.Stage(StageInput{
		Source:  SourceSeedModel,
		Summary: "SaaS: product shell, app, billing, payments, database",
		Ops: []StageOp{
			{
				Op: OpCreateNode,
				Create: CreateInput{
					Name: "Acme Billing",
					Type: "boundary",
					Spec: "Tiny B2B SaaS: self-serve signup, metered plans, and Stripe-backed invoices.",
				},
			},
			{
				Op: OpCreateNode,
				Create: CreateInput{
					Name:     "Web App",
					Type:     "service",
					Spec:     "Customer dashboard: auth, plan picker, usage charts, and billing portal links.",
					ParentID: NodeID("acme-billing"),
				},
			},
			{
				Op: OpCreateNode,
				Create: CreateInput{
					Name:     "Billing API",
					Type:     "service",
					Spec:     "Subscriptions, seat counts, invoice generation, and dunning hooks.",
					ParentID: NodeID("acme-billing"),
				},
			},
			{
				Op: OpCreateNode,
				Create: CreateInput{
					Name:     "Stripe Connector",
					Type:     "service",
					Spec:     "PaymentIntents, Checkout Sessions, and webhook signature verification.",
					ParentID: NodeID("acme-billing"),
				},
			},
			{
				Op: OpCreateNode,
				Create: CreateInput{
					Name:     "Postgres",
					Type:     "service",
					Spec:     "Tenants, subscriptions, usage events, and invoice line items.",
					ParentID: NodeID("acme-billing"),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("staged", id)
}

// AMBIT_STAGE_DIR=$PWD go test ./internal/core/ -run TestStageSaaSIteration2 -count=1 -v
func TestStageSaaSIteration2(t *testing.T) {
	dir := os.Getenv("AMBIT_STAGE_DIR")
	if dir == "" {
		t.Skip("set AMBIT_STAGE_DIR to the model directory")
	}
	idx, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	id, err := idx.Stage(StageInput{
		Source:  SourceSeedModel,
		Summary: "SaaS: Acme Billing service relationships",
		Ops: []StageOp{
			{
				Op:     OpSetRelationship,
				From:   NodeID("web-app"),
				To:     NodeID("billing-api"),
				Label:  "calls",
				Kind:   KindSync,
			},
			{
				Op:     OpSetRelationship,
				From:   NodeID("web-app"),
				To:     NodeID("stripe-connector"),
				Label:  "opens checkout",
				Kind:   KindAsync,
			},
			{
				Op:     OpSetRelationship,
				From:   NodeID("billing-api"),
				To:     NodeID("stripe-connector"),
				Label:  "charges via",
				Kind:   KindSync,
			},
			{
				Op:     OpSetRelationship,
				From:   NodeID("billing-api"),
				To:     NodeID("postgres"),
				Label:  "persists to",
				Kind:   KindData,
			},
			{
				Op:     OpSetRelationship,
				From:   NodeID("stripe-connector"),
				To:     NodeID("billing-api"),
				Label:  "webhook events",
				Kind:   KindAsync,
			},
			{
				Op:     OpSetRelationship,
				From:   NodeID("web-app"),
				To:     NodeID("postgres"),
				Label:  "reads session",
				Kind:   KindData,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("staged", id)
}

// AMBIT_STAGE_DIR=$PWD go test ./internal/core/ -run TestStageSaaSIteration3 -count=1 -v
func TestStageSaaSIteration3(t *testing.T) {
	dir := os.Getenv("AMBIT_STAGE_DIR")
	if dir == "" {
		t.Skip("set AMBIT_STAGE_DIR to the model directory")
	}
	idx, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	billingSpec := `## Responsibilities

- Plans: free, pro, team (seat-based)
- Lifecycle: trial → active → past_due → canceled
- Emits invoice.created and subscription.updated events

## Out of scope

- Tax calculation (handled in Stripe for now)`
	id, err := idx.Stage(StageInput{
		Source:  SourceSeedModel,
		Summary: "SaaS: usage rollup, Stripe webhooks, billing spec",
		Ops: []StageOp{
			{
				Op: OpCreateNode,
				Create: CreateInput{
					Name:     "Usage Worker",
					Type:     "worker",
					Spec:     "Rolls up meter events hourly; pushes aggregates to Billing API.",
					ParentID: NodeID("billing-api"),
				},
			},
			{
				Op: OpCreateNode,
				Create: CreateInput{
					Name:     "Stripe Webhooks",
					Type:     "worker",
					Spec:     "Handles checkout.session.completed, invoice.paid, and customer.subscription.deleted.",
					ParentID: NodeID("stripe-connector"),
				},
			},
			{
				Op:     OpUpdateNode,
				NodeID: NodeID("billing-api"),
				Update: UpdateInput{Spec: &billingSpec},
			},
			{
				Op:     OpSetRelationship,
				From:   NodeID("usage-worker"),
				To:     NodeID("billing-api"),
				Label:  "reports usage to",
				Kind:   KindAsync,
			},
			{
				Op:     OpSetRelationship,
				From:   NodeID("stripe-webhooks"),
				To:     NodeID("billing-api"),
				Label:  "forwards to",
				Kind:   KindAsync,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("staged", id)
}
