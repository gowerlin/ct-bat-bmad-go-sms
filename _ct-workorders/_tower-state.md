# 塔台狀態快照 _tower-state.md

## 🌅 起手式（Quick Recovery）
> 最後更新：2026-10-22T19:16:31+0800

### 立即待辦
1. CH3 stories 2、3、4 已全收；下一步由使用者決定（開 story 5 模板／6 限流／9、10 重試等工單）
2. `cmd/smsd` 仍是空 registry（所有 `/v1/*` 回 401），設定檔載入 story 上線前維持
3. T9002 工單時間戳異常待使用者決定：`completed_at: 19:17:00` 晚於其收單 commit（`348f696`，19:16:08）與塔台收單當下系統時間（19:16:31），疑為 Worker 憑印象填寫；塔台未改 Worker 欄位

### 近期完成摘要
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
| **目前里程碑** | CH3：stories 2、3、4（以 e2e 測試名對應 `stories.yaml`，不用 epics 編號） |
| **塔台版本** | Control Tower v5.12.2 |

---

## 🔢 編號追蹤

| 類型 | 最大編號 |
|------|---------|
| 工單 | T0003（另有練習／額外單 T9002；T0004 留給第四章模板工單） |
| BUG | 無 |
| PLAN | 無 |
| EXP | ROUTER：001（CONCLUDED） |
| 決策 | D003 |

---

## 📊 進度快照

| 工單 | story（e2e 測試名） | 狀態 | 備註 |
|------|------|------|------|
| T0001 | 骨架／`TestE2E_Healthz` | DONE | |
| T0002 | 2：`TestE2E_MockAdapter_FaultInjection` | DONE | sizing S；commit `f12b02e`；收單重跑 `-race` 全綠 |
| T0003 | 3＋4：`TestE2E_Auth_MissingKey401`、`TestE2E_Tenant_IsolationReturns404` | DONE | sizing M；commit `4302695`；D002 |
| T9002 | 文件：spine AD-17 回寫 D003 | DONE | sizing S；product `006d2da`、工單 `348f696`；僅動 spine＋工單檔，純文件未跑 `make test` |

`affects_files` overlap check：T0002 ∩ T0003 = 空集合；兩單 commit 檔案逐一核對皆在各自 `affects_files` 內，無混入。

---

## 📝 本 Session 決策日誌
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
