## Why

目前專案無任何測試，核心邏輯（applyPatchContent、phaseAllowed）的邊界情況未被驗證。

## What Changes

- 新增 `main_test.go`，測試核心純函數：
  - `applyPatchContent`：search_block 不存在、search_block 一次/多次匹配、空內容
  - `phaseAllowed`：各 phase 權限驗證
  - `containsGitignoreEntry`：gitignore 匹配邏輯

## Capabilities

### New Capabilities

（無 — skip_specs: true）

### Modified Capabilities

（無）

## Impact

- `main_test.go`：新增測試檔案，無生產程式碼變更