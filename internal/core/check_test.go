package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckReportsViolationAndLeavesLocalJSON(t *testing.T) {
	dir := commitTwoNodeRepo(t)
	idx := reopen(t, dir)
	if err := idx.SetAssignment("payments-service", time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(dir, ".arch", "local.json")
	before, err := os.ReadFile(local)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "src", "orders", "create.ts"), "changed\n")

	report, warnings, err := Check(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings %v", warnings)
	}
	if !strings.Contains(report, "src/orders/create.ts outside_scope orders-service") {
		t.Fatalf("report:\n%s", report)
	}
	after, err := os.ReadFile(local)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("local.json changed\nbefore %s\nafter %s", before, after)
	}
}

func TestCheckInformationalWhenUnassigned(t *testing.T) {
	dir := commitTwoNodeRepo(t)
	writeFile(t, filepath.Join(dir, "docs", "notes.md"), "changed\n")
	report, _, err := Check(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report, "no assignment is active") || !strings.Contains(report, "docs/notes.md informational") {
		t.Fatalf("report:\n%s", report)
	}
}

func TestCheckNodeStatusIsNotAssignment(t *testing.T) {
	dir := commitTwoNodeRepo(t)
	writeFile(t, filepath.Join(dir, "docs", "notes.md"), "changed\n")
	report, _, err := Check(dir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(report, "outside_scope") {
		t.Fatalf("status assigned was treated as an assignment:\n%s", report)
	}
}

func TestCheckDanglingAssignment(t *testing.T) {
	dir := commitTwoNodeRepo(t)
	writeFile(t, filepath.Join(dir, ".arch", "local.json"),
		"{\"assignment\":{\"node_id\":\"missing\",\"assigned_at\":\"2026-09-28T00:00:00Z\"}}\n")
	writeFile(t, filepath.Join(dir, "docs", "notes.md"), "changed\n")
	report, warnings, err := Check(dir)
	if err != nil {
		t.Fatal(err)
	}
	if report != "" || strings.Contains(report, "no assignment is active") {
		t.Fatalf("report %q", report)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "missing") || !strings.Contains(warnings[0], "does not exist") {
		t.Fatalf("warnings %v", warnings)
	}
}

func TestCheckMissingArch(t *testing.T) {
	dir := t.TempDir()
	_, _, err := Check(dir)
	if err == nil || !strings.Contains(err.Error(), ".arch") {
		t.Fatalf("err %v", err)
	}
}

func commitTwoNodeRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitInitRepo(t, dir)
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	idx := reopen(t, dir)
	if _, err := idx.Create(CreateInput{
		Name:           "Payments Service",
		Type:           "service",
		Implementation: []string{"src/payments/**"},
		Status:         StatusAssigned,
	}); err != nil {
		t.Fatal(err)
	}
	idx = reopen(t, dir)
	if _, err := idx.Create(CreateInput{
		Name:           "Orders Service",
		Type:           "service",
		Implementation: []string{"src/orders/**"},
	}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "src", "orders", "create.ts"), "base\n")
	writeFile(t, filepath.Join(dir, "docs", "notes.md"), "base\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "model")
	return dir
}
