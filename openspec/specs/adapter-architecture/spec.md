# adapter-architecture Specification

## Purpose
TBD - created by archiving change rust-action-core. Update Purpose after archive.

## Requirements

### Requirement: Adapter decoupling
系統 SHALL 透過 Adapter 抽象層分離通用 Action 治理核心與特定宿主環境（如 OpenCode、Claude Code、Hermes）。

#### Scenario: 轉譯專屬操作為通用 Action
- **WHEN** 宿主環境發送特定的 MCP 工具呼叫（例如 apply_patch）
- **THEN** 對應的 Adapter 將其轉譯為標準 Action 提交給 Governance Core

### Requirement: Coding adapter operations
系統 SHALL 提供 CodingAdapter，支援程式碼編輯專屬之 AST 分析、Patch 套用與編譯驗證。

#### Scenario: 提取程式碼骨架
- **WHEN** 呼叫 inspect_context 工具
- **THEN** CodingAdapter 透過 AST 解析器提取檔案結構骨架並回傳
