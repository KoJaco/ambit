package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// mcpMode selects how `ambit init` wires up the MCP server config.
type mcpMode int

const (
	mcpLocal mcpMode = iota // command "ambit"
	mcpNpx                  // command "npx -y @kojaco/ambit"
	mcpSkip                 // write nothing
)

func ambitMCPEntry(npx bool) map[string]any {
	if npx {
		return map[string]any{"command": "npx", "args": []string{"-y", "@kojaco/ambit", "mcp"}}
	}
	return map[string]any{"command": "ambit", "args": []string{"mcp"}}
}

// mergeMCPConfig adds mcpServers.ambit to the JSON in existing (nil/empty creates a new
// document). Other servers and top-level keys are preserved; an existing ambit entry is
// never overwritten. changed is false when nothing needed adding.
func mergeMCPConfig(existing []byte, npx bool) (out []byte, changed bool, err error) {
	doc := map[string]any{}
	if len(bytes.TrimSpace(existing)) > 0 {
		if err := json.Unmarshal(existing, &doc); err != nil {
			return nil, false, fmt.Errorf("existing file is not valid JSON: %w", err)
		}
	}
	servers, ok := doc["mcpServers"].(map[string]any)
	if !ok {
		if _, present := doc["mcpServers"]; present && doc["mcpServers"] != nil {
			return nil, false, fmt.Errorf("mcpServers is not an object")
		}
		servers = map[string]any{}
	}
	if _, has := servers["ambit"]; has {
		return existing, false, nil
	}
	servers["ambit"] = ambitMCPEntry(npx)
	doc["mcpServers"] = servers
	out, err = json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return append(out, '\n'), true, nil
}

// writeMCPConfigs creates or merges .mcp.json in dir, and .cursor/mcp.json when .cursor/
// exists. It returns human-readable status lines; failures are reported, not fatal.
func writeMCPConfigs(dir string, npx bool) []string {
	targets := []string{".mcp.json"}
	if st, err := os.Stat(filepath.Join(dir, ".cursor")); err == nil && st.IsDir() {
		targets = append(targets, filepath.Join(".cursor", "mcp.json"))
	}
	var lines []string
	for _, rel := range targets {
		path := filepath.Join(dir, rel)
		existing, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			lines = append(lines, fmt.Sprintf("mcp: could not read %s: %v", rel, err))
			continue
		}
		out, changed, err := mergeMCPConfig(existing, npx)
		if err != nil {
			lines = append(lines, fmt.Sprintf("mcp: left %s unchanged: %v", rel, err))
			continue
		}
		if !changed {
			lines = append(lines, fmt.Sprintf("mcp: %s already configures ambit (unchanged)", rel))
			continue
		}
		if err := os.WriteFile(path, out, 0o644); err != nil {
			lines = append(lines, fmt.Sprintf("mcp: could not write %s: %v", rel, err))
			continue
		}
		lines = append(lines, fmt.Sprintf("mcp: configured ambit in %s", rel))
	}
	return lines
}
