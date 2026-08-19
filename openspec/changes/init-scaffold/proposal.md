## Why

GuardrailMcp 是一個 Phase-gated 編碼工作流程 MCP server。初始變更建立專案骨架與四個核心工具：Phase Gate 引擎、inspect_context、apply_patch（含 compiler verify）、checkpoint。

## What Changes

- 新增 `phase-gate`：有限狀態機 + action whitelist + phase transition validation。
- 新增 `state-store`：`.opencode/state.json` 讀寫 + 自動 `.gitignore` 管理。
- 新增 `inspect-context`：三模式檔案讀取（skeleton/range/full_cleaned）+ regex 骨架萃取。
- 新增 `apply-patch`：search/replace patch + compiler verify + rollback + graphify 背景觸發。
- 新增 `checkpoint`：進度快照 + phase transition。
- 新增 `get-status`：查詢 Phase、修改歷史、AST 同步狀態。
- 新增 `graphify`：背景 goroutine 觸發 graphify extract。

## Capabilities

### New Capabilities

- `phase-gate`: INIT/PLANNING/EXECUTING/VERIFYING/COMPLETED 五階段狀態機，每階段 tool whitelist。
- `state-store`: JSON state 檔案讀寫，自動目錄與 gitignore 管理。
- `inspect-context`: 三種模式（skeleton/range/full_cleaned）檔案讀取，依語言 regex 骨架萃取。
- `apply-patch`: search/replace 精準取代，compiler verify（cargo/tsc/go build），失敗自動復原。
- `checkpoint`: 進度快照（timestamp + summary + modified_files），phase transition。
- `get-status`: 查詢當前 phase、allowed actions、修改歷史、AST 同步狀態。
- `graphify`: apply_patch 成功後背景觸發 graphify extract，ast_synced flag 管理。

## Impact

- 新增 4 個 Go source（main.go / state.go / apply.go / inspect.go）。
- 新增 `.opencode/state.json` 作為 Canonical State（加入 `.gitignore`）。
- 依賴 `github.com/modelcontextprotocol/go-sdk`（MCP server framework）。
- 不修改任何既有程式碼（全新專案）。

## 決策紀錄（Decision Record）

- **MR-1 Go 語言**：選擇 Go 作為實作語言。理由：Go 對 MCP protocol 支援良好（官方 SDK）、編譯為單一二進位、無 runtime 依賴。
- **MR-2 stdio transport**：使用 stdio 而非 SSE。理由：MCP client（opencode）以子行程啟動 server，stdio 是標準做法。
- **MR-3 直接寫入原地編譯**：先直接寫入原始檔案再 compiler verify（而非 staging buffer）。理由：簡化實作，compiler 失敗會自動復原。**未來改為 staging buffer 隔離目錄**。
- **MR-4 不綁 Tree-sitter**：AST 萃取委託 Graphify MCP。理由：Tree-sitter binding 增加複雜度，graphify-mcp 已提供 `graphify_skeleton_extract`。
