## Purpose

提供進度快照機制，讓 LLM agent 能記錄階段性成果並轉移 Phase，實現斷點續接。

## Requirements

### Requirement: Checkpoint creation

系統 SHALL 允許 agent 建立 Checkpoint，記錄 timestamp、summary 與修改檔案列表。

#### Scenario: 建立 checkpoint

- **WHEN** agent 呼叫 checkpoint 工具並提供 summary
- **THEN** 系統建立 Checkpoint 記錄，寫入 state.json

### Requirement: Phase transition

checkpoint SHALL 支援 next_phase 參數，將 Phase 轉移至指定階段。

#### Scenario: 轉移到下一階段

- **WHEN** agent 呼叫 checkpoint 且指定 next_phase
- **THEN** 系統轉移 Phase 並更新 allowed_actions

#### Scenario: 省略 next_phase

- **WHEN** agent 呼叫 checkpoint 且未指定 next_phase
- **THEN** 系統停留在當前 Phase

### Requirement: Checkpoint ID

每個 Checkpoint SHALL 有唯一 ID（格式：`chk_YYYYMMDD_HHMMSS`）。

#### Scenario: 自動產生 ID

- **WHEN** 系統建立 Checkpoint
- **THEN** 自動產生以 chk_ 開頭的唯一 ID

### Requirement: Active goal update

建立 Checkpoint 時 SHALL 將 summary 設為 active_goal。

#### Scenario: 更新目標描述

- **WHEN** checkpoint 建立成功
- **THEN** active_goal 設為該 checkpoint 的 summary

### Requirement: Phase gate

checkpoint SHALL 在 PLANNING / EXECUTING 階段可用。

#### Scenario: INIT 階段拒絕

- **WHEN** agent 在 INIT 階段呼叫 checkpoint
- **THEN** 系統回傳 Error
