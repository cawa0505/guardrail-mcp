## 1. Staging Directory Management

- [x] 1.1 新增 `setupStagingDir()` — 優先使用 `$XDG_RUNTIME_DIR/statemachine-staging/`，fallback `/tmp/statemachine-staging/`
- [x] 1.2 新增 `backupFile(src string, stagingDir string) (string, error)` — 複製到 `<staging-dir>/<safe-name>.bak`
- [x] 1.3 新增 `restoreFromBackup(backupPath, targetPath string) error` — 從備份復原
- [x] 1.4 新增 `cleanupBackup(backupPath string)` — 成功後刪除備份

## 2. apply_patch 流程改造

- [x] 2.1 `handleApplyPatch` 中：讀取原文 → `backupFile()` → 寫入原地 → compiler verify
- [x] 2.2 驗證失敗：`restoreFromBackup()` 取代記憶體復原
- [x] 2.3 驗證成功：`cleanupBackup()` 後繼續既有流程

## 3. state schema 更新

- [x] 3.1 `StagingBuf` 加入 `Dir string` 欄位記錄實際 staging 路徑
- [x] 3.2 `defaultState()` 初始值更新（不需改 — Dir runtime 決定）

## 4. 清理與驗證

- [x] 4.1 `go build ./...` 編譯通過
- [ ] 4.2 commit + push
