package mcp

import (
	"context"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestToolDescriptionsStateAdvisoryAndSubagentRules(t *testing.T) {
	idx, dir := openFixture(t)
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "ambit", Version: "v1"}, nil)
	registerTools(srv, &toolEnv{idx: idx, dir: dir})
	ctx := context.Background()
	t1, t2 := sdkmcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, t1, nil); err != nil {
		t.Fatal(err)
	}
	session, err := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test", Version: "v1"}, nil).Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	res, err := session.ListTools(ctx, &sdkmcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	seen := false
	for _, tool := range res.Tools {
		if tool.Name == "check_scope" {
			seen = true
			if !strings.Contains(tool.Description, "advisory") || !strings.Contains(tool.Description, "ambit check") {
				t.Fatalf("check_scope description: %s", tool.Description)
			}
		}
	}
	if !seen {
		t.Fatal("no check_scope")
	}
}
