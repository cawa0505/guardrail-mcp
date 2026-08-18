## Why

apply_patch 的錯誤訊息格式不一致，且 search_block 不匹配時僅回傳「search block not found」無助於 agent 修正搜尋區塊。

## What Changes

- 統一 phase gate 錯誤格式：所有工具都顯示 `(allowed: %v)`
- `applyPatchContent` 回傳 search_block 不匹配時，附上檔案中附近行上下文
- 統一 `fail` 呼叫風格：`tool: %v` 保持一致性

## Capabilities

### New Capabilities

（無 — 純改進，skip_specs: true）

### Modified Capabilities

（無）

## Impact

- `apply.go`：`applyPatchContent` 錯誤訊息改進
- `main.go`：phase gate 錯誤格式統一