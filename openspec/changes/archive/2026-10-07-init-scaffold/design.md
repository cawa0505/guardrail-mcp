## Context

初始變更建立 GuardrailMcp 的完整骨架。四個工具（inspect_context / apply_patch / get_status / checkpoint）透過 Phase Gate 引擎控管，state.json 作為唯一事實來源。

## Goals / Non-Goals

**Goals:**
- Phase Gate 狀態機（5 階段 + action whitelist）
- state.json 讀寫 + 自動 gitignore
- inspect_context 三模式（skeleton / range / full_cleaned）
- apply_patch search/replace + compiler verify + rollback
- checkpoint 進度記錄 + phase transition
- graphify 背景觸發

**Non-Goals:**
- Tree-sitter binding（由 Graphify MCP 負責）
- Staging buffer 隔離目錄（下一變更）
- LLM Verifier（待後續變更）
- PAUSED 降級機制（待後續變更）

## Decisions

- **D-1 Go + MCP SDK**：官方 `github.com/modelcontextprotocol/go-sdk`，stdio transport。
- **D-2 直接寫入 + compiler verify**：先寫入檔案再驗證，失敗復原。ponytail：避免 staging 過早複雜化。
- **D-3 regex skeleton 替代 Tree-sitter**：依語言 regex 萃取宣告式，足夠滿足 skeleton 需求。
- **D-4 graphify 背景 goroutine**：apply_patch 後以 `go func()` 觸發 extract，不阻塞主流程。

## Risks / Trade-offs

- **[直接寫入] 原地編譯若 compiler 吃掉記憶體**：大專案 cargo check 可能耗時數秒。→ 緩解：目前可接受，未來 staging buffer 可解決。
- **[regex skeleton] 準確度低於 Tree-sitter**：可能誤判註解內的關鍵字或複雜語法。→ 緩解：足夠提供 LLM context 摘要用途，深度 AST 用 graphify。
- **[graphify 背景執行] extract 失敗不被注意**：goroutine 僅 log error。→ 緩解：ast_synced flag 讓 agent 可透過 get_status 發現。

## Open Questions

- graphify extract 是否需要 timeout 保護？（目前無 timeout）
- compiler 不存在 PATH 時是否跳過驗證或回傳 Error？（目前跳過 + warning）
