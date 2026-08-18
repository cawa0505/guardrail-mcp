// Copyright 2026 jimmy. All rights reserved.
// Use of this source code is governed by a MIT-style license.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

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
	st, err := loadState()
	if err != nil {
		return fail("inspect_context: %v", err)
	}
	if !phaseAllowed(st.Phase, "inspect_context") {
		return fail("inspect_context: action not allowed in phase %s (allowed: %v)", st.Phase, st.AllowedActions)
	}

	mode := args.Mode
	if mode == "" {
		mode = "skeleton"
	}

	result, err := inspectFile(args.Path, mode, args.LineRange)
	if err != nil {
		return fail("inspect_context: %v", err)
	}

	return ok(map[string]any{
		"language":          result.Language,
		"total_lines":       result.TotalLines,
		"token_reduced_from": result.TokenReducedFrom,
		"token_reduced_to":   result.TokenReducedTo,
		"content":           result.Content,
		"truncated":         result.Truncated,
	})
}

type ApplyPatchArgs struct {
	Path                 string `json:"path"`
	SearchBlock          string `json:"search_block"`
	ReplaceBlock         string `json:"replace_block"`
	AutoCommitCheckpoint bool   `json:"auto_commit_checkpoint"`
}

func handleApplyPatch(ctx context.Context, req *mcp.CallToolRequest, args ApplyPatchArgs) (*mcp.CallToolResult, any, error) {
	st, err := loadState()
	if err != nil {
		return fail("apply_patch: %v", err)
	}
	if !phaseAllowed(st.Phase, "apply_patch") {
		return fail("apply_patch: action not allowed in phase %s", st.Phase)
	}

	// Resolve the file path
	fullPath := args.Path
	if !filepath.IsAbs(args.Path) {
		cwd, err := os.Getwd()
		if err != nil {
			return fail("apply_patch: getwd: %v", err)
		}
		fullPath = filepath.Join(cwd, args.Path)
	}

	// Read the original file
	original, err := os.ReadFile(fullPath)
	if err != nil {
		return fail("apply_patch: read file: %v", err)
	}

	// Apply the search/replace patch
	newContent, err := applyPatchContent(string(original), args.SearchBlock, args.ReplaceBlock)
	if err != nil {
		return fail("apply_patch: %v", err)
	}

	// Set up staging directory and create crash-safe backup
	stagingDir, err := setupStagingDir()
	if err != nil {
		return fail("apply_patch: %v", err)
	}

	backupPath, err := backupFile(fullPath, stagingDir)
	if err != nil {
		return fail("apply_patch: %v", err)
	}

	// Write the new content directly so the compiler sees the patched file in place
	if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
		cleanupBackup(backupPath)
		return fail("apply_patch: write file: %v", err)
	}

	// Find project root and run compiler validation
	projectRoot := findProjectRoot(fullPath)
	var compResult *CompResult
	if projectRoot != "" {
		compiler := detectCompiler(projectRoot)
		compResult = runCompiler(compiler, projectRoot)
	} else {
		compResult = &CompResult{Success: true, RawOutput: "no project root found, skipping compiler validation"}
	}

	if !compResult.Success {
		// Compiler failed — restore from disk backup, not from memory
		restoreFromBackup(backupPath, fullPath)
		cleanupBackup(backupPath)

		st.FailedAttempts++
		st.StagingBuffer.LastCompilerResult = compResult

		if st.FailedAttempts >= 3 {
			// Auto-transition to PAUSED
			st.Phase = "PAUSED"
			st.AllowedActions = phaseActions["PAUSED"]
			st.FailedAttempts = 0
			saveState(st)
			return fail("apply_patch: compiler validation failed 3 times — auto-transitioned to PAUSED phase. Use checkpoint to resume.\n%s", compResult.RawOutput)
		}

		saveState(st)
		return fail("apply_patch: compiler validation failed (attempt %d/3):\n%s", st.FailedAttempts, compResult.RawOutput)
	}

	// Compiler passed — remove the backup and keep the change
	cleanupBackup(backupPath)
	st.FailedAttempts = 0
	// Record the modified file in the staging buffer
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

	st.StagingBuffer = StagingBuf{
		Dir:                stagingDir,
		HasPendingPatch:    false,
		TargetFile:         &modifiedFile,
		PatchContent:       &args.ReplaceBlock,
		LastCompilerResult: compResult,
	}

	// Add to current checkpoint's modified_files
	if len(st.Checkpoints) > 0 {
		cp := &st.Checkpoints[len(st.Checkpoints)-1]
		cp.ModifiedFiles = append(cp.ModifiedFiles, modifiedFile)
	}

	// Mark AST as stale — graphify needs to catch up
	st.ASTSynced = false

	if err := saveState(st); err != nil {
		return fail("apply_patch: save state: %v", err)
	}

	// Spawn graphify extract in background (Phase 1: full re-extract)
	if projectRoot != "" {
		spawnGraphifyExtract(projectRoot, modifiedFile)
	}

	return ok(map[string]any{
		"success":             true,
		"file":                modifiedFile,
		"compiler":            compilerName(projectRoot),
		"compiler_output":     compResult.RawOutput,
		"ast_synced":          false,
		"graphify_extract_triggered": projectRoot != "",
	})
}

func compilerName(projectRoot string) string {
	if projectRoot == "" {
		return "none"
	}
	c := detectCompiler(projectRoot)
	if c == nil {
		return "none"
	}
	return c.Name
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
		"ast_synced":                st.ASTSynced,
	})
}

type CheckpointArgs struct {
	Summary   string `json:"summary"`
	NextPhase string `json:"next_phase"`
}

func handleCheckpoint(ctx context.Context, req *mcp.CallToolRequest, args CheckpointArgs) (*mcp.CallToolResult, any, error) {
	st, err := loadState()
	if err != nil {
		return fail("checkpoint: %v", err)
	}

	if !phaseAllowed(st.Phase, "checkpoint") {
		return fail("checkpoint: action not allowed in phase %s (allowed: %v)", st.Phase, st.AllowedActions)
	}

	// determine next phase
	nextPhase := args.NextPhase
	if nextPhase == "" {
		// default: stay in same phase
		nextPhase = st.Phase
	}

	// validate phase transition
	validTransitions := map[string][]string{
		"INIT":      {"PLANNING"},
		"PLANNING":  {"EXECUTING", "PLANNING"},
		"EXECUTING": {"VERIFYING", "EXECUTING"},
		"VERIFYING": {"COMPLETED", "EXECUTING"},
		"PAUSED":    {"PLANNING", "EXECUTING"},
		"COMPLETED": {},
	}
	allowed, found := validTransitions[st.Phase]
	if !found {
		return fail("checkpoint: unknown current phase %q", st.Phase)
	}
	valid := false
	for _, t := range allowed {
		if t == nextPhase {
			valid = true
			break
		}
	}
	if !valid {
		return fail("checkpoint: cannot transition from %s to %s (allowed: %v)", st.Phase, nextPhase, allowed)
	}

	// build checkpoint entry
	cp := Checkpoint{
		ID:        nextCheckpointID(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Summary:   args.Summary,
	}
	st.Checkpoints = append(st.Checkpoints, cp)
	st.Phase = nextPhase
	st.AllowedActions = phaseActions[nextPhase]
	st.ActiveGoal = args.Summary

	if err := saveState(st); err != nil {
		return fail("checkpoint: save state: %v", err)
	}

	return ok(map[string]any{
		"success":       true,
		"checkpoint_id": cp.ID,
		"phase":         st.Phase,
	})
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
