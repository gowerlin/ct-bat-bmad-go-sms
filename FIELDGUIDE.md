# FIELDGUIDE — sms-platform 教戰手冊

> 最後更新：2026-10-23
> 技術棧：Go 1.27.3、標準庫 `net/http`（ServeMux）、in-memory store、godog（僅 `features/`）
> 章節數：3

## 目錄

- [1. 套件邊界](#1-套件邊界)
- [2. 測試規約（TestE2E_*）](#2-測試規約teste2e_)
- [3. 錯誤碼與回應信封](#3-錯誤碼與回應信封)

---

## 1. 套件邊界

### L1 設計哲學

每個套件只擁有一種變化原因；跨界需求走介面或注入，不走「順手 import」。邊界用機器守住，不靠自律。

### L2 決策樹

要放一段新邏輯？
├─ 與 HTTP 路由、JSON、狀態碼有關 → `internal/httpapi`
├─ 決定「何時送、重試幾次、何時標 failed」→ `internal/queue`
├─ 與供應商協定（送出、解析回呼）有關 → `internal/provider/<adapter>`
├─ 入站回呼套用、出站 HMAC 通知、冪等 → `internal/webhook`
├─ 模板 CRUD、`{{var}}` 渲染 → `internal/template`
├─ 限流演算法、Retry-After 計算 → `internal/ratelimit`
├─ API key 雜湊、租戶隔離 → `internal/tenant`
├─ 實體結構、持久化 → `internal/store`
├─ 組裝依賴、開 listener、優雅關機 → `cmd/smsd`（不放業務邏輯）
└─ 不確定 → 讀該套件 `doc.go` 的「owns / does not own」，兩邊都不列就先問塔台

要新增 import？
├─ 在 `allowedImports` 內 → 可以
├─ 是 `providertest` 且來自 `_test.go` → 可以（`testOnlyImports`）
└─ 其他 → 不可以；改走介面或注入，不要改 `architecture_test.go` 放行

需要目前時間？
├─ 業務邏輯 → 吃注入的 `func() time.Time`（或 queue 的 `Wait`）
└─ 建構子預設值、`cmd/smsd` 組裝處 → 可直接 `time.Now`

### L3 專案快照

**允許的依賴圖**（`e2e/architecture_test.go:21-33`，AD-19）：

| 套件 | 可 import |
|---|---|
| `httpapi` | tenant, ratelimit, template, queue, webhook, store, provider |
| `queue`、`webhook` | provider, store |
| `tenant`、`template` | store |
| `ratelimit`、`store`、`provider` | （無） |
| `provider/mock`、`provider/mitake`、`provider/providertest` | provider |

**把關測試**（同檔）：`TestArchitecture_ImportGraph`（:81）、`_TestOnlyImportsRejectedInProductCode`（:126）、`_CmdOnlyImportsInternal`（:147）、`_AdaptersOnlyImportedByRoots`（:166）、`_ForbiddenCalls`（:195）、`_GoModOnlyRequiresGodog`（:227）、`_GodogOnlyInFeatures`（:256）。

**注意事項**：
- `cmd/smsd` 與 `e2e` 是組裝根，可 import 任何 internal 套件；adapter（mock／mitake）只有組裝根能 import。
- `go.mod` 只直接依賴 godog，且只給 `features/` 用；新增外部依賴須工單明示。
- `provider` 不重試、`httpapi` 不碰 store 資料結構細節。
- 每個套件的邊界文字在各自 `internal/<pkg>/doc.go`，改邊界先改 `doc.go`。

---

## 2. 測試規約（TestE2E_*）

### L1 設計哲學

測試驗證可觀察的行為，不驗證實作；測試必須確定性——不連網、不睡真實時間、不靠執行順序。

### L2 決策樹

新增功能增量？
├─ 一律附 E2E 測試放 `e2e/`，用 `testutil.NewTestServer`
│   ├─ 要控制時間（重試 back-off、限流視窗、過期清除）→ `env.Clock`（`FakeClock`：`Advance`／`Set`／`BlockUntilWaiters`）
│   ├─ 要控制供應商回應或回呼 → `env.Provider`（`FakeProvider`：`Enqueue`／`EnqueueReport`／`EnqueueCallbackError`／`SetDelay`）
│   └─ 要讓訊息停在 queued、不被 worker 消化 → `testutil.WithoutWorker()`
├─ 純套件內邏輯（演算法、渲染）→ 該套件的 `_test.go`
├─ 新 adapter → 通過 `providertest` 契約，再加 `httptest.Server` 的 E2E
└─ 想 `t.Skip`、刪測試或放寬斷言讓門檻變綠 → 禁止；修實作或回報

測試命名？
└─ `TestE2E_<Feature>_<Behavior>`，Behavior 描述結果（如 `Returns202AndQueryable`、`Returns429WithRetryAfter`）

### L3 專案快照

**Harness**：`e2e/testutil/server.go:75`（`NewTestServer`）、`clock.go`、`provider.go`。

**固定測試資料**（`server.go`）：`TenantA`／`TenantB`、`KeyA`＝`test-key-tenant-a`／`KeyB`；`TestRateLimitPerMin` 預設 1,000,000，只有限流測試透過 `env.Tenants` 調低。

**參考測試**：
- 基本流程：`e2e/messages_test.go:55`（`TestE2E_SendMessage_Returns202AndQueryable`）
- 重試用 FakeClock：`e2e/queue_retry_test.go:47`、`:67`
- 限流：`e2e/ratelimit_test.go:27`
- 租戶隔離：`e2e/tenant_isolation_test.go:12`
- 回呼＋冪等通知：`e2e/webhook_test.go:43`
- adapter 契約：`e2e/mitake_adapter_test.go:135`

**注意事項**：
- 門檻：`make test`（`go test ./... -race`）全綠，加上 `go build ./...`、`go vet ./...` 乾淨。
- Windows 主機跑 `-race` 要 gcc 且 `CGO_ENABLED=1`；找不到 gcc 就停下回報，不要拿掉 `-race`。
- 測試資料只用假值：號碼 `+8869000000xx`，不放真實 key／token。
- godog 情境（`features/`）使用 `WithoutWorker()` 對齊 T0008 之後的行為。

---

## 3. 錯誤碼與回應信封

### L1 設計哲學

錯誤只在一個出口格式化；內部錯誤不外洩細節，客戶端只看到穩定的 code 與安全的 message。

### L2 決策樹

要回傳錯誤？
├─ 在 handler 內 `return err`，由 `s.handler(...)` 交給 `writeError`；不要自己寫 envelope
├─ 輸入驗證失敗 → `return invalid("field '%s' ...")` → 400 `invalid_request`
├─ 屬於某功能的專屬哨兵錯誤？
│   ├─ 已有 mapper（template／ratelimit／callback）→ 在該 mapper 加 case
│   └─ 新功能 → 新增 `xxxError(err) (*apiError, bool)` 並登記到 `featureErrorMappers`
├─ 需要額外標頭（如 `Retry-After`）→ 填 `apiError.Header`
└─ 其他未知錯誤 → 落到 500 `internal_error`，只記 `error_class`，message 固定 `internal error`

路由找不到或方法不對？
└─ 由 `envelopeFallbacks` 轉成 envelope（404 `not_found`；405 `invalid_request` 並保留 `Allow`）

### L3 專案快照

**信封格式**：`{"error":{"code":"…","message":"…"}}`，`Content-Type: application/json`，唯一出口 `writeEnvelope`（`internal/httpapi/errors.go:71`）。

**錯誤碼對照**：

| HTTP | code | 來源 | 位置 |
|---|---|---|---|
| 400 | `invalid_request` | `invalid(...)`、405、無效回呼 | `errors.go:47`、`:113`、`providers_callbacks.go:56` |
| 400 | `invalid_template` | `template.ErrInvalidTemplate` | `errors_templates.go:19` |
| 401 | `unauthorized` | 缺／錯／停用 API key；回呼驗證失敗 | `errors.go:49`、`providers_callbacks.go:53` |
| 404 | `not_found` | `store.ErrNotFound`、跨租戶、未知 provider／ProviderRef、未路由 | `errors.go:51`、`providers_callbacks.go:51` |
| 422 | `missing_template_variable` | `template.MissingVariableError`（message 只列變數名） | `errors_templates.go:25` |
| 429 | `rate_limited` | `ratelimit.LimitedError`，`Retry-After` 為整數秒 | `errors_ratelimit.go:20` |
| 503 | `queue_full` | `queue.ErrQueueFull`（"queue is full, retry later"）、`queue.ErrClosed`（"service shutting down"） | `errors.go:53-55` |
| 500 | `internal_error` | 其他；message 固定 `internal error` | `errors.go:43` |

**中介層順序**（固定，AD-10）：`authenticate` → `rateLimit` → handler（`handler.go:28` 的 `s.v1`）；`/healthz` 不包。

**注意事項**：
- 不要把 template body、變數值、金鑰放進 message（`templateError` 只回變數名）。
- `queue.ErrClosed` 與 `ErrQueueFull` 共用 `queue_full` code，以 message 區分；新增客戶端判斷時依 code，不依 message。
- 新增錯誤碼必須同步 `docs/openapi.yaml` 與對應的 E2E（例如 `TestE2E_Errors_UnroutedRequestsUseEnvelope`，`e2e/messages_test.go:164`）。
- 跨租戶存取回 404 而非 403，避免洩漏資源存在與否。
