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

func TestStartWarnsWhenIgnoreCannotBeConfirmed(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, binPath, "init", dir)

	cmd := exec.Command(binPath, "start", "--addr", "127.0.0.1:0")
	cmd.Dir = dir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := cmd.StderrPipe()
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

	out := readLine(t, stdout)
	errLine := readLine(t, stderr)
	if !strings.HasPrefix(out, "listening on 127.0.0.1:") {
		t.Fatalf("listen line %q", out)
	}
	if !strings.Contains(errLine, "warning:") {
		t.Fatalf("stderr %q", errLine)
	}
}

func readLine(t *testing.T, r interface{ Read([]byte) (int, error) }) string {
	t.Helper()
	line := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(r)
		if sc.Scan() {
			line <- sc.Text()
		} else {
			line <- ""
		}
	}()
	select {
	case got := <-line:
		return got
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for output")
		return ""
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

	got := readLine(t, stdout)
	if !strings.HasPrefix(got, "listening on 127.0.0.1:") {
		t.Fatalf("listen line %q", got)
	}
}
