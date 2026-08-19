// Copyright 2026 jimmy Yen (cawa0505). All rights reserved.
// Use of this source code is governed by a MIT-style license.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cawa0505/guardrail-mcp/internal/apply"
	"github.com/cawa0505/guardrail-mcp/internal/inspect"
	"github.com/cawa0505/guardrail-mcp/internal/state"
	"github.com/cawa0505/guardrail-mcp/internal/token"
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

	mcp.AddTool(server, &mcp.Tool{
		Name:        "commit_token",
		Description: "管理 Commit Token 生命週期：建立、驗證、消費、撤銷。Token 是單次使用的授權憑證，綁定 proposal hash、workspace 路徑與 git revision。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"enum":        []string{"create", "validate", "consume", "revoke", "status"},
					"description": "操作類型：create（建立）、validate（驗證）、consume（消費）、revoke（撤銷）、status（查詢狀態）",
				},
				"token_id": map[string]any{
					"type":        "string",
					"description": "Token ID（validate/consume/revoke/status 需要）",
				},
				"proposal_hash": map[string]any{
					"type":        "string",
					"description": "Proposal SHA-256 hash（create 必填，validate 可選）",
				},
				"workspace_path": map[string]any{
					"type":        "string",
					"description": "Workspace 路徑（create 可選，預設為目前目錄）",
				},
				"ttl_minutes": map[string]any{
					"type":        "integer",
					"description": "Token 有效期限（分鐘，create 可選，預設 30）",
				},
			},
			"required": []string{"action"},
		},
	}, handleCommitToken)

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
	st, err := state.LoadState()
	if err != nil {
		return fail("inspect_context: %v", err)
	}
	if !state.PhaseAllowed(st.Phase, "inspect_context") {
		return fail("inspect_context: action not allowed in phase %s (allowed: %v)", st.Phase, st.AllowedActions)
	}

	mode := args.Mode
	if mode == "" {
		mode = "skeleton"
	}

	result, err := inspect.InspectFile(args.Path, mode, args.LineRange)
	if err != nil {
		return fail("inspect_context: %v", err)
	}

	return ok(map[string]any{
		"language":           result.Language,
		"total_lines":        result.TotalLines,
		"token_reduced_from": result.TokenReducedFrom,
		"token_reduced_to":   result.TokenReducedTo,
		"content":            result.Content,
		"truncated":          result.Truncated,
	})
}

type ApplyPatchArgs struct {
	Path                 string `json:"path"`
	SearchBlock          string `json:"search_block"`
	ReplaceBlock         string `json:"replace_block"`
	AutoCommitCheckpoint bool   `json:"auto_commit_checkpoint"`
}

func handleApplyPatch(ctx context.Context, req *mcp.CallToolRequest, args ApplyPatchArgs) (*mcp.CallToolResult, any, error) {
	st, err := state.LoadState()
	if err != nil {
		return fail("apply_patch: %v", err)
	}
	if !state.PhaseAllowed(st.Phase, "apply_patch") {
		return fail("apply_patch: action not allowed in phase %s (allowed: %v)", st.Phase, st.AllowedActions)
	}

	fullPath := args.Path
	if !filepath.IsAbs(args.Path) {
		cwd, err := os.Getwd()
		if err != nil {
			return fail("apply_patch: getwd: %v", err)
		}
		fullPath = filepath.Join(cwd, args.Path)
	}

	original, err := os.ReadFile(fullPath)
	if err != nil {
		return fail("apply_patch: read file: %v", err)
	}

	newContent, err := apply.ApplyPatchContent(string(original), args.SearchBlock, args.ReplaceBlock)
	if err != nil {
		return fail("apply_patch: %v", err)
	}

	stagingDir, err := apply.SetupStagingDir()
	if err != nil {
		return fail("apply_patch: %v", err)
	}

	backupPath, err := apply.BackupFile(fullPath, stagingDir)
	if err != nil {
		return fail("apply_patch: %v", err)
	}

	if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
		apply.CleanupBackup(backupPath)
		return fail("apply_patch: write file: %v", err)
	}

	projectRoot := apply.FindProjectRoot(fullPath)
	var compResult *state.CompResult
	if projectRoot != "" {
		compiler := apply.DetectCompiler(projectRoot)
		compResult = apply.RunCompiler(compiler, projectRoot)
	} else {
		compResult = &state.CompResult{Success: true, RawOutput: "no project root found, skipping compiler validation"}
	}

	if !compResult.Success {
		apply.RestoreFromBackup(backupPath, fullPath)
		apply.CleanupBackup(backupPath)

		st.FailedAttempts++
		st.StagingBuffer.LastCompilerResult = compResult

		if st.FailedAttempts >= 3 {
			st.Phase = "PAUSED"
			st.AllowedActions = state.PhaseActions["PAUSED"]
			st.FailedAttempts = 0
			state.SaveState(st)
			return fail("apply_patch: compiler validation failed 3 times — auto-transitioned to PAUSED phase. Use checkpoint to resume.\n%s", compResult.RawOutput)
		}

		state.SaveState(st)
		return fail("apply_patch: compiler validation failed (attempt %d/3):\n%s", st.FailedAttempts, compResult.RawOutput)
	}

	apply.CleanupBackup(backupPath)
	st.FailedAttempts = 0
	modifiedFile := args.Path
	relPath := args.Path
	if filepath.IsAbs(args.Path) {
		if cwd, err := os.Getwd(); err == nil {
			if rel, err := filepath.Rel(cwd, args.Path); err == nil {
				relPath = rel
			}
		}
	}
	modifiedFile = relPath

	st.StagingBuffer = state.StagingBuf{
		Dir:                stagingDir,
		HasPendingPatch:    false,
		TargetFile:         &modifiedFile,
		PatchContent:       &args.ReplaceBlock,
		LastCompilerResult: compResult,
	}

	if len(st.Checkpoints) > 0 {
		cp := &st.Checkpoints[len(st.Checkpoints)-1]
		cp.ModifiedFiles = append(cp.ModifiedFiles, modifiedFile)
	}

	st.ASTSynced = false

	if err := state.SaveState(st); err != nil {
		return fail("apply_patch: save state: %v", err)
	}

	if projectRoot != "" {
		apply.SpawnGraphifyExtract(projectRoot, modifiedFile)
	}

	return ok(map[string]any{
		"success":                  true,
		"file":                     modifiedFile,
		"compiler":                 compilerName(projectRoot),
		"compiler_output":          compResult.RawOutput,
		"ast_synced":               false,
		"graphify_extract_triggered": projectRoot != "",
	})
}

func compilerName(projectRoot string) string {
	if projectRoot == "" {
		return "none"
	}
	c := apply.DetectCompiler(projectRoot)
	if c == nil {
		return "none"
	}
	return c.Name
}

func handleGetStatus(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	st, err := state.LoadState()
	if err != nil {
		return fail("get_status: %v", err)
	}
	return ok(map[string]any{
		"phase":                     st.Phase,
		"active_goal":               st.ActiveGoal,
		"allowed_actions":           st.AllowedActions,
		"modified_files_in_session": state.ModifiedFiles(st),
		"last_checkpoint_id":        state.LastCheckpointID(st),
		"ast_synced":                st.ASTSynced,
		"commit_token":              st.CommitToken,
	})
}

type CheckpointArgs struct {
	Summary   string `json:"summary"`
	NextPhase string `json:"next_phase"`
}

func handleCheckpoint(ctx context.Context, req *mcp.CallToolRequest, args CheckpointArgs) (*mcp.CallToolResult, any, error) {
	st, err := state.LoadState()
	if err != nil {
		return fail("checkpoint: %v", err)
	}

	if !state.PhaseAllowed(st.Phase, "checkpoint") {
		return fail("checkpoint: action not allowed in phase %s (allowed: %v)", st.Phase, st.AllowedActions)
	}

	if err := state.CreateCheckpoint(st, args.Summary, args.NextPhase); err != nil {
		return fail("checkpoint: %v", err)
	}

	return ok(map[string]any{
		"success":       true,
		"checkpoint_id": state.LastCheckpointID(st),
		"phase":         st.Phase,
	})
}

// ── Commit Token Handler ──

type CommitTokenArgs struct {
	Action        string `json:"action"`
	TokenID       string `json:"token_id"`
	ProposalHash  string `json:"proposal_hash"`
	WorkspacePath string `json:"workspace_path"`
	TTLMinutes    int    `json:"ttl_minutes"`
}

func handleCommitToken(ctx context.Context, req *mcp.CallToolRequest, args CommitTokenArgs) (*mcp.CallToolResult, any, error) {
	st, err := state.LoadState()
	if err != nil {
		return fail("commit_token: %v", err)
	}

	switch args.Action {
	case "create":
		if args.ProposalHash == "" {
			return fail("commit_token create: proposal_hash is required")
		}
		wsPath := args.WorkspacePath
		if wsPath == "" {
			if cwd, err := os.Getwd(); err == nil {
				wsPath = cwd
			}
		}
		ttl := time.Duration(args.TTLMinutes) * time.Minute
		if args.TTLMinutes <= 0 {
			ttl = token.DefaultTTL
		}

		tok, err := token.New(args.ProposalHash, wsPath, ttl)
		if err != nil {
			return fail("commit_token create: %v", err)
		}

		st.CommitToken = tok
		if err := state.SaveState(st); err != nil {
			return fail("commit_token create: save state: %v", err)
		}

		return ok(map[string]any{
			"success":       true,
			"token_id":      tok.ID,
			"created_at":    tok.CreatedAt,
			"expires_at":    tok.ExpiresAt,
			"proposal_hash": tok.Bindings.ProposalHash,
			"workspace":     tok.Bindings.WorkspacePath,
			"revision":      tok.Bindings.Revision,
		})

	case "validate":
		if st.CommitToken == nil {
			return fail("commit_token validate: no active token")
		}
		if args.TokenID != "" && st.CommitToken.ID != args.TokenID {
			return fail("commit_token validate: token ID mismatch")
		}
		wsPath := args.WorkspacePath
		if wsPath == "" {
			if cwd, err := os.Getwd(); err == nil {
				wsPath = cwd
			}
		}

		if !token.IsValid(st.CommitToken) {
			var reasons []string
			if token.IsExpired(st.CommitToken) {
				reasons = append(reasons, "expired")
			}
			if st.CommitToken.Used {
				reasons = append(reasons, "already consumed")
			}
			if st.CommitToken.Revoked {
				reasons = append(reasons, "revoked")
			}
			return ok(map[string]any{
				"valid":  false,
				"reason": fmt.Sprintf("token is %s", strings.Join(reasons, ", ")),
			})
		}

		bindErr := token.ValidateBindings(st.CommitToken, args.ProposalHash, wsPath)
		if bindErr != nil {
			return ok(map[string]any{
				"valid":  false,
				"reason": bindErr.Error(),
			})
		}

		return ok(map[string]any{
			"valid": true,
		})

	case "consume":
		if st.CommitToken == nil {
			return fail("commit_token consume: no active token")
		}
		if args.TokenID != "" && st.CommitToken.ID != args.TokenID {
			return fail("commit_token consume: token ID mismatch")
		}

		// Validate bindings before consuming
		wsPath := args.WorkspacePath
		if wsPath == "" {
			if cwd, err := os.Getwd(); err == nil {
				wsPath = cwd
			}
		}
		if bindErr := token.ValidateBindings(st.CommitToken, args.ProposalHash, wsPath); bindErr != nil {
			return fail("commit_token consume: %v", bindErr)
		}

		if err := token.Consume(st.CommitToken); err != nil {
			return fail("commit_token consume: %v", err)
		}

		if err := state.SaveState(st); err != nil {
			return fail("commit_token consume: save state: %v", err)
		}

		return ok(map[string]any{
			"success":     true,
			"consumed_at": *st.CommitToken.ConsumedAt,
		})

	case "revoke":
		if st.CommitToken == nil {
			return fail("commit_token revoke: no active token")
		}
		if args.TokenID != "" && st.CommitToken.ID != args.TokenID {
			return fail("commit_token revoke: token ID mismatch")
		}

		if err := token.Revoke(st.CommitToken); err != nil {
			return fail("commit_token revoke: %v", err)
		}

		if err := state.SaveState(st); err != nil {
			return fail("commit_token revoke: save state: %v", err)
		}

		return ok(map[string]any{
			"success":    true,
			"revoked_at": *st.CommitToken.RevokedAt,
		})

	case "status":
		if st.CommitToken == nil {
			return ok(map[string]any{
				"token_present": false,
			})
		}
		return ok(map[string]any{
			"token_present": true,
			"token_id":      st.CommitToken.ID,
			"created_at":    st.CommitToken.CreatedAt,
			"expires_at":    st.CommitToken.ExpiresAt,
			"expired":       token.IsExpired(st.CommitToken),
			"used":          st.CommitToken.Used,
			"revoked":       st.CommitToken.Revoked,
			"valid":         token.IsValid(st.CommitToken),
			"bindings":      st.CommitToken.Bindings,
		})

	default:
		return fail("commit_token: unknown action %q", args.Action)
	}
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