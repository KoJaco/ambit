package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/KoJaco/ambit/internal/core"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type stageResponse struct {
	ProposalID string `json:"proposal_id"`
	Applied    bool   `json:"applied"`
	Message    string `json:"message"`
}

func stagedMessage(source, id, detail string) string {
	return fmt.Sprintf("Proposed %s as proposal %s. This has NOT been applied to the model. The architect must accept it in the ambit UI before it takes effect.", detail, id)
}

func (e *toolEnv) stage(ctx context.Context, in core.StageInput) (*sdkmcp.CallToolResult, stageResponse, error) {
	_ = ctx
	id, err := e.idx.Stage(in)
	if err != nil {
		return nil, stageResponse{}, err
	}
	out := stageResponse{
		ProposalID: id,
		Applied:    false,
		Message:    stagedMessage(in.Source, id, in.Summary),
	}
	return nil, out, nil
}

func appendWarnings(text string, idx *core.Index) string {
	warns := idx.Warnings()
	if len(warns) == 0 {
		return text
	}
	var b strings.Builder
	b.WriteString(text)
	b.WriteString("\n\nModel integrity warnings:\n")
	for _, w := range warns {
		b.WriteString("  - ")
		b.WriteString(w.String())
		b.WriteByte('\n')
	}
	return b.String()
}

func toolErr(err error) (*sdkmcp.CallToolResult, stageResponse, error) {
	return nil, stageResponse{}, err
}
