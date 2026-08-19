## 1. Commit Token 安全模型

- [ ] 1.1 定義 commit token 生命週期：產生、綁定、驗證、失效
- [ ] 1.2 實作 proposal hash 綁定（token 鎖定特定 proposal）
- [ ] 1.3 實作 workspace binding（token 鎖定 workspace 路徑）
- [ ] 1.4 實作 revision binding（token 鎖定 git revision）
- [ ] 1.5 實作一次性使用（單次 commit 後自動失效）
- [ ] 1.6 實作過期機制（TTL-based）
- [ ] 1.7 實作 crash recovery：process crash 後 token 狀態應可復原
- [ ] 1.8 實作 token revocation（手動撤銷）
- [ ] 1.9 [待討論] token 過期時間的預設值與可配置方式
- [ ] 1.10 [待討論] revocation 的 UI 或 MCP tool 介面

## 2. Track 1 Hard Guard — Phase Gate

- [ ] 2.1 實作 phase gate policy engine（per-phase 允許規則，非 whitelist 硬編碼）
- [ ] 2.2 實作 phase transition validation（含 PAUSED 階段）
- [ ] 2.3 實作 action whitelist 改為可配置 policy（目前為 `switch/case` 硬編碼）
- [ ] 2.4 實作 tool handler phase gate 檢查（已在 `main.go` 實作部分，需統一入口）

## 3. Track 1 Hard Guard — Graphify Validation

- [ ] 3.1 整合 Graphify MCP 為同步 AST/code graph 驗證 provider
- [ ] 3.2 定義 degraded mode 行為（Graphify 不可用時的 policy 決定放行或拒絕）
- [ ] 3.3 實作 Graphify MCP client wrapper（go-sdk MCP client 呼叫）
- [ ] 3.4 實作逾時保護（Graphify 回應逾期的處理）
- [ ] 3.5 [待討論] degraded mode 的精確 policy 定義：哪些情境可放行、哪些必須阻擋

## 4. Track 2 Soft Guard — Provider Integration

**串接方式：** 設定檔驅動，每組 verifier 配置 URL（必要）+ API token（選用，目前暫無 auth 機制）。verifier 清單為可配置陣列。

- [x] 4.1 定義 Soft Guard verifier contract（輸入/輸出/錯誤格式）
- [x] 4.2 實作 verifier config 資料結構（URL、token、enabled、required、type）
- [x] 4.3 實作 HTTP verifier 呼叫層（POST JSON → 檢查回應）
- [ ] 4.4 實作 Docker compiler/linter verifier 串接
- [ ] 4.5 實作 docker-llm-as-a-verifier 串接（外部專案已完成 verifier 本身，本任務僅 statemachine-mcp 端整合）
- [ ] 4.6 實作 local llama-server verifier 串接（外部專案已完成 verifier 本身，本任務僅 statemachine-mcp 端整合）
- [x] 4.7 實作 verifier 配置管理（啟用/停用/required/optional）
- [x] 4.8 實作 required verifier 缺失時拒絕操作
- [x] 4.9 實作 optional verifier 跳過機制
- [ ] 4.10 [待討論] 未來 VL UI verifier 的整合方式與 contract
- [ ] 4.11 [待討論] verifier loading 機制：MCP protocol vs dynamic library

## 5. 拒絕 Payload / Micro-patching

- [x] 5.1 實作 payload 驗證（拒絕不符合規範的 patch）
- [x] 5.2 實作 micro-patching 防護（過小或無意義的 patch 應被拒絕）
- [x] 5.3 預設門檻：MinPatchLines=3, MinPatchChars=10

## 6. Remote Repository 改名

- [ ] 6.1 將 remote repository 改名為 `guardrail-mcp`（GitHub UI 操作）
- [ ] 6.2 更新 local remote URL：`git remote set-url origin git@github.com:<org>/guardrail-mcp.git`
- [ ] 6.3 [待討論] 本地 Go module path / binary name / 目錄名稱的後續相容性改名 — 是否改、何時改、改的範圍
- [ ] 6.4 [待討論] 改名後 openspec/specs 中引用舊 module path 的更新策略

## 7. README 對齊

- [ ] 7.1 更新 README 以反映 Guardrail MCP 定位（remote repo 為 guardrail-mcp）
- [ ] 7.2 更新 README 以反映目前實作語言為 Go、Rust 為未來方向
- [ ] 7.3 更新 README 以反映雙軌驗證架構

## 8. 驗證

- [ ] 8.1 `go build ./...` 編譯通過
- [ ] 8.2 `go test ./...` 測試通過
- [ ] 8.3 雙軌驗證整合 E2E 測試
- [ ] 8.4 commit + push
