## Context

見 proposal.md。目前問題：
- `apply_patch` phase gate 不顯示 allowed actions（其他工具會顯示）
- search_block 不匹配只回傳泛泛訊息，無助於修正
- `fail` 呼叫風格不統一

## Goals / Non-Goals

**Goals:**
- 所有 phase gate 錯誤格式一致：`tool: action not allowed in phase %s (allowed: %v)`
- search_block 不匹配時，從檔案中擷取目標行附近的上下文行，協助 agent 定位
- 標準化 `fail` 的第一個參數格式

**Non-Goals:**
- 不改變錯誤回傳的結構（JSON 格式維持不變）
- 不修改成功路徑

## Decisions

### 1. search_block 上下文錯誤訊息

`applyPatchContent` 在 search_block 找不到時，回傳包含檔案中前後行內容的錯誤訊息，協助 agent 對比預期內容與實際內容。

格式：`search block not found in file\n\nFile content around expected location:\n<5-10 context lines>`

### 2. Phase gate 統一

將 `apply_patch` 的 phase gate 錯誤從 `"apply_patch: action not allowed in phase %s"` 改為 `"apply_patch: action not allowed in phase %s (allowed: %v)"`，與其他工具一致。

## Risks / Trade-offs

- [Risk] 上下文行可能洩漏大型檔案內容 → 限制為 5 行，符合 inspect_context 的 token 節約精神