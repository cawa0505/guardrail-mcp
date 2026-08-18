## Why

當前 `apply_patch` 直接寫入原始檔案後才執行 compiler verify，若編譯器耗盡記憶體、行程被殺、或斷電中斷，原始檔案可能損毀。Staging Buffer 先在隔離目錄寫入與驗證，成功後才複製回原地，消除原地編輯的風險。

## What Changes

- `apply.go`：加入 staging 流程 — 複製檔案到 `/tmp/statemachine-staging/` → 在 staging 內 apply patch → compiler verify → 成功才複製回原地
- `state.go`：加入 `StagingDir` 欄位（可自訂路徑，預設 `$XDG_RUNTIME_DIR/statemachine-staging/` 或 `/tmp/statemachine-staging/`）
- 新增 `.openspec.yaml` 設定 `skip_specs: true`（行為規格不變，僅實作安全強化）

## Capabilities

### New Capabilities

（無 — 純實作改進，不引入新能力）

### Modified Capabilities

（無 — spec-level 行為不變：search/replace、compiler verify、rollback 從 agent 角度完全一致）

## Impact

- `apply.go`：新增 `applyWithStaging()`、`copyFile()`、`setupStagingDir()` 函式
- `state.go`：新增 `StagingDir` 設定欄位
- `openspec/changes/staging-buffer/.openspec.yaml`：設定 `skip_specs: true`
- 無外部依賴變更
