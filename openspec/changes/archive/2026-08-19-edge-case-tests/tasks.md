## 1. applyPatchContent 測試

- [ ] 1.1 search_block 精準匹配
- [ ] 1.2 search_block 不存在（含上下文提示）
- [ ] 1.3 空檔案內容
- [ ] 1.4 空 search_block
- [ ] 1.5 search_block 出現多次

## 2. phaseAllowed 測試

- [ ] 2.1 各 phase 允許的操作
- [ ] 2.2 各 phase 阻擋的操作
- [ ] 2.3 未知 phase 行為

## 3. containsGitignoreEntry 測試

- [ ] 3.1 精準匹配
- [ ] 3.2 結尾 / 變體
- [ ] 3.3 不匹配

## 4. 驗證

- [x] 4.1 `go test ./...` 通過
- [ ] 4.2 commit + push