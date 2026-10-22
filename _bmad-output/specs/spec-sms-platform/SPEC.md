---
id: SPEC-sms-platform
companions:
  - glossary.md
  - state-machines.md
  - error-codes.md
  - ../../planning-artifacts/prds/prd-sms-platform-2026-10-22/prd.md
  - ../../planning-artifacts/architecture/architecture-sms-platform-2026-10-22/ARCHITECTURE-SPINE.md
sources:
  - ../../planning-artifacts/prds/prd-sms-platform-2026-10-22/addendum.md
---

> **Canonical contract.** This SPEC and the files in `companions:` are the complete, preservation-validated contract for what to build, test, and validate. Source documents listed in frontmatter are for traceability — consult them only if you need narrative rationale or prose color this contract intentionally omits.

# sms-platform v1：內部 SMS 閘道

## Why

痛點：5 個團隊、3 種語言各自串接國內 SMS Provider，重試與狀態回呼行為不一致，出事要逐團隊追查。法遵要求發送紀錄不得外送（排除國外 CPaaS）。v1 把重試與狀態回呼收斂到單一閘道，讓所有租戶行為一致、問題可在一處查證。v1 刻意不承諾高可用，也不得承載驗證碼類關鍵簡訊；這是範圍取捨，非永久排除。

## Capabilities

每個 CAP 的逐條驗收見 PRD 對應 FR（adopted companion）；FR 編號為穩定 ID。架構規則見 spine 的 AD。

- **CAP-1**（FR-1, 7；AD-2, 6）
  - **intent:** 租戶可非同步送出訊息並取得 `id`。
  - **success:** 合法請求（號碼＋內容或 `template_id`）回 202、非空 `id`、Status `queued`；缺欄位、號碼非 E.164、內容超過 70 字元回 400 `invalid_request`；訊息依 `state-machines.md` 由背景工作送出。
- **CAP-2**（FR-2, 3, 27）
  - **intent:** 租戶可查單則狀態、列出並依 Status 篩選自己的訊息，並看到失敗原因與嘗試次數。
  - **success:** GET 單則回 `id`、`status`、`attempts`；`failed`／`undelivered` 另含非空 `failure_reason`（不含號碼與內容）；假對端連續暫時失敗時 `attempts`=4 且原因為重試用盡；`status=failed` 篩選只回該租戶的 `failed`；非法 `status` 回 400；不存在 `id` 回 404。
- **CAP-3**（FR-4, 5, 6；AD-1, 7）
  - **intent:** 維運者可用 `PROVIDER=mock|mitake` 切換 Provider，其餘模組不依賴特定 Provider。
  - **success:** 兩個 Adapter 通過同一組介面契約測試；新增 Adapter 不改 API、佇列、Webhook 程式碼；`mock` 不發外部請求；`mitake` 只連可設定的假對端並對應四種回應（成功→`sent`、暫時失敗→重試、永久失敗→`failed`、回呼→`delivered`／`undelivered`）；`PROVIDER` 值非法時啟動失敗且不使用預設。
- **CAP-4**（FR-8, 9；AD-13）
  - **intent:** Provider 暫時失敗時，閘道對同一 Provider 重試，耗盡後標記失敗，不跨 Provider。
  - **success:** 連續暫時失敗時 Provider 被呼叫 4 次後 `failed`，`mock` 被呼叫 0 次；第 2 次重試成功則不再呼叫且非 `failed`；永久失敗不重試直接 `failed`。
- **CAP-5**（FR-10, 19；AD-3, 7）
  - **intent:** 閘道接收並驗證 Provider 回呼以更新 Status，終態不可被覆寫。
  - **success:** 驗證失敗回 401 且 Status 不變；只接受 `delivered`／`undelivered`，其他值回 400；對應不到訊息回 404；對終態或重複回呼不改 Status（`failed` 後收 `delivered` 仍為 `failed`），回 200 冪等。
- **CAP-6**（FR-11, 12；AD-3, 5, 13）
  - **intent:** 租戶的 Webhook 在 Status 變為 `sent`、`delivered`、`undelivered`、`failed` 時收到 HMAC 簽章通知，且同一 Status 只通知一次。
  - **success:** 正常訊息收 2 次（`sent`、`delivered`），重試用盡收 1 次（`failed`）；body 為 `{"id","status"}`；以租戶密鑰重算 HMAC 與 `X-Signature` 一致、錯誤密鑰不一致；Provider 重複回呼 N 次仍只通知 1 次；租戶端非 2xx 時重試 3 次且不產生重複有效通知；`queued`、`sending` 不通知。
- **CAP-7**（FR-13, 14, 15；AD-16）
  - **intent:** 租戶可管理自己的模板，並以 `{{變數}}` 渲染後送信。
  - **success:** 建立回 201；對象不存在或屬他租戶回 404；括號不成對回 400 `invalid_template`；「訂單 {{order_id}} 已出貨」＋`order_id=A123`→「訂單 A123 已出貨」且在入佇列前渲染；缺變數回 422 `missing_template_variable`（message 列變數名）且不建立訊息；多餘變數不報錯。
- **CAP-8**（FR-16, 17；AD-12）
  - **intent:** 每個 API key 獨立限流，上限依所屬租戶設定。
  - **success:** 超過一分鐘上限回 429 `rate_limited` 並帶正整數秒 `Retry-After`；等待該秒數後重送成功；一 key 被限流不影響另一 key；不同租戶可有不同上限；未設定用全域預設。
- **CAP-9**（FR-18, 20, 21；AD-4, 11）
  - **intent:** 租戶以 API key 驗證並彼此完全隔離，維運者以設定檔建立與停用租戶與 key。
  - **success:** 缺、無效或停用 key 回 401 `unauthorized`（`/v1/*` 除 Provider 回呼端點）；租戶 A 存取 B 的訊息、模板一律 404（非 403），列表不含 B 的資料，且每條有自動化測試；設定檔新增的 key 載入後可通過驗證；key 以 SHA-256 雜湊儲存，日誌無明文。
- **CAP-10**（FR-25, 26；AD-8）
  - **intent:** 所有錯誤回應使用同一信封與穩定錯誤碼。
  - **success:** 任何 4xx／5xx body 為 `{"error":{"code","message"}}`，`code` 為小寫底線字串；測試遍歷所有已定義錯誤路徑；500 為固定 `internal_error` 訊息且不洩漏內部資訊；未知路徑與不符方法也走信封。
- **CAP-11**（FR-28；AD-14）
  - **intent:** 重啟與滿載時的訊息命運對租戶可預期。
  - **success:** 重啟後重啟前非終態訊息不送出，GET 回 404 `not_found`（測試證明）；限制載明於租戶接入文件；佇列滿載時 `POST /v1/messages` 回 503 `queue_full` 且不建立訊息；終態訊息保留 24 小時後 GET 回 404。
- **CAP-12**（FR-29；AD-9）
  - **intent:** 發送紀錄（號碼、內容、Status）不離開設定允許的目的地。
  - **success:** E2E 攔截所有對外連線，除 Provider 設定位址與租戶 Webhook URL 外無任何請求；不在允許清單的目的地被拒。
- **CAP-13**（FR-22, 23, 24；AD-10, 19）
  - **intent:** 每個功能都有自動化 E2E，作為「完成」的定義。
  - **success:** 4.1–4.7 每項功能至少一個自動化測試；跨模組行為（FR-1, 7, 8, 10, 11, 12）有從 HTTP 入口經佇列、`mock` Adapter 到 Webhook 出口的 E2E；`go test ./... -race` 零失敗零 data race；測試在無外網環境全綠。

## Constraints

- 單一 Go 程序、單副本、in-process 佇列；重啟丟失佇列與記憶體訊息（已接受）。
- 不承諾 99.9% 可用；租戶不得送驗證碼類簡訊（純政策，無偵測或攔截）。
- 佇列、Provider、store 以介面隔離；`provider/mock`、`provider/mitake` 只有 `cmd` 可 import；新增 Adapter 不得改其他模組（AD-1, 2）。
- Adapter 的 `Send` 不重試、不 sleep，error 必須分類為 transient 或 permanent，未分類視為 permanent；重試與退避（1s、2s、4s）歸 `queue`（AD-1, 13）。
- Status 只能經 `store` 單一轉移表以 CAS 變更；轉移與 outbox 入列為同一原子操作；`(message_id, status)` 唯一（AD-5）。
- 每個 `store` 查詢帶 `tenant_id`，來源只能是 request context；系統級查詢介面 `httpapi` 不得持有（AD-4）。
- 標準庫優先，`go.mod` 不得有 `require`；時間與等待全經注入；`http.Client` 只在 `cmd/smsd` 建立並用允許清單 Transport；以架構測試把關（AD-9, 10, 17, 19）。
- 日誌只記識別欄位，不記號碼、內容、API key、各類密鑰（AD-15）。
- 設定為單一 JSON 檔，啟動載入一次不熱載入，非法值啟動失敗；環境變數固定（AD-11, 18）。租戶的 Webhook URL 欄位名為 `callback_url`（使用者確認，解 AD-11 原文矛盾）。
- 完成門檻為 `make test`（`go test ./... -race`）。

## Non-goals

- 跨 Provider failover。
- 真實 Provider 帳號整合（`mitake` 只連假對端）。
- 國外 CPaaS（法遵排除）。
- Web 主控台（範圍取捨，非永久排除）。
- 租戶與 API key 管理 API（僅設定檔）。
- 持久化佇列、多副本、監控、高可用（v1 之後）。
- 偵測或攔截驗證碼類簡訊。

## Success signal

小林的服務改接閘道後，不寫任何重試或輪詢程式碼，即先後收到簽章驗證通過的 `sent`、`delivered` 通知，且與 `GET /v1/messages/{id}` 一致；任一 `failed`／`undelivered` 訊息只查閘道這一處就能看到原因與嘗試次數（僅限程序存活期間）。v1 完成時 5 個租戶都完成接入檢核表並能以 `mock` Adapter 通過自己的整合驗證，且 `go test ./... -race` 在主分支持續全綠。

## Assumptions

- 號碼 E.164、內容上限 70 code point、`POST` p95 < 200 ms（不含 Provider）。
- 列表依建立時間新到舊，`limit` 預設 50、上限 200，cursor 為不透明字串。
- Webhook 為 HMAC-SHA256、header `X-Signature`，每租戶獨立密鑰與 URL。
- 退避 1s／2s／4s 無 jitter；Webhook 通知同樣重試 3 次。
- 全域預設限流 60／分鐘；佇列容量 1000；worker 固定 4；終態保留 24 小時。
- `sent` 超過 24 小時無回呼，由 `store` 轉為 `undelivered`（`failure_reason=回呼逾時`）；此轉移為架構補充，PRD 未列。
- 滿載回 503 `queue_full`（PRD Q8 暫解）；405 以 `invalid_request` 回並帶 `Allow`。
- 真實 Provider 的錯誤碼對照、回呼驗證與位址法遵檢核留待接入時補。
- 設定變更需重啟才生效；優雅關機最多等 10 秒後丟棄未排空訊息。
- 24 小時 `undelivered` 轉移與 503 `queue_full` 兩項已由使用者確認採用。
- Deferred：`cmd/smsd` 與 `e2e/testutil` 各自組裝依賴可能漂移，v1 不抽出共用組裝套件，待日後重評。
- 逾時與連線錯誤歸 transient，「結果不明」造成重複簡訊的風險 v1 接受。

## Open Questions

- 自建對比現成方案（開源閘道、具 failover 的國內聚合商）未評估且未結案（PRD Q2，負責人：閘道團隊 tech lead，期限：架構開工前）。結論若為現成方案，本 spec 與 spine 作廢重審。
- 「約 30 萬則／月」與自建 TCO 未查證（PRD Q3），限流預設值與佇列容量待校準；五租戶現行行為差異清單缺（PRD Q6），SM-1 無基準。
- 向五個租戶說明 v1 未解決原始事故（驗證碼斷線 2 小時）的負責人未指定，須在第一個租戶換接前完成（PRD Q1）。
- 遷移順序與「兩季」起算點、v1 之後階段的啟動條件、驗證碼限制的機制均未定義（PRD Q4, Q5, Q7）。
