## 1. search_block 上下文提示

- [x] 1.1 `apply.go`：`applyPatchContent` 在 search_block 不匹配時，回傳檔案中附近行上下文

## 2. Phase gate 錯誤統一

- [x] 2.1 `main.go`：`apply_patch` phase gate 錯誤加入 `(allowed: %v)`

## 3. 驗證

- [x] 3.1 `go build ./...` 編譯通過
- [x] 3.2 commit + push