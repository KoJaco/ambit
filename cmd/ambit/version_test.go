package main

import (
	"bufio"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	out := run(t, t.TempDir(), binPath, "version")
	if strings.TrimSpace(out) != "ambit dev" {
		t.Fatalf("version output %q", out)
	}
}

func TestStartServesEmbeddedUI(t *testing.T) {
	if _, err := os.Stat("../../internal/webui/dist/index.html"); err != nil {
		t.Skip("UI bundle not built; run make build")
	}
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
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	line := readLine(t, stdout)
	addr := strings.TrimPrefix(line, "listening on ")
	_ = bufio.NewReader(stdout)
	resp, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !strings.Contains(strings.ToLower(string(body)), "<html") {
		t.Fatalf("GET /: %d %q", resp.StatusCode, body)
	}
}
