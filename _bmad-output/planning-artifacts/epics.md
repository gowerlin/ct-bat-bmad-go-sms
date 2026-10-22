---
status: final
stepsCompleted: ['step-01-validate-prerequisites', 'step-02-design-epics', 'step-03-create-stories', 'step-04-final-validation']
inputDocuments:
  - _bmad-output/planning-artifacts/prds/prd-sms-platform-2026-10-22/prd.md
  - _bmad-output/planning-artifacts/prds/prd-sms-platform-2026-10-22/addendum.md
  - _bmad-output/planning-artifacts/architecture/architecture-sms-platform-2026-10-22/ARCHITECTURE-SPINE.md
---

# sms-platform - Epic Breakdown

## Overview

This document provides the complete epic and story breakdown for sms-platform, decomposing the requirements from the PRD and Architecture requirements into implementable stories. No UX design exists (v1 is API-only; PRD §2.2 excludes a Web console).

## Requirements Inventory

### Functional Requirements

FR-1：租戶可用 `POST /v1/messages` 送出訊息，回 202 與 `id`，Status 為 `queued`；欄位缺漏、號碼格式不符或內容超過 70 字元回 400 `invalid_request`；p95 < 200 ms（不含 Provider）。
FR-2：租戶可用 `GET /v1/messages/{id}` 查詢自己訊息的 Status；不存在回 404 `not_found`。
FR-3：租戶可用 `GET /v1/messages?status=…` 列出自己的訊息並篩選；由新到舊、limit／cursor 分頁（預設 50、上限 200）；不合法 `status` 回 400。
FR-4：以單一 Provider 介面隔離 Provider 差異；新增 Adapter 不需修改 API、佇列、Webhook；兩個 Adapter 通過同一組契約測試。
FR-5：維運者以 `PROVIDER=mock|mitake` 切換 Provider；`mock` 不發外部請求；`mitake` 指向可設定的（假）對端；非法值啟動失敗。
FR-6：`mitake` Adapter 將假對端回應對應為 Status：成功→`sent`、暫時失敗→重試、永久失敗→`failed`、回呼→`delivered`／`undelivered`。
FR-7：訊息入佇列由背景送出；Status 依序 `queued`→`sending`→`sent`→終態，重試用盡或永久失敗由 `sending` 直接進 `failed`；佇列為抽象介面。
FR-8：Provider 暫時失敗時對同一 Provider 最多重試 3 次（共呼叫 4 次）；第 2 次重試成功則不再呼叫；永久失敗不重試。
FR-9：不做跨 Provider failover；重試用盡後 `mock` 被呼叫 0 次，Status 為 `failed`。
FR-10：提供端點接收 Provider 回呼並更新 Status；只接受 `delivered`／`undelivered`；對應不到回 404；終態不可再改變。
FR-11：Status 變成 `sent`／`delivered`／`undelivered`／`failed` 時向租戶 Webhook URL POST，帶 HMAC 簽章（HMAC-SHA256、`X-Signature`）；`queued`／`sending` 不通知。
FR-12：同一 Status 只通知一次（以 `id`＋`status` 去重）；租戶端非 2xx 時最多重試 3 次，用盡後只記錄，不影響 Status。
FR-13：租戶可建立、讀取、更新、刪除自己的模板；跨租戶或不存在回 404；內容無法解析回 400 `invalid_template`。
FR-14：以送信時提供的值取代模板 `{{變數}}`；渲染發生在入佇列之前。
FR-15：帶 `template_id` 但缺變數回 422 `missing_template_variable` 並列出變數名、不建立訊息；多餘變數不報錯；模板不存在或不屬於該租戶回 404。
FR-16：每個 API key 獨立限流，上限取自所屬租戶設定（每分鐘請求數）；超過回 429 `rate_limited`；一個 key 被限流不影響其他 key；未設定用全域預設。
FR-17：每個 429 帶正整數秒的 `Retry-After`；等待後重送成功。
FR-18：`/v1/*` 需有效 `X-API-Key`；缺少、無效或已停用回 401 `unauthorized`（Provider 回呼端點除外）。
FR-19：驗證 Provider 回呼來源；未通過回 401 `unauthorized` 且 Status 不變。
FR-20：租戶隔離：跨租戶查訊息／模板一律 404 `not_found`；列表不含他租戶訊息；皆有自動化測試。
FR-21：維運者以設定檔建立與停用租戶、API key；key 以雜湊儲存；日誌無 key 明文；設定變更需重啟生效。
FR-22：4.1–4.7 每項功能至少有一個自動化測試；跨模組行為（FR-1、7、8、10、11、12）有 HTTP 入口到 Webhook 出口的 E2E。
FR-23：`go test ./... -race` 全部通過才算完成，並納入 CI 阻擋合併。
FR-24：E2E 只使用 `mock` 或假對端，無外網也全綠。
FR-25：任何 4xx／5xx 的 body 為 `{"error":{"code","message"}}`；測試遍歷所有錯誤路徑。
FR-26：錯誤碼清單至少涵蓋 `invalid_request`、`unauthorized`、`not_found`、`rate_limited`、`missing_template_variable`、`invalid_template`、`internal_error`；清單有文件，變更視為相容性變更。
FR-27：`GET /v1/messages/{id}` 對所有訊息含 `attempts`；`failed`／`undelivered` 另含非空 `failure_reason`（不含號碼與內容）；連續暫時失敗時 `attempts`=4 且原因指出重試用盡。
FR-28：重啟後未進終態的訊息不會被送出，查詢回 404 `not_found`；限制寫入租戶接入文件與 PRD §5。
FR-29：除設定指定的 Provider 位址與租戶 Webhook URL 外，不對任何其他目的地發出網路請求（E2E 攔截證明）。

### NonFunctional Requirements

NFR1：內部錯誤回 500 `internal_error`，body 不洩漏堆疊或內部資訊。
NFR2：日誌不記錄簡訊內容明文與 API key（可記 id 與 Status）。跨 Story 的約束：使用 `log/slog` JSON 輸出，欄位限於 AD-15 白名單（`message_id`、`tenant_id`、`status`、`attempts`、`provider`、錯誤類別），不含號碼、內容、API key、簽章密鑰、回呼密鑰。每則 Story 實作時遵守，不在個別 AC 重複。
NFR3：`POST /v1/messages` 回應 p95 < 200 ms（不含 Provider 往返）。跨 Story 的約束，以 `mock` 量測，不在個別 AC 重複。
NFR4：`go test ./... -race` 零失敗、零 data race。
NFR5：E2E 測試不依賴外部網路。
NFR6：發送紀錄（號碼、內容、Status）不外送（法遵，對應 FR-29）。

### Additional Requirements

- 無 starter template；`go.mod` 與 `internal/*/doc.go` 已存在。Epic 1 Story 1 是補齊骨架與組裝根，不是初始化專案。
- AD-17：只用標準庫，`go.mod` 不得出現 `require`。
- AD-19：架構測試（`e2e/`）檢查 import 依賴圖、`http.Client{`／`http.DefaultClient`／`time.Now(`／`time.Sleep(` 的位置、`go.mod` 無 `require`，納入 `make test`。
- AD-10：時間與等待只經注入的 `Now`、`Wait`；`e2e/testutil.FakeClock` 需補 `Wait`；ID 用 `crypto/rand`，前綴 `msg_`、`tpl_`。
- AD-9：`http.Client` 只在 `cmd/smsd` 建立，Transport 以允許清單（Provider 位址＋各租戶 Webhook URL）把關。
- AD-1：`provider.Provider` 僅 `Send`、`ParseCallback`、`Name`；Adapter 不重試；error wrap `ErrTransient`／`ErrPermanent`；契約測試 `provider/providertest`；組裝根以外不得 import `mock`／`mitake`。
- AD-2：`queue.Queue` 介面（`Submit`、`Run`、`Close`）；`Submit` 為預留→寫 store→放入的單一操作；4 個 worker；重試在同 worker 內退避。
- AD-3：webhook 分 `Inbound` 與 `Dispatcher`（單 goroutine、outbox `ClaimNext`、同一訊息依序送出）；通知 body 固定 `{"id","status"}`。
- AD-4：`store.TenantScoped`（首參數 tenantID）與 `store.System` 分離；tenant_id 只從 context 取。
- AD-5：單一狀態轉移表、compare-and-set、轉移與 `OutboundNotification` 在同一鎖內寫入；`sent` 逾 24 小時無回呼轉 `undelivered`。
- AD-6：`provider_ref`、`attempts` 與 `sending→sent` 同次 CAS；`CallbackSink` 讓 `mock` 回呼走真實 handler。
- AD-7：回呼驗證在 `ParseCallback`（共用密鑰、constant-time）；路徑 `POST /v1/providers/{name}/callbacks`，不經 API key、不計限流；終態或重複回呼回 200。
- AD-8：錯誤信封只在 `httpapi` 的單一 `writeError`；攔截 `ServeMux` 預設 404／405；驗證（E.164、70 code point）集中單一函式；滿載 503 `queue_full`。
- AD-11：單一 JSON 設定檔（`SMS_CONFIG`）；API key 存 SHA-256；啟動載入一次；未知欄位、非法 `PROVIDER`、缺必填即啟動失敗。
- AD-12：token bucket，每 API key 一桶；順序 authenticate→ratelimit→handler；全域預設 60；`Retry-After = max(1, ceil(秒))`。
- AD-13：重試退避 1s、2s、4s；Webhook 通知同規則；`failure_reason` 為「重試用盡」或「永久失敗」。
- AD-14：佇列容量 1000；終態訊息保留 24 小時，`PurgeExpired(now)` 移除；cursor 為不透明字串，排序 `(created_at, id)`。
- AD-15：`log/slog` JSON 輸出；欄位白名單；不記號碼、內容、密鑰。
- AD-16：先 `template.Render` 再 `queue.Submit`。
- AD-18：單程序單副本；環境變數 `SMS_CONFIG`、`PROVIDER`、`PROVIDER_URL`、`PROVIDER_CALLBACK_SECRET`、`LISTEN_ADDR`（預設 `:8080`）；`/healthz` 存活探針；`make test` 為完成門檻。
- 維護 `docs/openapi.yaml`、錯誤碼清單文件與租戶接入文件（含 FR-28 限制）。
- 優雅關機：停 listener→`queue.Close`→停 `Dispatcher`，最多等 10 秒。

### UX Design Requirements

無（v1 僅 API，無 UI）。

### FR Coverage Map

FR-1: Epic 1（Story 1.1）送出訊息
FR-2: Epic 1（Story 1.1，失敗原因欄位見 1.7）查詢單則
FR-3: Epic 1（Story 1.3）列表與 status 篩選
FR-4: Epic 1（Story 1.2、1.9）Provider 介面與共用契約測試
FR-5: Epic 1（Story 1.2 `mock`、Story 1.9 `mitake`）以 `PROVIDER` 切換
FR-6: Epic 1（Story 1.9）`mitake` 回應對應
FR-7: Epic 1（Story 1.1 入列、Story 1.6 背景送出）
FR-8: Epic 1（Story 1.6、1.7）重試 3 次
FR-9: Epic 1（Story 1.7）不 failover
FR-10: Epic 1（Story 1.8）接收回呼並更新狀態
FR-11: Epic 1（Story 1.8）HMAC 簽章通知
FR-12: Epic 1（Story 1.8）只通知一次與通知重試
FR-13: Epic 1（Story 1.4）模板 CRUD
FR-14: Epic 1（Story 1.4）變數渲染
FR-15: Epic 1（Story 1.5）缺變數 422
FR-16: Epic 2（Story 2.3）每 API key 限流
FR-17: Epic 2（Story 2.3）`Retry-After`
FR-18: Epic 2（Story 2.1）驗證 API key
FR-19: Epic 1（Story 1.8）回呼來源驗證
FR-20: Epic 2（Story 2.2）租戶隔離
FR-21: Epic 2（Story 2.1）設定檔建立與停用租戶、API key
FR-22: Epic 1、2（每個 Story 最後一條 AC 為對應 E2E）
FR-23: Epic 1（Story 1.1 建立 `make test`）
FR-24: Epic 1（Story 1.1 建立機制，Story 1.9 以假對端驗證）
FR-25: Epic 1（Story 1.1 建立錯誤信封，各 Story 沿用）
FR-26: Epic 1（Story 1.1 起逐 Story 累積錯誤碼；Epic 2 補 `unauthorized`、`rate_limited`）
FR-27: Epic 1（Story 1.1 `attempts` 欄位、Story 1.7 `failure_reason`）
FR-28: Epic 1（Story 1.1）
FR-29: Epic 1（Story 1.8 Webhook 出站、Story 1.9 Provider 出站）

29 條 FR 全部有歸屬。

## Epic List

### Epic 1：發送核心
訊息送不送得出去：租戶送出並查詢訊息、依 status 列表篩選、用模板送信；失敗時自動重試，結果可查證；Provider 回呼更新狀態，租戶只收到一次簽章通知；`mock` 與 `mitake` 兩個 Adapter 通過同一組契約測試。本 Epic 不含 API key 驗證與限流，以固定的預設租戶運作。
**FRs covered:** FR-1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 19, 23, 24, 25, 26, 27, 28, 29
**Stories：** 9（對應 spec Story 1、2、7、5、8、9、10、11、12）

### Epic 2：租戶與治理
誰可以送、可以送多快：缺 API key 回 401、跨租戶查詢回 404、超量回 429 與 `Retry-After`。認證先於限流。
**FRs covered:** FR-16, 17, 18, 20, 21
**Stories：** 3（對應 spec Story 3、4、6）
**依賴：** 只依賴 Epic 1。Epic 1 的 Story 以預設租戶運作，Epic 2 在不改變 `store` 介面語意的前提下換成真實租戶。

## Epic 1：發送核心

訊息送不送得出去。

### Story 1.1：送出訊息並可查詢

As a 租戶的後端工程師,
I want 用 `POST /v1/messages` 送出訊息並用 `GET /v1/messages/{id}` 查詢它,
So that 我知道閘道已收下訊息，且能取得它的狀態。

**Acceptance Criteria:**

**Given** 服務以 `mock` Provider 與 in-memory `store` 啟動，使用固定的預設租戶（本 Story 不需要 `X-API-Key`）；本 Story 只需要最小的 `mock` 樁（實作 `provider.Provider` 三個方法、`Send` 固定回成功），完整的故障注入在 Story 1.2
**When** 以合法的 E.164 號碼與 70 字元以內的內容呼叫 `POST /v1/messages`
**Then** 回 202，body 含非空且前綴為 `msg_` 的 `id`，訊息 Status 為 `queued`
**And** 訊息經 `queue.Submit` 建立（預留名額→寫 `store`→放入），佇列滿載時回 503 `queue_full` 且不留下訊息

**Given** 已有一則訊息
**When** 呼叫 `GET /v1/messages/{id}`
**Then** 回 200，body 含 `id`、`status` 與整數 `attempts`
**And** 查詢不存在的 `id` 回 404 `not_found`

**Given** 缺少必填欄位、號碼非 E.164 或內容超過 70 個 Unicode code point
**When** 呼叫 `POST /v1/messages`
**Then** 回 400 `invalid_request`，不建立訊息

**Given** 任何 4xx／5xx 路徑，包含 `ServeMux` 預設的未知路徑 404 與方法不符 405
**When** 回應送出
**Then** body 為 `{"error":{"code","message"}}`，405 帶 `Allow`，500 的 message 為固定字串且不洩漏內部資訊

**Given** 服務重啟（新的程序實例）
**When** 查詢重啟前建立的 `id`
**Then** 回 404 `not_found`（FR-28）
**And** 此限制寫入租戶接入文件

**Given** 一則已進入終態的訊息，且 `store` 的注入時間已前進超過 24 小時
**When** 呼叫 `PurgeExpired(now)` 後以 `GET /v1/messages/{id}` 查詢
**Then** 回 404 `not_found`；未滿 24 小時的終態訊息與非終態訊息不被移除
**And** 背景 goroutine 只是定時呼叫 `PurgeExpired`，時間經注入的 `Now`、`Wait`，測試不睡真實時間（AD-10、AD-14）

**Given** 專案骨架
**When** 執行 `make test`（`go test ./... -race`）
**Then** 全數通過，且包含架構測試（import 依賴圖、`http.Client{`／`time.Now(`／`time.Sleep(` 位置、`go.mod` 無 `require`）
**And** `store` 的狀態轉移表、`TenantScoped`／`System` 介面形狀遵守 AD-4、AD-5

**Given** 錯誤碼清單與 API 文件
**When** 本 Story 完成
**Then** 錯誤碼清單文件化（變更視為相容性變更），`docs/openapi.yaml` 涵蓋本 Story 的端點，`/healthz` 回存活
**And** E2E 測試：`TestE2E_SendMessage_Returns202AndQueryable`

### Story 1.2：mock Provider 故障注入

As a 租戶的後端工程師,
I want `mock` Adapter 能注入暫時失敗、永久失敗與成功,
So that 後續重試與失敗路徑都能在不連外網的情況下被測試。

**Acceptance Criteria:**

**Given** `provider.Provider` 介面只有 `Send`、`ParseCallback`、`Name`
**When** 以 `PROVIDER=mock` 啟動
**Then** 所有送信都經 `mock`，不發出任何外部網路請求
**And** `PROVIDER` 不在 `mock|mitake` 時程序啟動失敗並印出明確錯誤，不使用預設值

**Given** `mock` 被設定為注入暫時失敗
**When** 呼叫 `Send`
**Then** 回傳 wrap `ErrTransient` 的 error，且 `Send` 不重試、不 sleep

**Given** `mock` 被設定為注入永久失敗
**When** 呼叫 `Send`
**Then** 回傳 wrap `ErrPermanent` 的 error，原因碼不含號碼與內容

**Given** `mock` 被設定為成功
**When** 呼叫 `Send`
**Then** 回傳非空 `ProviderRef`
**And** `mock` 通過 `provider/providertest` 共用契約測試
**And** E2E 測試：`TestE2E_MockAdapter_FaultInjection`

### Story 1.3：依 status 列表篩選

As a 租戶的後端工程師,
I want 列出我的訊息並依 Status 篩選,
So that 我能一次找出所有失敗的訊息。

**Acceptance Criteria:**

**Given** 預設租戶下有 `queued` 與 `failed` 等不同 Status 的訊息（測試直接透過 `store` 寫入各種 Status，不依賴 Story 1.6、1.7）
**When** 呼叫 `GET /v1/messages?status=failed`
**Then** 回應只含 Status 為 `failed` 的訊息

**Given** 未帶 `status`
**When** 呼叫 `GET /v1/messages`
**Then** 回全部訊息，依 `(created_at, id)` 由新到舊排序

**Given** 訊息數量超過 `limit`
**When** 帶 `limit` 與 `cursor`
**Then** `limit` 預設 50、上限 200，cursor 為 `store` 產生的不透明字串，翻頁無重複與遺漏

**Given** 不合法的 `status` 值
**When** 呼叫列表
**Then** 回 400 `invalid_request`
**And** E2E 測試：`TestE2E_ListMessages_FilterByStatus`

### Story 1.4：模板渲染變數

As a 租戶的後端工程師,
I want 建立模板並在送信時帶 `template_id` 與變數,
So that 我不必在每個呼叫端自己組字串。

**Acceptance Criteria:**

**Given** 合法的模板內容
**When** 呼叫模板的建立、讀取、更新、刪除
**Then** 建立回 201 與前綴為 `tpl_` 的 `id`；對象不存在回 404 `not_found`
**And** 括號不成對等無法解析的內容回 400 `invalid_template`

**Given** 模板「訂單 {{order_id}} 已出貨」與變數 `order_id=A123`
**When** 以 `template_id` 呼叫 `POST /v1/messages`
**Then** 回 202，傳給 `queue.Submit` 的內容為「訂單 A123 已出貨」（以測試用的 `queue.Queue` 實作攔截；背景送出到 Provider 在 Story 1.6 才有）
**And** 渲染發生在 `queue.Submit` 之前

**Given** 帶有模板未使用的多餘變數
**When** 送信
**Then** 不報錯

**Given** 渲染結果超過 70 個 code point
**When** 送信
**Then** 與直接內容走同一個驗證函式，回 400 `invalid_request`
**And** E2E 測試：`TestE2E_Template_RendersVariables`

### Story 1.5：模板缺變數回 422

As a 租戶的後端工程師,
I want 缺少模板變數時立即得到明確錯誤,
So that 不會送出內容不完整的簡訊。

**Acceptance Criteria:**

**Given** 模板需要 `order_id` 與 `name`
**When** 送信時只帶 `order_id`
**Then** 回 422 `missing_template_variable`，message 列出缺少的 `name`
**And** 不建立訊息（列表中查不到）

**Given** 不存在的 `template_id`
**When** 送信
**Then** 回 404 `not_found`
**And** E2E 測試：`TestE2E_Template_MissingVariableReturns422`

### Story 1.6：重試後成功

As a 租戶的後端工程師,
I want 閘道在 Provider 暫時失敗時自動重試,
So that 我不必自己寫重試邏輯。

**Acceptance Criteria:**

**Given** `mock` 注入前兩次暫時失敗、第三次成功
**When** 背景 worker 處理一則 `queued` 訊息
**Then** Status 依序為 `queued`→`sending`→`sent`，不是 `failed`，成功後 Provider 不再被呼叫
**And** `attempts` 為對 Provider 的呼叫次數（本情境為 3）

**Given** 暫時失敗的重試
**When** 退避
**Then** 間隔為 1s、2s、4s，經注入的 `Wait` 等待，測試不睡真實時間
**And** 重試在同一 worker 內進行，不重新入列

**Given** `Send` 成功
**When** 寫入狀態
**Then** `provider_ref`、`attempts` 與 `sending→sent` 在單次 CAS 內完成

**Given** 永久失敗（例如號碼無效）
**When** worker 處理
**Then** 不重試，直接進入 `failed`
**And** E2E 測試：`TestE2E_Queue_RetriesThenSucceeds`

### Story 1.7：重試用盡標記 failed

As a 事故排查者,
I want 重試用盡的訊息被標記為 `failed` 並附上原因,
So that 我只查閘道一處就能找到失敗原因。

**Acceptance Criteria:**

**Given** `mock` 連續注入暫時失敗
**When** worker 處理訊息
**Then** Provider 共被呼叫 4 次（首次加 3 次重試），Status 變為 `failed`
**And** `GET /v1/messages/{id}` 的 `attempts` 為 4，`failure_reason` 為「重試用盡」

**Given** 永久失敗
**When** worker 處理
**Then** `failure_reason` 為「永久失敗」，可附加 Provider 原因碼
**And** `failure_reason` 不含收件號碼與簡訊內容

**Given** 狀態轉為 `failed`
**When** 寫入
**Then** 在同一次 CAS 內帶非空 `failure_reason`（AD-5）

**Given** 重試用盡
**When** 檢查其他 Adapter
**Then** 不跨 Provider failover，另一個 Provider 被呼叫 0 次
**And** E2E 測試：`TestE2E_Queue_ExhaustsRetriesMarksFailed`

### Story 1.8：回呼更新狀態並只通知租戶一次

As a 租戶的後端工程師,
I want Provider 回呼後收到簽章驗證通過的狀態通知，且同一狀態只收到一次,
So that 我不必輪詢，也不會重複處理。

**Acceptance Criteria:**

**Given** 本 Story 自帶最小的設定載入：`SMS_CONFIG` 指向的 JSON 設定檔只需描述預設租戶的 `callback_url` 與 `webhook_secret`，啟動時載入一次，經唯讀的 `store.TenantDirectory` 取值；未知欄位或缺必填欄位讓程序啟動失敗（Story 2.1 再擴充成完整的租戶設定，不改變此介面語意）
**When** 啟動並有一則訊息為 `sent`
**Then** 預設租戶的 Webhook 目的地與簽章密鑰取自該設定檔

**Given** 預設租戶在設定檔中有 `callback_url` 與 `webhook_secret`，一則訊息為 `sent`
**When** 以正確的回呼密鑰呼叫 `POST /v1/providers/{name}/callbacks` 回報 `delivered`
**Then** Status 更新為 `delivered`
**And** 租戶 Webhook 共收到 2 次通知（`sent`、`delivered`），body 為 `{"id","status"}`

**Given** Webhook 通知
**When** 用租戶密鑰對 body 重算 HMAC-SHA256
**Then** 與 `X-Signature` 一致，用錯誤密鑰則不一致

**Given** Provider 對同一訊息、同一 Status 重複回呼 N 次
**When** 處理
**Then** 回 200（冪等），租戶只收到 1 次該狀態通知
**And** 轉移與 outbox 入列為同一原子操作，`(message_id, status)` 唯一

**Given** 終態訊息（例如 `failed`）
**When** 收到 `delivered` 回呼
**Then** Status 不變

**Given** 回呼驗證失敗、狀態值不是 `delivered`／`undelivered`、或對應不到訊息
**When** 處理
**Then** 分別回 401 `unauthorized`（Status 不變）、400 `invalid_request`、404 `not_found`；回呼路徑不經 API key 驗證

**Given** 一則 `sent` 的訊息，注入時間前進超過 24 小時仍未收到回呼
**When** `store` 的逾時掃描執行
**Then** Status 轉為 `undelivered`，同一次 CAS 帶 `failure_reason`「回呼逾時」（AD-5）
**And** 租戶 Webhook 收到 `undelivered` 通知（`(message_id, status)` 唯一）；未滿 24 小時者不轉移；之後收到的回呼不再改變 Status

**Given** 租戶 Webhook 端點回非 2xx
**When** 通知
**Then** 重試最多 3 次（1s、2s、4s），用盡後只記日誌，不改訊息 Status，同一訊息的通知依序送出
**And** 出站請求只經受限 `http.Client`，目的地不在允許清單內即失敗
**And** E2E 測試：`TestE2E_Webhook_DeliveredUpdatesStatusAndNotifiesTenantOnce`

### Story 1.9：mitake 風格 Adapter 契約測試

As a 維運者,
I want 以 `PROVIDER=mitake` 讓閘道連到假對端,
So that 在沒有真實帳號時也能驗證 Provider 差異被隔離。

**Acceptance Criteria:**

**Given** `PROVIDER=mitake` 且 `PROVIDER_URL` 指向假對端
**When** 送信
**Then** 送信經 `mitake` Adapter，`mock` 不被呼叫

**Given** 假對端的四種回應
**When** `mitake` Adapter 處理
**Then** 成功→`sent`；暫時失敗→依重試規則；永久失敗→`failed`；回呼→`delivered` 或 `undelivered`

**Given** `mock` 與 `mitake`
**When** 執行 `provider/providertest`
**Then** 兩者通過同一組契約測試；新增 Adapter 不需修改 API、佇列、Webhook 的程式碼

**Given** 無外部網路的環境
**When** 執行 E2E
**Then** 全綠；`http.Client` 只在 `cmd/smsd` 建立並注入，Transport 允許清單由 Provider 位址與租戶 Webhook URL 組成，其他目的地一律失敗
**And** 假對端回呼在 `Send` 回應之後才發送
**And** E2E 測試：`TestE2E_MitakeAdapter_ContractAgainstFakeServer`

## Epic 2：租戶與治理

誰可以送、可以送多快。

### Story 2.1：缺 API key 回 401

As a 維運者,
I want 只有持有效 API key 的租戶能呼叫 `/v1/*`,
So that 未授權的呼叫被擋下，租戶與 API key 由設定檔管理。

**Acceptance Criteria:**

**Given** 在 Story 1.8 的最小設定載入之上擴充：`SMS_CONFIG` 指向的 JSON 設定檔描述租戶、API key（只存 SHA-256 hex）、`callback_url`、`webhook_secret`、`rate_limit_per_min`、`disabled`
**When** 啟動
**Then** 設定只載入一次，轉成不可變的 `store.TenantConfig`；未知欄位或缺必填欄位讓程序啟動失敗並印出明確錯誤

**Given** 請求沒有 `X-API-Key`
**When** 呼叫任何 `/v1/*` 端點（Story 1.1 至 1.5 的端點都納入驗證）
**Then** 回 401 `unauthorized`，使用統一錯誤信封

**Given** 無效或標記為停用的 API key
**When** 呼叫
**Then** 同樣回 401 `unauthorized`

**Given** 有效 API key
**When** 呼叫
**Then** middleware 將其解成 `tenant_id` 放進 request context，之後只從 context 取，取代 Epic 1 的預設租戶，`store` 介面語意不變

**Given** Provider 回呼端點
**When** 不帶 `X-API-Key`
**Then** 不受本 Story 驗證影響（驗證由 Adapter 負責）

**Given** 日誌
**When** 任何請求被處理
**Then** 不出現 API key 明文
**And** E2E 測試：`TestE2E_Auth_MissingKey401`

### Story 2.2：跨租戶查詢回 404

As a 租戶的負責人,
I want 其他租戶完全看不到我的訊息與模板,
So that 我的資料不外洩，連存在性也不洩漏。

**Acceptance Criteria:**

**Given** 租戶 A 與租戶 B 各自有訊息
**When** 租戶 A 用 `GET /v1/messages/{id}` 查詢租戶 B 的訊息
**Then** 回 404 `not_found`，不是 403

**Given** 租戶 A 的訊息列表
**When** 列出（含 `status` 篩選）
**Then** 不含租戶 B 的任何訊息

**Given** 租戶 B 的模板
**When** 租戶 A 對它做讀取、更新、刪除，或在送信時帶它的 `template_id`
**Then** 一律回 404 `not_found`

**Given** `httpapi` 持有的 repository
**When** 檢查介面
**Then** 每個方法第一個參數為 `tenantID`，空字串回錯誤；以 `ProviderRef` 查訊息等系統級操作只在 `store.System`，`httpapi` 無法取得
**And** E2E 測試：`TestE2E_Tenant_IsolationReturns404`

### Story 2.3：超量回 429 與 Retry-After

As a 維運者,
I want 每個 API key 依所屬租戶的上限限流,
So that 單一租戶不會壓垮閘道或耗盡 Provider 額度。

**Acceptance Criteria:**

**Given** 租戶的 `rate_limit_per_min` 為 N（未設定用全域預設 60）
**When** 同一 API key 在一分鐘內請求超過 N 次
**Then** 後續請求回 429 `rate_limited`，使用統一錯誤信封
**And** 每個 429 帶正整數秒的 `Retry-After`，值為 `max(1, ceil(等到下一枚 token 的秒數))`

**Given** 一個 API key 被限流
**When** 另一個 API key 請求
**Then** 不受影響；租戶 A 調高上限不影響租戶 B

**Given** 等待 `Retry-After` 所指秒數（以 `FakeClock` 前進）
**When** 重送
**Then** 請求成功，測試不睡真實時間

**Given** 無效或缺少的 API key
**When** 請求
**Then** 回 401 且不建立或消耗任何桶（middleware 順序固定為 authenticate→ratelimit→handler）
**And** Provider 回呼端點不計入限流
**And** E2E 測試：`TestE2E_RateLimit_Returns429WithRetryAfter`
