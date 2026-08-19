## Why

GuardrailMcp 的產品定位與架構邊界目前不明確。對外定位是 Guardrail MCP，但實作語言（Go）與未來方向（Rust）混雜、Graphify 的角色模糊、雙軌驗證（Hard Guard / Soft Guard）缺乏定義。本變更釐清這些邊界，確保架構文件與產品事實一致。

**產品事實：**
- 對外定位：Guardrail MCP — 透過 phase gate、policy 驗證、compiler/linter 與 code graph 來保護編碼工作流程
- Remote repository：改名為 `guardrail-mcp`
- 本地 Go module / binary / 目錄名稱：維持 `statemachine-mcp` / `GuardrailMcp`，不在本變更改名
- 核心實作語言：**Go**（已上線）
- 未來實作語言：**Rust**（方向性規劃，不可寫成現況）

## What Changes

- 重寫 `proposal.md`（本文件）— 修正產品事實，移除 Go/Rust 混淆
- 新增 `design.md` — 明確定義 statemachine 核心、Track 1 Hard Guard、Track 2 Soft Guard、Graphify 定位
- 新增 `tasks.md` — 實作階段任務清單，未決事項標記 `[待討論]`
- 新增 `specs/product-boundary/spec.md` — 命名與責任邊界規範

## Capabilities

### New Capabilities

（無 — 本變更僅建立文件，不修改程式碼）

### Modified Capabilities

（無）

## Impact

- 重寫 `openspec/changes/architecture-guidance/proposal.md`
- 新增 `openspec/changes/architecture-guidance/design.md`
- 新增 `openspec/changes/architecture-guidance/tasks.md`
- 新增 `openspec/changes/architecture-guidance/specs/product-boundary/spec.md`
- 不修改 `openspec/specs/` 下既有檔案
- 不修改 `README.md`
- 不建立抽象 interface 程式碼

## 決策紀錄（Decision Record）

- **MR-1 雙命名策略**：remote repository 改名為 `guardrail-mcp`；本地 Go module、binary、目錄名稱維持 `statemachine-mcp` / `GuardrailMcp`，不在本變更改名。
- **MR-2 雙軌制**：Track 1（Hard Guard）為同步阻擋驗證，Track 2（Soft Guard）為可配置但啟用時同步阻擋的驗證。
- **MR-3 Graphify 目前是外部 MCP**：核心不內建 tree-sitter parse，Graphify MCP 是現階段唯一的 AST provider。
- **MR-4 Graphify SDK 插件是未來項目**：未來獨立提供 Go SDK 的 Graphify plugin example，但不把 SDK 提前耦合到現在核心。
- **MR-5 缺少 Track 2 verifier 時停用而非略過**：當 policy 要求某個 Track 2 verifier 但不存在時，依 policy 決定停用該 verifier 或拒絕操作，不得默默略過。
