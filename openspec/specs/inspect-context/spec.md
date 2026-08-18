## Purpose

提供 LLM agent 安全、節省 token 的檔案讀取能力，支援 skeleton / range / full_cleaned 三種模式。深度 AST 分析由 Graphify MCP 負責。

## Requirements

### Requirement: Language detection

系統 SHALL 依副檔名自動偵測程式語言。

#### Scenario: 副檔名對應語言

- **WHEN** 傳入 .rs 檔案
- **THEN** 語言辨識為 rust
- **WHEN** 傳入 .ts 檔案
- **THEN** 語言辨識為 typescript
- **WHEN** 傳入 .go 檔案
- **THEN** 語言辨識為 go

### Requirement: Skeleton mode

系統 SHALL 提供 skeleton 模式，移除註解、空行、重複行後回傳前 100 行，另以 regex 萃取語言宣告式骨架。

#### Scenario: skeleton 回傳精簡內容

- **WHEN** agent 呼叫 inspect_context 且 mode 為 skeleton
- **THEN** 系統回傳移除註解與空行後的精簡內容，最多 100 行

### Requirement: Range mode

系統 SHALL 提供 range 模式，精準回傳指定行號區間的原始內容。

#### Scenario: 讀取指定行區間

- **WHEN** agent 呼叫 inspect_context 且 mode 為 range 並提供 line_range
- **THEN** 系統回傳該行號區間的精確內容

### Requirement: Full cleaned mode

系統 SHALL 提供 full_cleaned 模式，移除註解與空行後回傳全文（80KB 截斷保護）。

#### Scenario: 全文清理

- **WHEN** agent 呼叫 inspect_context 且 mode 為 full_cleaned
- **THEN** 系統回移除註解與空行後的全文

### Requirement: Non-text file detection

系統 SHALL 檢查檔案是否為文字檔（無 null byte + UTF-8 有效），非文字檔應回傳 Error。

#### Scenario: 拒絕非文字檔

- **WHEN** agent 嘗試讀取二進位檔案
- **THEN** 系統回傳 Error

### Requirement: Phase gate

inspect_context SHALL 僅在 PLANNING / EXECUTING / VERIFYING 階段可用。

#### Scenario: INIT 階段拒絕

- **WHEN** agent 在 INIT 階段呼叫 inspect_context
- **THEN** 系統回傳 Error

### Requirement: Regex skeleton by language

系統 SHALL 依語言使用不同 regex 模式萃取宣告式（fn/struct/class/interface/def）。

#### Scenario: Rust 檔案萃取 fn/struct/enum

- **WHEN** agent 讀取 Rust 檔案且 mode 為 skeleton
- **THEN** 系統回傳 fn/struct/enum/trait/impl/mod 宣告行號

#### Scenario: Go 檔案萃取 func/type/struct

- **WHEN** agent 讀取 Go 檔案且 mode 為 skeleton
- **THEN** 系統回傳 func/type/struct/interface 宣告行號

#### Scenario: TypeScript 檔案萃取 function/class/interface

- **WHEN** agent 讀取 TypeScript 檔案且 mode 為 skeleton
- **THEN** 系統回傳 function/class/interface/type/import/export 宣告行號

#### Scenario: Python 檔案萃取 def/class

- **WHEN** agent 讀取 Python 檔案且 mode 為 skeleton
- **THEN** 系統回傳 def/class/import/async def/decorators 宣告行號
