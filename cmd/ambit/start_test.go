package main

import (
	"bufio"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestStartRejectsNonLoopback(t *testing.T) {
	cmd := exec.Command(binPath, "start", "--addr", "0.0.0.0:8080")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected rejection, got %s", out)
	}
	if !strings.Contains(string(out), "not a loopback address") {
		t.Fatalf("output %s", out)
	}
}

func TestStartListensOnLoopback(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, binPath, "init", dir)

	cmd := exec.Command(binPath, "start", "--addr", "127.0.0.1:0")
	cmd.Dir = dir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})

	line := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		if sc.Scan() {
			line <- sc.Text()
		} else {
			line <- ""
		}
	}()

	select {
	case got := <-line:
		if !strings.HasPrefix(got, "listening on 127.0.0.1:") {
			t.Fatalf("listen line %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for listen line")
	}
}
