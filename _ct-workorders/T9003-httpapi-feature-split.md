---
schema_version: 1
schema_kind: workorder
id: T9003
title: httpapi 拆成每功能一檔，讓 T0005～T0007 不撞檔（純結構調整）
type: refactor
status: DONE
repo: sms-platform
project: sms-platform
created_at: "2026-10-22T22:57:21+0800"
started_at: "2026-10-22T23:02:18+0800"
updated_at: "2026-10-22T23:03:48+0800"
completed_at: "2026-10-22T23:03:48+0800"
sizing: M
followups:
  - T0004
  - T0005
  - T0006
  - T0007
affects_files:
  - internal/httpapi/handler.go
  - internal/httpapi/deps.go
  - internal/httpapi/health.go
  - internal/httpapi/json.go
  - internal/httpapi/messages.go
  - internal/httpapi/messages_send.go
  - internal/httpapi/messages_get.go
  - internal/httpapi/messages_list.go
  - internal/httpapi/templates.go
  - internal/httpapi/ratelimit.go
  - internal/httpapi/errors.go
  - internal/httpapi/errors_templates.go
  - internal/httpapi/errors_ratelimit.go
  - internal/httpapi/doc.go
  - internal/httpapi/httpapi_test.go
  - _ct-workorders/T9003-httpapi-feature-split.md
interaction:
  mode_hint: off
  interactive: false
  intervention_type: fire-and-forget
renew_count: 0
workdir: main repo
---
# 工單 T9003-httpapi-feature-split

## 元資料
- **工單編號**：T9003（課程把 T0004 之後的號留給第四章功能單；練習與額外的單用 T9xxx）
- **任務名稱**：httpapi 拆成每功能一檔，讓 T0005～T0007 不撞檔
- **狀態**：DONE
- **類型**：refactor（只做結構調整，行為不變）
- **建立時間**：2026-10-22 22:57 (UTC+8)
- **intervention_type**：fire-and-forget
- **來源**：第四章開場；T0005（限流）、T0006（列表）、T0007（模板缺變數 422）之後要 yolo 同時派出，`affects_files` 不能重疊
- **相關決策**：D001、D003（單一 mux、`/v1` 每條路由各自包 `v1(...)`，不用子 mux）、D004（本單的拆法）
- **affects_files**：見 frontmatter（皆在 `internal/httpapi/` 內，加本工單檔）

## 為什麼要做

目前三張功能單看起來都會改到同一批檔案：

| 共用點 | 現況 | 撞檔的原因 |
|---|---|---|
| `handler.go` 路由表 | 一個 `NewHandler` 註冊所有路由 | 每個功能都要加路由行 |
| `handler.go` 的 `v1` 中介層串接 | `s.authenticate(s.handler(...))` 寫在每條路由上 | 限流要插在 authenticate 與 handler 之間，會改到每一行 |
| `handler.go` 的 `Deps` | 同檔 | 限流要加 Limiter、模板要加 Templates |
| `errors.go` 的 `writeError` | 一個 switch | 429（帶 `Retry-After`）、422 都要加一個 case |
| `messages.go` | 發送、查單筆、回應 DTO 同檔 | 列表（T0006）與模板渲染（T0004／T0007）都要改 |

做完後，每個功能只改自己的檔案；上表剩下必須共用的地方，由本單先預留好**接口**（stub），之後只填內容、不改共用檔。

## 任務指令

### 前置條件
- 讀 `AGENTS.md`、`_ct-workorders/_decision-log.md`（D001、D003、D004）
- 讀 `internal/httpapi/` 全部檔案、`e2e/architecture_test.go`（import 圖與禁用呼叫）
- 先跑一次 `make test`（Windows 需 `CGO_ENABLED=1` 與 MinGW gcc，見 `AGENTS.md`），確認**開工前**是全綠，並記下測試數量當基準

### 目標結構（D004：明確 stub，不用 `init()` 註冊）

不要用套件層級的 registry 加 `init()`：路由、中介層順序、錯誤對應都要在程式裡一眼看得到。拆法如下，檔名固定，後續工單依這份檔名認領：

```
internal/httpapi/
  handler.go          NewHandler：建 mux、依序呼叫各 registerXxx、回傳 envelopeFallbacks(mux)；
                      v1(h) 中介層串接（authenticate → rateLimit → h）；type server
  deps.go             Deps 結構（原樣搬移，欄位與註解不變）
  health.go           handleHealthz ＋ registerHealth
  json.go             writeJSON、maxRequestBytes（各功能共用的 JSON 小工具）
  messages.go         messageResponse ＋ toMessageResponse(store.Message)（getMessage 與之後的列表共用的 DTO）
  messages_send.go    sendRequest、sendMessage ＋ registerMessagesSend（POST /v1/messages）
  messages_get.go     getMessage ＋ registerMessagesGet（GET /v1/messages/{id}）
  messages_list.go    【stub】registerMessagesList(mux)，目前不註冊任何路由；T0006 填
  templates.go        【stub】registerTemplates(mux)，目前不註冊任何路由；T0004 填
  ratelimit.go        【stub】func (s *server) rateLimit(next http.Handler) http.Handler { return next }；T0005 填
  errors.go           writeError、writeEnvelope、invalidError、handler adapter、envelopeFallbacks（既有 case 原樣保留）
  errors_templates.go 【stub】templateError(err error) (*apiError, bool)，目前回 nil,false；T0004／T0007 填
  errors_ratelimit.go 【stub】rateLimitError(err error) (*apiError, bool)，目前回 nil,false；T0005 填
  auth.go / validate.go  不動
```

`handler.go` 的 `NewHandler` 形狀（命名可微調，但要做到「新功能只填 stub、不改本檔」）：

```go
mux := http.NewServeMux()
registerHealth(mux)
s.registerMessagesSend(mux)
s.registerMessagesGet(mux)
s.registerMessagesList(mux)   // stub，T0006
s.registerTemplates(mux)      // stub，T0004
return envelopeFallbacks(mux)
```

- **`v1` 串接**：`func (s *server) v1(h http.Handler) http.Handler` 回傳 `s.authenticate(s.rateLimit(h))`；現有兩條 `/v1/messages` 路由改成 `mux.Handle("POST /v1/messages", s.v1(s.handler(s.sendMessage)))`。此時 `rateLimit` 是直通，行為與原來完全相同。順序不得改：authenticate → rateLimit → handler（spine AD-10；無效 key 沒有桶，直接 401）。`/healthz` 不包 `v1`。
- **錯誤對應**：在 `errors.go` 新增
  ```go
  // apiError is a status/code/message (plus optional headers, e.g. Retry-After)
  // a feature maps its sentinel errors to. writeError stays the only place the
  // envelope is written (AD-8).
  type apiError struct { Status int; Code, Message string; Header http.Header }
  ```
  `writeError` 在現有 `invalidError`、`errUnauthorized`、`ErrNotFound`、`ErrQueueFull`、`ErrClosed` 之後、`default` 之前，依序詢問 `featureErrorMappers = []func(error) (*apiError, bool){templateError, rateLimitError}`；命中就套 `Header` 並以 `writeEnvelope` 輸出。既有 case 的狀態碼、code、message **一個字都不改**；未知錯誤仍是 500 固定訊息且不洩漏內部資訊。
- **`Deps`**：搬到 `deps.go`，欄位、必填檢查（`NewHandler` 對 `Queue`／`Store`／`Tenants` 為 nil 時 panic）不變。**不要**現在加 `Templates`、`Limiter`（對應套件還沒有型別；T0004、T0005 各自加）。
- 每個 stub 檔頂端用一行註解說明「哪張工單來填、為什麼是 stub」，不要寫成 TODO 雜訊；stub 不得讓 `go vet` 或 `gofmt` 報錯，也不得被新增的測試以外的東西當成已完成功能。

### 共用點處理對照（本單做的事）

| 共用點 | 本單處理 | 之後誰改 |
|---|---|---|
| 路由註冊 | 每功能一個 `registerXxx`，`NewHandler` 預先列好 | 各功能填自己的 stub／檔 |
| 中介層順序 | `v1()` 預留 `rateLimit` 位置（直通） | T0005 填 `ratelimit.go` |
| `writeError` | `featureErrorMappers` ＋ 兩個 stub mapper | T0004／T0007 填 `errors_templates.go`；T0005 填 `errors_ratelimit.go` |
| `Deps` | 獨立成 `deps.go` | T0004 加 Templates；T0005 加 Limiter（T0004 先於 T0005～T0007，不會同時改） |
| `messages.go` | 發送／查單筆／DTO 分檔 | T0006 只新增 `messages_list.go`；T0004、T0007 改 `messages_send.go` |

### 不在本單處理（只在回報區記一筆「預計誰改」）
- `cmd/smsd/main.go`、`e2e/testutil/server.go`：加 Limiter／Templates 的組裝只有 T0004 與 T0005 會碰，T0004 先做完，T0005 之後單獨改；T0006、T0007 不需要動。**本單不改這兩個檔。**
- `docs/openapi.yaml`：只有 T0006 補列表回應（T0005 的 429、T0007 的 422 已在檔內以 `$ref` 宣告）。
- 後續工單的 e2e 測試各用自己的新檔（`e2e/ratelimit_test.go`、`e2e/messages_list_test.go`、`e2e/template_*_test.go`）；需要的測試輔助函式放在該測試檔內，不要加到 `e2e/testutil/server.go`。

### 預期後續工單的 `affects_files`（驗證本單拆得夠開；塔台開單時沿用）

| 工單 | 會改的檔（互不重疊） |
|---|---|
| T0005 限流 | `internal/ratelimit/*`、`internal/httpapi/ratelimit.go`、`errors_ratelimit.go`、`deps.go`、`internal/tenant/tenant.go`（若限額欄位需要）、`cmd/smsd/main.go`、`e2e/testutil/server.go`、`e2e/ratelimit_test.go` |
| T0006 列表 | `internal/store/store.go`＋`store_test.go`、`internal/httpapi/messages_list.go`、`e2e/messages_list_test.go`、`docs/openapi.yaml` |
| T0007 缺變數 422（T0004 之後） | `internal/template/*`、`internal/httpapi/templates.go`、`errors_templates.go`、`messages_send.go`、`e2e/template_missing_variable_test.go` |

### 輸入上下文
- 這是行為不變的重構。**不得**新增路由、錯誤碼、欄位、套件或依賴；`go.mod` 維持無 `require`。
- 現有測試（`httpapi_test.go`、`e2e/*`、`queue`／`store`／`tenant`／`mock` 單元測試）一個都不能刪、不能改寬、不能 `t.Skip`。`httpapi_test.go` 只在拆檔導致編譯不過時才調整引用，不改斷言。
- 新增的測試僅限針對新接口的最小驗證（見驗收條件第 6 項），放 `httpapi_test.go`。

### 預期產出
- 上方「目標結構」列出的檔案；`handler.go`、`errors.go`、`messages.go` 縮減為各自的新職責
- `doc.go` 的 package 說明補一段「新增功能怎麼接」（填哪個 stub、`v1` 順序、`featureErrorMappers`），不超過十行
- 回報區列出：每個舊函式搬到哪個新檔的對照、測試數量前後對照

### 驗收條件
- [ ] `make test`（= `go test ./... -race`）全綠；測試總數不少於開工前基準，沒有 `t.Skip`
- [ ] `go build ./...`、`go vet ./...` 乾淨；`gofmt -l .` 無輸出；`go.mod` 無 `require`
- [ ] 行為不變：三條既有路由的狀態碼、回應 JSON、錯誤信封、404／405 攔截、401 順序（未帶 key 的未知路徑仍回 404／405 信封）與重構前一致；`e2e` 既有測試無一修改斷言
- [ ] `NewHandler` 依序呼叫各 `registerXxx`；`v1()` 順序為 authenticate → rateLimit → handler；`/healthz` 不包 `v1`
- [ ] `writeError` 既有 case 不變，新增 `apiError` 與 `featureErrorMappers`（兩個 stub 回 `nil,false`）；未知錯誤仍回 500 固定訊息
- [ ] 新增最小測試證明接口可用，並**用完還原**：例如測試內臨時傳一個回 `apiError{Status: 429, Code: "x", Header: Retry-After}` 的 mapper，確認 `writeError` 輸出對應狀態、信封與 header（不得改動全域 `featureErrorMappers` 後不還原）
- [ ] 只改 `affects_files` 列出的檔案；`git diff --name-only` 核對，**未動** `cmd/smsd/main.go`、`e2e/testutil/server.go`、`docs/openapi.yaml`、`go.mod`、`_bmad/`
- [ ] 回報區填好，`status` 與 body 鏡像一致；commit 訊息含 `T9003`

### 不做
- 不新增任何功能行為（限流、模板、列表都不實作，只留 stub）
- 不加 `Templates`／`Limiter` 欄位；不碰 `internal/` 其他套件
- 不改 `e2e/architecture_test.go`（若它因新檔失敗，先回報，不要放寬）
- 不 push；不改 `_bmad/`、`.claude/`；不改 `_ct-workorders/` 底下除本工單以外的檔案

## Sub-session 執行指示

> 在 `sms-platform/` 執行 `/ct-exec T9003`。時間值一律用系統指令取得（`date "+%Y-%m-%dT%H:%M:%S%z"`），不要憑印象填。
> 驗證指令成功跑完、看到輸出之後，才在回報區寫「通過」。暫存腳本寫在 `_bmad-output/tmp/`。

---

## 回報區

> 以下由 sub-session 填寫

### 完成狀態
DONE

### 產出摘要
- 著陸檢查：C-0 PASS（repo=`sms-platform`，basename(REPO_ROOT)=`sms-platform`）；`BAT_*`／`CT_*` 環境變數未見（非 BAT、非 worktree）。
- product commit：`0ec3854`（訊息含 T9003）。
- 基準（開工前）：`go test ./... -race -v` 共 71 行 `--- PASS`（含子測試），全綠；完工後 72 行（多 1 個新測試），全綠，無 `t.Skip`。
- `go build ./...`、`go vet ./...` 乾淨；`gofmt -l .` 無輸出；`go.mod` 無 `require`。
- 未動 `cmd/smsd/main.go`、`e2e/testutil/server.go`、`docs/openapi.yaml`、`go.mod`、`_bmad/`；`e2e/*` 與既有斷言未改。
- 新增測試 `TestWriteErrorFeatureMapperAppliesStatusEnvelopeAndHeader`（`httpapi_test.go`）：以 `t.Cleanup` 還原 `featureErrorMappers`，驗證 429、信封、`Retry-After`。

舊函式搬移對照：
| 舊位置 | 新位置 |
|---|---|
| `handler.go` `Deps` | `deps.go` |
| `handler.go` `handleHealthz` | `health.go`（加 `registerHealth`） |
| `handler.go` 路由註冊 | `handler.go` `NewHandler` 依序呼叫 `registerXxx`；新增 `v1()`（authenticate → rateLimit → handler） |
| `messages.go` `writeJSON`、`maxRequestBytes` | `json.go` |
| `messages.go` `messageResponse` | `messages.go`（新增 `toMessageResponse`） |
| `messages.go` `sendRequest`、`sendMessage` | `messages_send.go`（加 `registerMessagesSend`） |
| `messages.go` `getMessage` | `messages_get.go`（加 `registerMessagesGet`） |
| 新增 stub | `messages_list.go`、`templates.go`、`ratelimit.go`、`errors_templates.go`、`errors_ratelimit.go` |
| `errors.go` | 既有 case 不變；新增 `apiError`、`featureErrorMappers`（在 `default` 內、500 之前詢問） |
| `doc.go` | 補「新增功能怎麼接」段落（9 行） |

偏差／備註：
- `featureErrorMappers` 的詢問放在 `switch` 的 `default` 分支內（等同「所有既有 case 之後、500 之前」），既有 case 一字未動。
- 預計誰改 `cmd/smsd/main.go`、`e2e/testutil/server.go`：T0004、T0005（T0004 先）；T0006、T0007 不需動。`docs/openapi.yaml`：T0006。
- 過程中 Python 文字模式把 `doc.go`、`errors.go` 寫成 CRLF，已用 `gofmt -w` 還原為 LF 後才驗證與 commit。

### 互動紀錄

### Renew 歷程
無

### 遭遇問題
無（上述 CRLF 問題已當場修正）。

### sprint-status.yaml 已更新
不適用

### 回報時間
2026-10-22T23:03:48+0800
