# 塔台狀態快照 _tower-state.md

## 🌅 起手式（Quick Recovery）
> 最後更新：2026-10-23T00:03:50+0800（取自系統）

### 立即待辦
1. **T0005／T0006／T0007 已全部 DONE（4-10 錄製，`main` = `2b9428e`）**；主 checkout `go test ./... -race` 全綠，三個 Worker 分頁已回收，BAT 無 pending 題目。CH4 stories 5～8 完成
1b. **T0005／T0006 留下的待補（交給後續工單）**：`Deps.Limiter` 為 nil 時不限流，應在下一張動 `handler.go`／`httpapi_test.go` 的工單併入 `NewHandler` 必填檢查；`internal/ratelimit/doc.go` 仍寫 token bucket vs fixed window 待問、欄位名 `tenant.rate_limit`，`docs/error-codes.md` 429「本版已實作」仍為「否」；`e2e/testutil` 測試租戶限額設 1,000,000（夾具值，非產品規則）；T0006 E2E 以 helper `deliver` 模擬送信 worker，送信 worker 上線後改走 queue
1a. **T0004 留下的待補（交給後續工單）**：queue 送信 worker 那張單要把 `TestE2E_Template_RendersVariables` 改成用 `FakeProvider` 收到的內容驗證（D005）；`Message` 未存 `template_id`／`vars`；`handler.go` 的 nil 檢查未含 `Templates`；`validate.go` 註解過時
4. `cmd/smsd` 仍是空 registry（所有 `/v1/*` 回 401），設定檔載入 story 上線前維持
5. T9002 工單時間戳異常待使用者決定：`completed_at: 19:17:00` 晚於其收單 commit（`348f696`，19:16:08）與塔台收單當下系統時間（19:16:31），疑為 Worker 憑印象填寫；塔台未改 Worker 欄位

### 近期完成摘要
- T0005／T0006／T0007 DONE（4-10 錄製，收單 2026-10-22T23:59:57+0800～2026-10-23T00:03:16+0800）：同一 checkout 不隔離 yolo 並行；限流（D009／D010，使用者決定）、訊息列表 status 篩選、模板缺變數 422；各單乾淨匯出重驗＋主 checkout `go test ./... -race` 全綠；三個 Worker 以 `--close-worker --delay-ms 10000` 回收
- T9004 DONE（收單 2026-10-22T23:52:26+0800）：新增 `docs/dev-workflow.md` 第四章 BAT 小節；`--isolation worktree` yolo 派，product `86e3d7e`（`ct/T9004`）、工單 `47c701b`／`94fecfa`；模式 ① 合併 merge commit `6107732`，`main` ff 至 `6107732`；Worker 分頁與 worktree 已回收（D008）
- T0004 DONE（收單 2026-10-22T23:44:31+0800）：模板 CRUD 與 `{{var}}` 渲染，product commit `9c7e7aa`、工單 commit `8bd7144`／`f9598c5`；yolo 派（`--wait-started` 確認啟動 `2026-10-22T15:34:37.087Z`）；Q-1～Q-3 經 BAT 通道塔台代答（D005～D007）；塔台重跑 `CGO_ENABLED=1 go test ./... -race -count=1`、`go build`、`go vet`、`gofmt -l .` 全乾淨，`TestE2E_Template_RendersVariables`／`RejectsInvalidRequests` PASS；diff 14 檔皆在 `affects_files` 內
- T9003 DONE（收單 2026-10-22T23:07:52+0800）：httpapi 每功能一檔、預留 stub，product commit `0ec3854`、工單 commit `a020cda`／`2569672`；塔台重跑 `CGO_ENABLED=1 go test ./... -race`、`go vet`、`gofmt -l .` 全乾淨，`go.mod` 無 `require`，product diff 15 檔皆在 `affects_files` 內；D004
- 2026-10-22 開立 T0004、T0005、T0006、T0007、T9004
- T9002 DONE（收單 2026-10-22T19:16:31+0800）：spine AD-17 回寫 D003，product commit `006d2da`、工單 commit `348f696`；EXP worktree 與 `exp/router` 已由使用者清除
- EXP-ROUTER-001 CONCLUDED（2026-10-22）：標準庫 `ServeMux` 7/7 路由可表達，方案 B（chi）未實測；D003 不推翻 D001；`exp/router` 以 fast-forward 併回 main（`8f6c58e`）
- T0002 DONE（收單 2026-10-22）：mock 故障注入，commit `f12b02e`；`FailTransient`／`FailPermanent`／`Timeout`／`Delay`／`SetAfter`／`Reset`
- T0003 DONE（收單 2026-10-22）：API key 驗證與租戶隔離，commit `4302695`；D002
- T0001 DONE：Go module 骨架與 `/healthz`
- CH2 已完成 story 1（發送 API，`TestE2E_SendMessage_Returns202AndQueryable`）；`internal/{queue,store,httpapi}` 與 `e2e/architecture_test.go` 已存在
- 2026-10-22 開立 T0002、T0003

### 快速連結
- Bug Tracker → `_bug-tracker.md`（尚未建立）
- Backlog → `_backlog.md`（尚未建立）
- Decision Log → [_decision-log.md](_decision-log.md)
- Learnings → `_learnings.md`（尚未建立）

---

## 📦 基本資訊

| 欄位 | 內容 |
|------|------|
| **專案** | sms-platform（課程示範專案，Go，module `example.com/sms-platform`） |
| **目前里程碑** | CH4：stories 5、6、7、8（BAT 派工）；前一里程碑 CH3：stories 2、3、4（以 e2e 測試名對應 `stories.yaml`，不用 epics 編號） |
| **塔台版本** | Control Tower v5.12.2 |

---

## 🔢 編號追蹤

| 類型 | 最大編號 |
|------|---------|
| 工單 | T0007（另有練習／額外單 T9002、T9003、T9004） |
| BUG | 無 |
| PLAN | 無 |
| EXP | ROUTER：001（CONCLUDED） |
| 決策 | D010 |

---

## 📊 進度快照

| 工單 | story（e2e 測試名） | 狀態 | 備註 |
|------|------|------|------|
| T0001 | 骨架／`TestE2E_Healthz` | DONE | |
| T0002 | 2：`TestE2E_MockAdapter_FaultInjection` | DONE | sizing S；commit `f12b02e`；收單重跑 `-race` 全綠 |
| T0003 | 3＋4：`TestE2E_Auth_MissingKey401`、`TestE2E_Tenant_IsolationReturns404` | DONE | sizing M；commit `4302695`；D002 |
| T9004 | 文件：新增 `docs/dev-workflow.md` 第四章 BAT 小節（worktree 示範） | DONE | sizing S；隔離派工；product `86e3d7e`；模式 ① 合併 `6107732`；D008 |
| T0007 | 8：`TestE2E_Template_MissingVariableReturns422` | DONE | sizing S；yolo；product `5c6bd1f`、工單 `6072f85`；塔台乾淨匯出重驗 `-race` 全綠 |
| T0006 | 7：`TestE2E_ListMessages_FilterByStatus` | DONE | sizing M；yolo；product `a5ab662`、工單 `3f960c5`／`8b31c48`；塔台乾淨匯出重驗 `-race` 全綠 |
| T0005 | 6：`TestE2E_RateLimit_Returns429WithRetryAfter` | DONE | sizing M；yolo；product `0776289`、工單 `f27cee9`／`d723aaf`／`2b9428e`；Q-1／Q-2 BAT 通道答覆（D009、D010） |
| T0004 | 5：`TestE2E_Template_RendersVariables` | DONE | sizing L；改以 yolo 派；product `9c7e7aa`；D005～D007 |
| T9003 | 結構：httpapi 每功能一檔（前置） | DONE | sizing M；product `0ec3854`；D004；72 個 PASS 行（基準 71）；行為不變 |
| T9002 | 文件：spine AD-17 回寫 D003 | DONE | sizing S；product `006d2da`、工單 `348f696`；僅動 spine＋工單檔，純文件未跑 `make test` |

`affects_files` overlap check（2026-10-22 開單時）：T0005 ∩ T0006 ∩ T0007 兩兩皆為空集合（另對 T9004 亦為空）；T0004 與 T0005／T0006／T0007 有重疊（`deps.go`、`main.go`、`testutil/server.go`、`store.go`、`openapi.yaml`、`template.go`、`messages_send.go`…），已用 `depends_on: T0004` 序列化。

`affects_files` overlap check（舊）：T0002 ∩ T0003 = 空集合；兩單 commit 檔案逐一核對皆在各自 `affects_files` 內，無混入。

---

## 📝 本 Session 決策日誌
- D009～D010（T0005 yolo 提問，使用者決定）：桶容量＝租戶 `rate_limit_per_min`、每秒補 `/60`、租戶設定預設 60；`Retry-After` = `max(1, ceil(秒數))`
- D008（T9004）：五條直接合併判準全過，模式 ① 機械合併，yolo 斷點 E 自動 ff
- D005～D007（T0004 yolo 提問，塔台代決）：E2E 暫以 store `Body` 驗證渲染；`ErrInvalidTemplate` → 400 `invalid_template`；`vars` 為字串值物件
- D003（EXP-ROUTER-001）：路由維持標準庫 `ServeMux`，中介層函式包裝＋`chain` helper（A1），不引入 chi；不推翻 D001／AD-17
- D002（T0003）：API key 只從 header `X-API-Key` 讀，query 帶 key 視為沒帶（選項 A；B 兩處皆收、C query 回 400 已否決）

---

## ⏳ 待處理事項
- **T0003 暫時狀態**：`cmd/smsd/main.go` 先組裝空 registry，真實 `smsd` 的 `/v1/*` 全回 401，直到設定檔載入 story 上線。
- 收單時已重跑 `CGO_ENABLED=1 go test ./... -race`、`go vet ./...`、`gofmt -l .`（全乾淨），`go.mod` 無 `require`，測試無 `t.Skip`。
- 尚未跑 `*sync`：目前無 BUG／PLAN／EXP，索引無漂移。
- 本機環境：Windows 跑 `-race` 需 MinGW gcc 與 `CGO_ENABLED=1`（見 `AGENTS.md`）。

---

## 🔍 環境快照
> 最後掃描：2026-10-22T18:55:16+0800（Full Scan，首次建立）

- **Git**：分支 main，起點 commit `4c24091`（chore: 課程 sms-platform 起點（dojo-ch2）），工作區乾淨
- **BAT**：不可派發（bash 與 PowerShell 都沒有 `BAT_*` 環境變數）；auto-session = off（defaults）；手動派工
- **設定**：無 `_tower-config.yaml`、無 `~/.claude/control-tower-data/config.yaml`
- **語言／工具**：Go 1.27.3（`go.mod`，標準庫，無 `require`，D001）、GNU make；驗收門檻 `make test`（= `go test ./... -race`）
- **目錄**：`cmd/smsd`、`internal/{httpapi,queue,store,provider,provider/mock,tenant,ratelimit,template,webhook}`、`e2e`＋`e2e/testutil`、`docs`、`_bmad-output/specs/spec-sms-platform/`（SPEC、stories.yaml、error-codes、state-machines、glossary）
- **現況**：
  - `httpapi`：`GET /healthz`、`POST /v1/messages`、`GET /v1/messages/{id}`；錯誤信封單一出口 `writeError`；`withTenant` 暫以 `DefaultTenantID` 佔位（T0003 取代）
  - `provider`：介面與 `ErrTransient`／`ErrPermanent`／`ErrUnauthorized` 已有；`mock` 永遠成功（T0002 擴充）；`mitake` 尚未建立
  - `tenant`、`ratelimit`、`template`、`webhook`：只有 `doc.go`
  - `e2e/architecture_test.go` 守 import 圖、`time.Sleep(`／`time.Now(`／`http.Client{` 禁用、`go.mod` 無 `require`
- **專案規則**：`AGENTS.md` Policy（不連外網、不用真實憑證、不略過測試、不新增依賴、不動 `_bmad/`、工單只改 `affects_files`）；Known pitfalls：驗證腳本成功前不寫「通過」、暫存腳本放 `_bmad-output/tmp/`
- **全域規則**：時間戳取自系統

---

## YOLO 歷程

### 當前 Session（2026-10-22T23:34:37+0800 起，auto-session = yolo，來源 session，未存檔）
- [派工] 2026-10-22T23:34:37+0800 — T0004 `--mode yolo --no-interactive --wait-started 30000`，exit `0`，`started: 2026-10-22T15:34:37.087Z`
- [提問] 2026-10-22T23:37:23+0800 — T0004 Q-1 E2E 驗證方式 → [A] store `Body`（塔台代決，BAT 通道）
- [提問] 2026-10-22T23:37:23+0800 — T0004 Q-2 `ErrInvalidTemplate` 錯誤碼 → [A] `invalid_template`（塔台代決，BAT 通道）
- [提問] 2026-10-22T23:37:23+0800 — T0004 Q-3 `vars` 形狀 → [A] 字串值物件（塔台代決，BAT 通道）
- [收單] 2026-10-22T23:44:31+0800 — T0004 DONE

### 當前 Session（2026-10-22T23:49:34+0800 起，auto-session = yolo，來源 session，未存檔；4-06 錄製）
- [啟動] 2026-10-22T23:49:34+0800 — `--set-role tower --mode ask` exit `0` 後改 yolo，`--set-role tower --mode yolo` exit `0`
- [派工] 2026-10-22T23:49:49+0800 — T9004 `--mode yolo --no-interactive --isolation worktree --wait-started 30000`，exit `0`，`started: 2026-10-22T15:49:49.525Z`；worktree `..\.bat-worktrees\sms-platform\T9004`、branch `ct/T9004`、base `d7fd9a2d60ceb4bb1599e50a6de678b7963b7300`（log `isolation:"worktree"`，已隔離）
- [收單] 2026-10-22T23:52:26+0800 — T9004 DONE；`--worktree-status` 五條判準全過 → 模式 ①
- [斷點 E] 2026-10-22T23:52:26+0800 — 模式 ① ff 前重查判準仍成立 → 自動 `--ff-only`，`main` = `6107732`（不暫停）
- [清理] 2026-10-22T23:52:26+0800 — `--close-worker T9004 --worktree remove` exit `0`；整合 worktree detach；分支保留待使用者同意
- [清理] 2026-10-22T23:54:02+0800 — 使用者同意，`git branch -d ct/T9004 ct-merge/T9004` exit `0`（兩者皆已併入 `main`）

### 當前 Session（2026-10-22T23:56:39+0800 起，auto-session = yolo，來源 session，未存檔；4-10 錄製）
- [啟動] 2026-10-22T23:56:39+0800 — `--set-role tower --mode ask` exit `0` 後改 yolo，`--set-role tower --mode yolo` exit `0`
- [overlap] 2026-10-22T23:56:59+0800 — T0005／T0006／T0007 `affects_files` 兩兩無完全相同路徑、皆為檔案層標註 → PASS×3；`worker_question_timeout: 30` 寫入三張（`24535d9`）
- [派工] 2026-10-22T23:57:12+0800 — T0005 `--mode yolo --no-interactive --wait-started 30000`（不隔離），exit `0`，`started: 2026-10-22T15:57:12.434Z`
- [派工] 2026-10-22T23:57:19+0800 — T0006 同上，exit `0`，`started: 2026-10-22T15:57:19.768Z`
- [派工] 2026-10-22T23:57:26+0800 — T0007 同上，exit `0`，`started: 2026-10-22T15:57:26.482Z`
- [提問] 2026-10-22T23:58:42+0800 — T0005 Q-1 桶容量／補充速率／限額來源 → [A] 容量＝租戶 `rate_limit_per_min`、每秒補 `/60`、租戶設定預設 60（使用者決定，BAT 通道 `--answer` exit `0`；D009）
- [提問] 2026-10-22T23:58:42+0800 — T0005 Q-2 `Retry-After` 取整 → [A] `max(1, ceil(秒數))`（使用者決定，BAT 通道 `--answer` exit `0`；D010）
- [收單] 2026-10-22T23:59:57+0800 — T0007 DONE；`5c6bd1f`＋`6072f85` 兩 commit 檔案皆在 `affects_files`；`6072f85` 乾淨匯出重跑 build／vet／gofmt／`go test ./... -race -count=1` 全綠
- [回收] 2026-10-22T23:59:57+0800 — `--close-worker T0007 --delay-ms 10000` exit `0`
- [收單] 2026-10-23T00:02:48+0800 — T0006 DONE；`3f960c5`／`a5ab662`／`8b31c48` 檔案皆在 `affects_files`；`a5ab662` 乾淨匯出重驗全綠
- [回收] 2026-10-23T00:02:48+0800 — `--close-worker T0006 --delay-ms 10000` exit `0`
- [收單] 2026-10-23T00:03:07+0800 — T0005 DONE；`f27cee9`／`0776289`／`d723aaf`／`2b9428e` 檔案皆在 `affects_files`；實作與 D009／D010 一致；`2b9428e` 乾淨匯出重驗全綠
- [回收] 2026-10-23T00:03:16+0800 — `--close-worker T0005 --delay-ms 10000` exit `0`
- [終驗] 2026-10-23T00:03:50+0800 — 主 checkout（`2b9428e`，工作區乾淨）`go test ./... -race` exit `0`；`bat_list_terminals` 三個 Worker 皆 `alive: false`；`bat_list_questions status=all` 兩題皆 `answered`、無 pending

### 計數器
- Renew：0／3；連續 FAILED：0／3
- 4-10 session：派發 3 張、全 DONE；斷點 A×0 B×0 C×0 D×0 E×0；提問 2（BAT 通道，使用者決定）
