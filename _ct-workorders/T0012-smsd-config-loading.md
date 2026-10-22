---
schema_version: 1
schema_kind: workorder
id: T0012
title: smsd 載入 SMS_CONFIG，組裝真實租戶與 API key（epic 1 回顧 F1）
type: impl
status: PENDING
repo: sms-platform
project: sms-platform
created_at: "2026-10-23T07:13:08+0800"
sizing: M
depends_on: []
followups:
  - T0013
affects_files:
  - cmd/smsd/config.go
  - cmd/smsd/config_test.go
  - cmd/smsd/app.go
  - cmd/smsd/app_test.go
  - cmd/smsd/main.go
  - cmd/smsd/provider.go
  - internal/tenant/tenant.go
  - internal/tenant/tenant_test.go
  - docs/smsd-config.example.json
  - docs/onboarding.md
  - .env.example
  - _ct-workorders/T0012-smsd-config-loading.md
interaction:
  mode_hint: yolo
  interactive: false
  intervention_type: fire-and-forget
renew_count: 0
worker_question_timeout: 30
workdir: main repo
---
# 工單 T0012-smsd-config-loading

## 元資料
- **工單編號**：T0012
- **任務名稱**：smsd 載入 `SMS_CONFIG`，組裝真實租戶與 API key
- **狀態**：PENDING
- **類型**：impl
- **建立時間**：2026-10-23 07:13 (UTC+8)
- **intervention_type**：fire-and-forget
- **來源**：`_bmad-output/implementation-artifacts/epic-1-retro-2026-10-23.md` Action item 1（F1）；決策 D025、D026
- **depends_on**：無（`main` 乾淨、`go test ./... -race -count=1` 全綠即可派）。**T0013 依賴本單**，兩張都動 `cmd/smsd/main.go`，不得並行
- **派發前塔台檢查**：主 checkout 工作區乾淨；本單與 T0013 的 `affects_files` 在 `cmd/smsd/main.go` 有交集，序列化（先 T0012）

## 工作量預估
- **預估規模**：中
- **Context Window 風險**：低
- **降級策略**：先完成「解析＋驗證＋啟動失敗」（`config.go`／`config_test.go`），回報 `PARTIAL`；`app.go` 組裝與文件留下一輪

## 規格層級自問（派發前）
- [x] 目標層：執行
- [x] 決策權歸屬：設定檔欄位名與驗證規則（D026）、`SMS_CONFIG` 必填（D025）、不引入 `store.TenantDirectory`（D025）已由塔台定；檔案內部拆法與函式名 Worker 可決定
- [x] 資訊完整度：Worker 讀前置條件即可獨立完成
- [x] 回頭成本：A
- [x] 記憶覆蓋：無衝突

## 任務指令

### 前置條件
- 讀 `AGENTS.md`、`_ct-workorders/_decision-log.md`（D025、D026、D013、D019）
- 讀 `_bmad-output/implementation-artifacts/epic-1-retro-2026-10-23.md` F1
- 讀 `ARCHITECTURE-SPINE.md` AD-9、AD-11、AD-15、AD-18；`epics.md` 的 Story 1.8（約 344 行）與 Story 2.1（約 419 行）
- 讀 `cmd/smsd/main.go`、`cmd/smsd/provider.go`、`internal/tenant/tenant.go`、`internal/httpapi/deps.go`

### 輸入上下文
**問題**：`cmd/smsd/main.go` 以 `tenant.NewRegistry()`（空）組裝，沒有任何 `SMS_CONFIG` 載入程式。真實 `smsd` 對所有 `/v1/*` 回 401，Webhook Dispatcher 的允許清單也是空的。E2E 全綠只因 `e2e/testutil` 直接注入租戶，沒走組裝根。

**本單範圍（D025）**：沿用現有 `tenant.Registry`，**不**引入 `store.TenantDirectory`（Spine 的目標介面，另案）。載入一次、不熱載入。

**環境變數**：`SMS_CONFIG` = 設定檔路徑，**必填**；未設定、空字串、檔案不存在、無法解析、驗證失敗 → 啟動失敗（exit 1），與 `PROVIDER` 一致，不印密鑰。

**設定檔格式（D026，欄位名照做）**：單一 JSON 檔，標準庫 `encoding/json`，`DisallowUnknownFields`：

```json
{
  "tenants": [
    {
      "id": "tenant-a",
      "callback_url": "http://127.0.0.1:9001/hook",
      "webhook_secret": "fake-webhook-secret-a",
      "rate_limit_per_min": 60,
      "api_keys": [
        { "sha256": "<64 位小寫 hex>", "disabled": false }
      ]
    }
  ]
}
```

驗證（任一不成立即啟動失敗，錯誤訊息指出是哪個欄位／第幾個租戶，但**不得回顯** `webhook_secret`、`sha256` 值或任何 API key）：
- 頂層與租戶、api_key 物件都不得有未知欄位；`tenants` 至少一個
- `id` 必填、不得重複
- `api_keys` 至少一把；`sha256` 必須是 64 位小寫 hex，跨整份設定不得重複；`disabled` 預設 `false`（API key 層級，FR-21）
- `callback_url` 選填；有填則必須是絕對 `http`／`https` URL，且 `webhook_secret` 必填（沒有 `callback_url` 的租戶不通知）
- `rate_limit_per_min` 選填，不得為負；`0`／省略 = 沿用 `ratelimit` 預設（60，D009）

**組裝**：
- 把 `main()` 的組裝抽成一個可測函式（名稱 Worker 決定，建議 `newApp(getenv func(string) string, now func() time.Time) (*app, error)`，放 `cmd/smsd/app.go`），`main()` 只剩：開 listener → 呼叫它 → `run`。T0013 的 `main_test.go` 會在這個函式上建測試，所以請讓它回傳測試需要的東西（handler、queue、store、啟動／停止 hook），細節在回報區〈產出摘要〉寫明
- `internal/tenant` 新增「以 SHA-256 hex 直接登錄 API key」的入口（現有 `AddKey` 吃明文並自己 hash，設定檔只有 hash），含「以 disabled 狀態登錄」；不改既有 `AddKey`／`Authenticate` 的行為與測試斷言
- Dispatcher 允許清單（AD-9）取自已載入租戶的 `callback_url`（現有 `tenants.CallbackURLs()`＋`callbackHosts`）；`tenant.Registry.SetCallback` 設租戶的 callback
- 設定檔只在啟動讀一次；租戶、key 物件之後不可由設定路徑再修改
- 日誌：不得含 API key、hash、`webhook_secret`、號碼、內容（AD-15）。載入成功只記租戶數量
- 更新過時註解：`main.go` 的 `TEMPORARY` 註解、`provider.go` 的 `tenant webhook URLs join it later (T0009)`
- 範例設定 `docs/smsd-config.example.json`：**只放假值**（`fake-webhook-secret-*`、`127.0.0.1` 位址；`sha256` 填一個測試用金鑰 `test-key-a` 的雜湊，並在 `docs/onboarding.md` 說明如何用標準庫或 `sha256sum` 產生）。`.env.example` 補 `SMS_CONFIG=`（必填、無預設，格式同既有鍵說明）

### 預期產出
- `cmd/smsd/config.go`＋`config_test.go`：解析、驗證、錯誤訊息不洩密的單元測試（每條驗證規則至少一個負向案例）
- `cmd/smsd/app.go`＋`app_test.go`：組裝函式與「走真實組裝根」的測試（見驗收條件 3）
- `cmd/smsd/main.go`、`provider.go` 的修改
- `internal/tenant` 的 hash 登錄入口與測試
- 範例設定與文件、`.env.example`

### 驗收條件
- [ ] `go test ./... -race -count=1` 全綠（貼最後 20 行；`-race` 無法執行時寫明原因與替代環境，不得拿掉 `-race`）
- [ ] `go build ./...`、`go vet ./...` 乾淨；`gofmt -l .` 無輸出
- [ ] **走真實組裝根**（回顧 F1 的預防條）：`cmd/smsd/app_test.go` 以 `PROVIDER=mock`＋暫存目錄中的設定檔（`t.TempDir()`）呼叫組裝函式，對其 handler 做：(a) `POST /v1/messages` 帶 `X-API-Key: test-key-a` → **202**，`GET /v1/messages/{id}` 同 key → 200；(b) 同一請求帶錯 key／空 key／`disabled: true` 的 key → 401；(c) 租戶 B 的 key 讀租戶 A 的訊息 → 404；(d) 超過 `rate_limit_per_min` → 429。號碼只用 `+8869000000xx`。此測試位於 `cmd/smsd/`，而不在 `e2e/`，因為 `package main` 無法被 `e2e/` import；理由寫進回報區（這是 `AGENTS.md`「增量附 E2E 放 `e2e/`」的有意例外）
- [ ] 啟動失敗案例各有測試：`SMS_CONFIG` 未設、檔案不存在、非法 JSON、未知欄位、`tenants` 空、重複租戶 id、重複 `sha256`、`sha256` 非 64 位 hex、有 `callback_url` 無 `webhook_secret`、`callback_url` 非絕對 http(s)、`rate_limit_per_min` 為負；每個錯誤訊息都不含測試用的 `webhook_secret`／`sha256` 值
- [ ] Dispatcher 允許清單包含已載入租戶 `callback_url` 的 host（單元測試）
- [ ] 手動驗證（貼輸出，成功跑完才寫「通過」）：以 `PROVIDER=mock`、`SMS_CONFIG=docs/smsd-config.example.json`、`LISTEN_ADDR=127.0.0.1:18081` 實際啟動編譯後的 `smsd`，用 `X-API-Key: test-key-a` `POST /v1/messages` 回 202；未設 `SMS_CONFIG` 啟動 exit 1；結束後刪除暫存執行檔，暫存檔寫在 `_bmad-output/tmp/`
- [ ] `go.mod` 沒有新增 `require`；`git grep -n 't\.Skip' -- '*.go'` exit `1`；業務邏輯無 `time.Now`（`cmd/smsd` 組裝處除外）／`time.Sleep`；測試無外網呼叫
- [ ] 既有 `e2e/`、`features/` 測試全數仍 PASS，斷言沒有被放寬
- [ ] `git diff --name-only <base>..HEAD` 只出現 `affects_files` 列出的檔案
- [ ] 回報區填好，`status` 與 body 鏡像一致；產品 commit 與工單 commit 訊息都含 `T0012`

### 不做
- 不新增 `go.mod` 的 `require`（D001）；不引入 YAML／TOML 套件
- 不引入 `store.TenantDirectory`、不重構 `tenant.Registry` 的對外介面（D025）
- 不做熱載入、租戶管理 API、設定檔寫回
- 不補 `cmd/smsd` 的關機順序／`purgeLoop`／`q.Run` 錯誤測試（T0013）；本單只為可測性抽出組裝函式
- 不改 `e2e/` 與 `e2e/architecture_test.go`（若因新檔失敗，先回報，不要放寬）
- 不改 `internal/{queue,webhook,httpapi,store,provider}` 的行為（需要改就寫進〈遭遇問題〉）
- 不改 `_ct-workorders/` 底下的塔台私有檔；不 push；不改 `_bmad/`、`.claude/`；不用 `t.Skip`、不刪測試、不放寬斷言

## Sub-session 執行指示

> 由塔台以 yolo 模式在 BAT 派工（`/ct-exec T0012`，主 checkout，不隔離）。遇到工單沒寫的取捨，選最小改動並記在回報區〈互動紀錄〉，真的卡住才走 `yolo-mode.md` § Worker 向塔台提問。
> 時間值一律用系統指令取得（`date "+%Y-%m-%dT%H:%M:%S%z"`），不要憑印象填。驗證指令成功跑完、看到輸出之後，才在回報區寫「通過」。暫存腳本寫在 `_bmad-output/tmp/`。
> Windows 跑 `-race` 要先讓 shell 找得到 gcc 並設 `CGO_ENABLED=1`（`AGENTS.md` 末段）。

---

## 回報區

> 以下由 sub-session 填寫

### 完成狀態
（DONE / FAILED / BLOCKED / PARTIAL）

### 產出摘要
（含組裝函式的名稱、回傳型別與可供 T0013 使用的 seam）

### 互動紀錄

### Renew 歷程
無

### 遭遇問題

### sprint-status.yaml 已更新
不適用

### 回報時間
