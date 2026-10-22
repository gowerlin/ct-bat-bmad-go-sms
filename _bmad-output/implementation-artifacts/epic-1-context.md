# Epic 1 Context: 發送核心

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

讓訊息「送得出去」：租戶送出並查詢訊息、依 Status 列表篩選、用模板送信；Provider 暫時失敗時自動重試，失敗原因可在閘道單一處查證；Provider 回呼更新狀態，租戶對同一狀態只收到一次簽章通知；`mock` 與 `mitake` 兩個 Adapter 通過同一組契約測試。本 Epic 不含 API key 驗證與限流，以固定的預設租戶運作（Epic 2 在不改變 `store` 介面語意下換成真實租戶）。

## Stories

- Story 1.1: 送出訊息並可查詢
- Story 1.2: mock Provider 故障注入
- Story 1.3: 依 status 列表篩選
- Story 1.4: 模板渲染變數
- Story 1.5: 模板缺變數回 422
- Story 1.6: 重試後成功
- Story 1.7: 重試用盡標記 failed
- Story 1.8: 回呼更新狀態並只通知租戶一次
- Story 1.9: mitake 風格 Adapter 契約測試

## Requirements & Constraints

- 送出訊息回 202 與 `msg_` 前綴 `id`，Status 起始為 `queued`；欄位缺漏、號碼非 E.164、內容超過 70 個 Unicode code point 回 400 `invalid_request`。查無資料回 404 `not_found`。
- 列表：由新到舊（`created_at, id`），limit 預設 50、上限 200，cursor 為不透明字串；不合法 `status` 回 400。
- Status 流程：`queued`→`sending`→`sent`→`delivered`／`undelivered`；重試用盡或永久失敗由 `sending` 直接進 `failed`。終態不可再改變。
- 重試：暫時失敗對同一 Provider 最多重試 3 次（共 4 次呼叫），永久失敗不重試，不跨 Provider failover。`attempts` 為對 Provider 的呼叫次數；`failed`／`undelivered` 必帶非空 `failure_reason`（重試用盡、永久失敗、回呼逾時），不含號碼與內容。
- 模板：CRUD（`tpl_` 前綴 id，無法解析回 400 `invalid_template`）；渲染在入佇列之前；缺變數回 422 `missing_template_variable` 並列出變數名且不建立訊息；多餘變數不報錯；渲染結果走同一個長度驗證。
- 回呼：只接受 `delivered`／`undelivered`；驗證失敗 401、狀態值非法 400、對應不到 404；重複或終態後的回呼回 200 且不改 Status。
- Webhook：`sent`／`delivered`／`undelivered`／`failed` 才通知，body 固定 `{"id","status"}`，HMAC-SHA256 放 `X-Signature`；以 `(message_id, status)` 去重；租戶端非 2xx 重試 3 次，用盡只記日誌。
- 所有 4xx／5xx 一律 `{"error":{"code","message"}}`（含 ServeMux 預設 404／405；405 帶 `Allow`；500 訊息固定不洩漏內部資訊）。錯誤碼清單需文件化，變更視為相容性變更。
- 重啟後未終態訊息不保留（查詢 404），限制須寫入租戶接入文件。
- 跨 Story 約束：日誌不得含號碼、內容、API key、密鑰；`POST /v1/messages` p95 < 200 ms（以 `mock` 量測）；`go test ./... -race` 零失敗；E2E 不依賴外網，且除 Provider 位址與租戶 Webhook URL 外不對其他目的地發請求。
- 每個 Story 的最後一條 AC 為對應的 `TestE2E_*`；`docs/openapi.yaml` 隨端點維護。

## Technical Decisions

- 只用標準庫，`go.mod` 不得有 `require`。無 starter；Story 1.1 是補齊骨架與組裝根。
- 架構測試（`e2e/`，納入 `make test`）檢查 import 依賴圖、`http.Client{`／`http.DefaultClient`／`time.Now(`／`time.Sleep(` 出現位置、`go.mod` 無 `require`。
- 時間與等待只經注入的 `Now`、`Wait`；`e2e/testutil.FakeClock` 需補 `Wait`。ID 用 `crypto/rand`。
- `provider.Provider` 僅 `Send`、`ParseCallback`、`Name`；Adapter 不重試；error wrap `ErrTransient`／`ErrPermanent`；契約測試在 `provider/providertest`；組裝根以外不得 import `mock`／`mitake`。`PROVIDER=mock|mitake`，非法值啟動失敗；`mitake` 讀 `PROVIDER_URL`。
- `queue.Queue`（`Submit`、`Run`、`Close`）：`Submit` 為預留→寫 store→放入的單一操作；容量 1000，滿載 503 `queue_full` 且不留訊息；4 個 worker；重試在同 worker 內以 1s／2s／4s 退避，不重新入列。
- `store`：`TenantScoped`（首參數 tenantID，只從 context 取）與 `System` 分離；單一狀態轉移表，compare-and-set；轉移與 `OutboundNotification` 在同一鎖內寫入；`provider_ref`、`attempts` 與 `sending→sent` 同次 CAS；`failed` 同次 CAS 帶 `failure_reason`。終態保留 24 小時，`PurgeExpired(now)` 移除；`sent` 逾 24 小時無回呼轉 `undelivered`（`failure_reason`「回呼逾時」）。背景 goroutine 只定時呼叫，時間經注入。
- webhook 分 `Inbound`（回呼處理）與 `Dispatcher`（單 goroutine、outbox `ClaimNext`、同一訊息依序送出）；回呼驗證在 `ParseCallback`（共用密鑰 `PROVIDER_CALLBACK_SECRET`、constant-time）；路徑 `POST /v1/providers/{name}/callbacks`，不經 API key；`CallbackSink` 讓 `mock` 回呼走真實 handler。
- `http.Client` 只在 `cmd/smsd` 建立並注入，Transport 以允許清單（Provider 位址＋租戶 Webhook URL）把關。
- 單一 `writeError` 在 `httpapi`；驗證（E.164、70 code point）集中單一函式；先 `template.Render` 再 `queue.Submit`。
- 設定：單一 JSON 檔 `SMS_CONFIG`，啟動載入一次，未知欄位或缺必填即啟動失敗；本 Epic 只需預設租戶的 `callback_url`、`webhook_secret`，經唯讀 `store.TenantDirectory` 取值（Story 1.8 內建，Epic 2 擴充）。`LISTEN_ADDR` 預設 `:8080`，`/healthz` 存活探針。
- 日誌用 `log/slog` JSON，欄位白名單：`message_id`、`tenant_id`、`status`、`attempts`、`provider`、錯誤類別。
- 優雅關機：停 listener→`queue.Close`→停 `Dispatcher`，最多等 10 秒。
- 模組邊界依各 `internal/*/doc.go` 的「不擁有」；`cmd` 只 import `internal/*`。

## Cross-Story Dependencies

- Story 1.1 建立骨架、`store`、`queue`、`httpapi`、錯誤信封、`make test`、架構測試與最小 `mock` 樁，其餘 Story 皆依賴它。
- Story 1.2（完整 `mock` 故障注入）是 1.6、1.7 的前置；1.9 讓 `mitake` 與 `mock` 共用 1.2 建立的契約測試。
- Story 1.3 測試直接透過 `store` 寫入各種 Status，不依賴 1.6、1.7。
- Story 1.4 以測試用 `queue.Queue` 攔截 `Submit`，不依賴 1.6；1.5 建立在 1.4 之上。
- Story 1.6 先建立背景送出與退避；1.7 在其上加 `failed`／`failure_reason`。
- Story 1.8 依賴 1.6 的 `sent` 轉移，並自帶最小設定載入；Story 2.1 會擴充該設定而不改介面語意。
- Story 1.9 的 Transport 允許清單需要 1.8 的租戶 Webhook URL。
- Epic 2（驗證、租戶隔離、限流）依賴本 Epic；本 Epic 的端點在 Story 2.1 起全部納入 API key 驗證，回呼端點除外。
