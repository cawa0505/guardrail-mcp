## 1. State 結構更新

- [x] 1.1 `State` 新增 `FailedAttempts int` 欄位 (`json:"failed_attempts"`)
- [x] 1.2 `phaseActions["PAUSED"]` 加入 `checkpoint`（`{"get_status", "checkpoint"}`）

## 2. apply_patch 降級邏輯

- [x] 2.1 compiler 失敗分支：`st.FailedAttempts++`，若 >= 3 則自動 `st.Phase = "PAUSED"`，更新 `AllowedActions`，重置 `FailedAttempts`
- [x] 2.2 compiler 成功分支：`st.FailedAttempts = 0`

## 3. 清理與驗證

- [x] 3.1 `go build ./...` 編譯通過
- [ ] 3.2 commit + push