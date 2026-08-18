## Context

當前 `apply_patch` 的 rollback 機制僅依靠記憶體的 `backup []byte` slice。寫入原始檔案 → compiler verify → 失敗時從記憶體復原。若 process 在寫入後、復原前 crash（OOM、SIGKILL、斷電），原始檔案損毀且無法回復。

見 `proposal.md` — Why 段落取得完整動機。

## Goals / Non-Goals

**Goals:**
- apply_patch 寫入前先建立磁碟備份
- compiler verify 通過後才移除備份
- compiler verify 失敗時從磁碟備份復原
- 備份目錄統一管理（`$XDG_RUNTIME_DIR/statemachine-staging/` 或 `/tmp/statemachine-staging/`）

**Non-Goals:**
- 不改變 apply_patch 的 MCP interface（參數、回傳值、行為不變）
- 不改變 compiler verify 的邏輯
- 不處理跨檔案 atomic commit（未來變更）

## Decisions

- **D-1 磁碟備份取代記憶體 backup**：當前 `backup := make([]byte, len(original))` 改為 `os.WriteFile(stagingPath, original, 0644)`。理由：crash-safe。
- **D-2 Staging dir 路徑**：優先使用 `$XDG_RUNTIME_DIR/statemachine-staging/`，fallback 到 `/tmp/statemachine-staging/`。理由：XDG 規範，tmpfs 在大部分 Linux 上不佔磁碟。
- **D-3 備份檔案命名**：`<staging-dir>/<absolute-path-with-slashes-replaced>.bak`。理由：避免目錄結構嵌套，一個 flat 目錄方便清理。
- **D-4 成功後立即清除**：compiler verify 通過後刪除 `.bak` 檔。理由：不殘留垃圾。
- **D-5 state.json 中 StagingDir 欄位**：`state.go` 已有 `StagingBuf` 結構，加入 `Dir string` 欄位記錄實際路徑。理由：debug 時可查詢。

## Risks / Trade-offs

- **[tmpfs 空間] 大檔案備份可能撐爆 tmpfs**：若目標檔案 > 100MB，備份可能耗盡 `/tmp`。→ 緩解：極罕見（apply_patch 通常改小型原始碼檔），可接受。
- **[flat 目錄] 檔名衝突**：兩個不同專案的同路徑檔案備份到同一個 flat 目錄。→ 緩解：極罕見（不同 session 不會同時寫同檔案），加入 PID 到檔名可解決。
- **[權限] /tmp 可能被 cleanup daemon 清除**：systemd-tmpfiles 可能清理舊檔案。→ 緩解：備份只在 compiler verify 期間短暫存在（數秒至數分鐘），不影響。

## Migration Plan

1. 修改 `apply.go`：加入 `setupStagingDir()`、`backupFile()`、`restoreFromBackup()`、`cleanupBackup()`
2. 修改 `handleApplyPatch` 流程：讀取 → 備份到 staging → 寫入原地 → compiler verify → 成功清除 / 失敗復原
3. 更新 `state.go`：`StagingBuf` 加入 `Dir string` 欄位
4. 更新 `inspect.go` 中 `main.go` 的 tool description（Tree-sitter 提及修正）
5. 驗證：`go build ./...` + 人工測試 apply_patch 流程

## Open Questions

（無）
