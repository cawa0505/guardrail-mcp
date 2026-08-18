## Purpose

提供安全的程式碼修改機制：search/replace patch 經 compiler 驗證通過後才寫入硬碟，失敗自動復原。

## Requirements

### Requirement: Search/replace patch

系統 SHALL 以 search_block 與 replace_block 進行精準字串取代。

#### Scenario: 精準取代

- **WHEN** agent 提供精準的 search_block 與 replace_block
- **THEN** 系統以 strings.Index 找到搜尋區塊並取代

#### Scenario: search_block 不匹配

- **WHEN** search_block 在檔案中不存在
- **THEN** 系統回傳 Error，不修改檔案

### Requirement: Compiler validation

系統 SHALL 在寫入 patch 後執行 compiler 驗證，驗證失敗時復原原始內容。

#### Scenario: 編譯驗證成功

- **WHEN** patch 通過 compiler 驗證
- **THEN** 系統保留變更，更新 state，背景觸發 graphify

#### Scenario: 編譯驗證失敗

- **WHEN** patch 未通過 compiler 驗證
- **THEN** 系統復原原始內容，回傳 compiler output

### Requirement: Compiler detection

系統 SHALL 依專案類型自動偵測 compiler。

#### Scenario: Rust 專案用 cargo check

- **WHEN** 專案根目錄含 Cargo.toml
- **THEN** 系統使用 `cargo check` 驗證

#### Scenario: TypeScript 專案用 tsc

- **WHEN** 專案根目錄含 tsconfig.json
- **THEN** 系統使用 `tsc --noEmit` 驗證

#### Scenario: Go 專案用 go build

- **WHEN** 專案根目錄含 go.mod
- **THEN** 系統使用 `go build ./...` 驗證

### Requirement: Project root detection

系統 SHALL 從修改檔案所在目錄向上搜尋專案根目錄（以 Cargo.toml / go.mod / tsconfig.json 等標記）。

#### Scenario: 找到專案根目錄

- **WHEN** 修改檔案位於專案子目錄
- **THEN** 系統向上搜尋找到含專案標記的目錄

### Requirement: Phase gate

apply_patch SHALL 僅在 EXECUTING 階段可用。

#### Scenario: 非 EXECUTING 拒絕

- **WHEN** agent 在非 EXECUTING 階段呼叫 apply_patch
- **THEN** 系統回傳 Error

### Requirement: Modified file tracking

系統 SHALL 記錄每個 checkpoint 中修改過的檔案列表。

#### Scenario: 記錄修改檔案

- **WHEN** apply_patch 成功
- **THEN** 系統將修改檔案加入當前 checkpoint 的 modified_files
