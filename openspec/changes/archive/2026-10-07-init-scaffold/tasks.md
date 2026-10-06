# Tasks — init-scaffold

> 初始變更：建立 GuardrailMcp 專案骨架與四個核心工具。

- [x] **T1 MCP server scaffold**：main.go — 註冊 inspect_context / apply_patch / get_status / checkpoint 四個工具，stdio transport
- [x] **T2 state.json 讀寫**：state.go — loadState / saveState / 預設狀態 / 自動 gitignore
- [x] **T3 Phase Gate 引擎**：main.go — phaseAllowed / 轉移規則 / action whitelist
- [x] **T4 inspect_context**：inspect.go + main.go — 語言偵測 / skeleton / range / full_cleaned / 非文字檔阻擋
- [x] **T5 apply_patch**：apply.go + main.go — search/replace / compiler detection / compiler verify / rollback
- [x] **T6 checkpoint**：main.go — checkpoint 建立 / phase transition / checkpoint ID
- [x] **T7 get_status**：main.go — phase / allowed_actions / modified_files / last_checkpoint / ast_synced
- [x] **T8 graphify 背景觸發**：apply.go — 背景 goroutine / ast_synced flag
- [x] **T9 文件**：README.md / LICENSE / openspec 文件
