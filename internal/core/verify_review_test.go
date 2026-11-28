//go:build !windows

package core

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestVerifyTimeoutClamped(t *testing.T) {
	if got := verifyTimeout(100000); got != 600*time.Second {
		t.Fatalf("100000 -> %s", got)
	}
	if got := verifyTimeout(0); got != 120*time.Second {
		t.Fatalf("0 -> %s", got)
	}
	if got := verifyTimeout(5); got != 5*time.Second {
		t.Fatalf("5 -> %s", got)
	}
}

func TestRunVerifyKillsBackgroundGrandchild(t *testing.T) {
	idx, dir := planRepo(t)
	pidFile := filepath.Join(t.TempDir(), "pid")
	mustCreate(t, idx, CreateInput{ID: "v", Name: "V", Type: "service", Verify: []string{
		"sleep 100 & echo $! > " + pidFile,
	}})
	rep, err := RunVerify(context.Background(), idx, VerifyOptions{Root: dir, NodeID: "v"})
	if err != nil {
		t.Fatal(err)
	}
	// The shell exits 0 while the grandchild holds the pipe: that is a pass, not a failure.
	if !rep.Pass || rep.Results[0].Status != VerifyPassed {
		t.Fatalf("expected pass: %+v", rep.Results[0])
	}
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		if err := syscall.Kill(pid, 0); err != nil {
			return // gone (or reaped)
		}
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("background grandchild %d survived", pid)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestCheckDependentsBudgetSkipsRemaining(t *testing.T) {
	idx, dir := planRepo(t)
	mustCreate(t, idx, CreateInput{ID: "base", Name: "Base", Type: "service"})
	for _, id := range []NodeID{"d1", "d2", "d3"} {
		mustCreate(t, idx, CreateInput{ID: id, Name: string(id), Type: "service", DependsOn: []NodeID{"base"}, Verify: []string{"sleep 3"}})
	}
	old := dependentsBudget
	dependentsBudget = 500 * time.Millisecond
	defer func() { dependentsBudget = old }()
	start := time.Now()
	rep, err := CheckDependents(context.Background(), idx, VerifyOptions{Root: dir, NodeID: "base"})
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("budget not enforced: %s", time.Since(start))
	}
	if len(rep.Skipped) != 3 || rep.Pass {
		t.Fatalf("expected all three skipped for budget: %+v", rep)
	}
	for _, s := range rep.Skipped {
		if !strings.Contains(s.Reason, "budget") {
			t.Fatalf("reason %q", s.Reason)
		}
	}
}
