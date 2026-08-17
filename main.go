// Copyright 2026 jimmy. All rights reserved.
// Use of this source code is governed by a MIT-style license.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	server := mcp.NewServer(&mcp.Implementation{Name: "statemachine", Version: "1.0.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "inspect_context",
		Description: "安全讀取檔案結構。自動透過 Tree-sitter 萃取 Function/Struct 骨架與行號，極大幅度節省 Token。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "相對於專案根目錄的檔案路徑。",
				},
				"mode": map[string]any{
					"type":    "string",
					"enum":    []string{"skeleton", "range", "full_cleaned"},
					"default": "skeleton",
				},
				"line_range": map[string]any{
					"type":     "array",
					"items":    map[string]any{"type": "integer"},
					"minItems": 2,
					"maxItems": 2,
				},
			},
			"required": []string{"path"},
		},
	}, handleInspectContext)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "apply_patch",
		Description: "套用程式碼修改。會自動通過 Compiler (cargo check / tsc) 驗證語法，通過後才寫入硬碟並記錄狀態。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "要修改的檔案路徑。",
				},
				"search_block": map[string]any{
					"type":        "string",
					"description": "原本檔案中要被替換的精準原始碼片段。",
				},
				"replace_block": map[string]any{
					"type":        "string",
					"description": "準備替換進去的新原始碼片段。",
				},
				"auto_commit_checkpoint": map[string]any{
					"type":    "boolean",
					"default": true,
				},
			},
			"required": []string{"path", "search_block", "replace_block"},
		},
	}, handleApplyPatch)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_status",
		Description: "取得當前工作流狀態、Phase 階段、以及最後一次的 Checkpoint 資訊。",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}, handleGetStatus)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "checkpoint",
		Description: "建立進度快照。將目前成功編譯的狀態與摘要寫入 state.json，建立斷點憑證。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"summary": map[string]any{
					"type":        "string",
					"description": "簡短描述此階段完成了什麼。",
				},
				"next_phase": map[string]any{
					"type": "string",
					"enum": []string{"PLANNING", "EXECUTING", "VERIFYING", "COMPLETED"},
				},
			},
			"required": []string{"summary"},
		},
	}, handleCheckpoint)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("statemachine-mcp: %v", err)
		os.Exit(1)
	}
}

// ── Tool Handlers ──

type InspectArgs struct {
	Path      string `json:"path"`
	Mode      string `json:"mode"`
	LineRange []int  `json:"line_range,omitempty"`
}

func handleInspectContext(ctx context.Context, req *mcp.CallToolRequest, args InspectArgs) (*mcp.CallToolResult, any, error) {
	return fail("inspect_context: not yet implemented")
}

type ApplyPatchArgs struct {
	Path                 string `json:"path"`
	SearchBlock          string `json:"search_block"`
	ReplaceBlock         string `json:"replace_block"`
	AutoCommitCheckpoint bool   `json:"auto_commit_checkpoint"`
}

func handleApplyPatch(ctx context.Context, req *mcp.CallToolRequest, args ApplyPatchArgs) (*mcp.CallToolResult, any, error) {
	return fail("apply_patch: not yet implemented")
}

func handleGetStatus(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	st, err := loadState()
	if err != nil {
		return fail("get_status: %v", err)
	}
	return ok(map[string]any{
		"phase":                     st.Phase,
		"active_goal":               st.ActiveGoal,
		"allowed_actions":           st.AllowedActions,
		"modified_files_in_session": modifiedFiles(st),
		"last_checkpoint_id":        lastCheckpointID(st),
	})
}

type CheckpointArgs struct {
	Summary   string `json:"summary"`
	NextPhase string `json:"next_phase"`
}

func handleCheckpoint(ctx context.Context, req *mcp.CallToolRequest, args CheckpointArgs) (*mcp.CallToolResult, any, error) {
	return fail("checkpoint: not yet implemented")
}

// ── helpers ──

func ok(data map[string]any) (*mcp.CallToolResult, any, error) {
	b, _ := json.Marshal(data)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil, nil
}

func fail(format string, a ...any) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(format, a...)}},
	}, nil, nil
}

func modifiedFiles(st *State) []string {
	var files []string
	seen := map[string]bool{}
	for _, cp := range st.Checkpoints {
		for _, f := range cp.ModifiedFiles {
			if !seen[f] {
				seen[f] = true
				files = append(files, f)
			}
		}
	}
	return files
}

func lastCheckpointID(st *State) string {
	if len(st.Checkpoints) == 0 {
		return ""
	}
	return st.Checkpoints[len(st.Checkpoints)-1].ID
}
