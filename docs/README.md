# sms-platform docs

本目錄是 API 草案與資料模型，也是 BMad 的 `project_knowledge`（`_bmad/bmm/config.yaml` 指向 `{project-root}/docs`，D-2）。

| 檔案 | 內容 | 狀態 |
|---|---|---|
| `openapi.yaml` | § 5.4 路徑清單（OpenAPI 3.1 空殼，schema 留 TODO） | P1-c 空殼；每張實作工單補 schema；CH5 完稿 |
| `error-codes.md` | 錯誤信封與錯誤碼清單（變更＝相容性變更） | Story 1.1 |
| `onboarding.md` | 租戶接入文件（含重啟限制 FR-28） | Story 1.1 起 |
| 本檔 | 資料模型草案 ↔ Go 套件對照 | P1-c |

## 執行設定：`PROVIDER`

`smsd` 由環境變數 `PROVIDER` 選擇簡訊供應商 adapter（鍵名依 SPINE AD-18；範例見 repo 根目錄 `.env.example`）。**沒有預設值**：未設定、空字串或 `mock`／`mitake` 以外的值，`smsd` 在啟動時以 `slog` 記錄 `error_class` 與合法值後 exit `1`，不回顯任何環境變數的值。

| `PROVIDER` | 另需設定 | 說明 |
|---|---|---|
| `mock` | 無 | 課程主線；不連網路，可注入故障（T0002） |
| `mitake` | `PROVIDER_URL`、`MITAKE_USERNAME`、`MITAKE_PASSWORD`、`PROVIDER_CALLBACK_SECRET`（缺一即啟動失敗） | 三竹**風格**的模擬 HTTP adapter（D014、D016、D017）：不是真實規格，帳號密碼與密鑰都是假的，只連假對端，不連真實端點 |

- 本機執行要帶 `PROVIDER`：`PROVIDER=mock make run`（`Makefile` 不設預設值）。dev container 的 `compose.yaml` 已設 `PROVIDER: mock`。
- `mitake` 的密碼與回呼密鑰不得出現在日誌與錯誤訊息；供應商位址只經 `cmd/smsd` 建立的受限 `http.Client`（允許清單、不跟隨轉址，AD-9）連線。
- `mitake` 的回呼：`POST`、`application/x-www-form-urlencoded`，欄位 `msgid`、`statuscode`（`4` → `delivered`；`5`／`6`／`8` → `undelivered`；其他值拒收）、`donetime`（`yyyyMMddHHmmss`，UTC+8），密鑰放 header `X-Mitake-Secret`（D017）。

## 資料模型草案（對照 COURSE-PLAN § 5.3）

欄位只到足以規劃；定稿在 CH2 的 PRD／SPINE（`_bmad-output/planning-artifacts/`）。

| 實體 | 欄位（草案） | 備註 | 落在哪個套件 |
|---|---|---|---|
| Tenant | `id`、`name`、`api_key_hash`、`callback_url`、`callback_secret`、`rate_limit`（per-minute） | API key 以 header `X-API-Key` 傳；只存 hash | `internal/tenant`（規則）、`internal/store`（持久化） |
| Message | `id`、`tenant_id`、`to`、`body`、`template_id?`、`vars?`、`status`（queued／sending／sent／delivered／undelivered／failed）、`attempts`、`provider`、`provider_ref`、`created_at`、`updated_at` | in-memory store，SPINE 留 DB 邊界 | `internal/store`（實體）、`internal/queue`（狀態轉移） |
| Template | `id`、`tenant_id`、`name`、`body`（`{{var}}`）、`created_at` | 缺變數 → 422 | `internal/template` |
| DeliveryReport | `message_id`、`provider`、`status`、`raw`、`received_at` | 供應商回呼。供應商只給 `provider_ref`；`provider_ref → message_id` 的對應由 webhook 做 | `internal/provider`（`DeliveryReport` 型別，含 `ProviderRef`）、`internal/webhook`（對應與套用） |
| OutboundNotification | `id`、`message_id`、`tenant_id`、`signature`、`delivered_once` | HMAC 簽章；冪等（最多通知一次） | `internal/webhook` |

## 狀態機（草案）

```
queued ──(queue 取出)──> sending ──(Provider.Send ok)──> sent ──(回呼 delivered)──> delivered
   ▲                        │                                  └──(回呼 undelivered)──> undelivered
   └──(重試，attempts+1)────┘──(重試耗盡)──> failed
```

## 套件邊界一覽

| 套件 | 擁有 | 不擁有 |
|---|---|---|
| `internal/httpapi` | 路由、JSON 形狀、錯誤信封、狀態碼、`X-API-Key` 取值 | 業務規則 |
| `internal/queue` | 佇列、重試、back-off、queued→sending→sent／failed | Provider 本身、持久化 |
| `internal/provider` | `Provider` 介面與跨界型別；`mock/`、`mitake/`（CH5）實作 | 重試、狀態持久化、租戶通知 |
| `internal/template` | 模板 CRUD、`{{var}}` 渲染、缺變數錯誤 | 持久化、HTTP |
| `internal/ratelimit` | 每 API key 限流演算法、`Retry-After` | 限額存放（tenant）、回 429（httpapi） |
| `internal/tenant` | API key hash／查找、跨租戶 404 規則 | header 本身、持久化 |
| `internal/webhook` | 入站回呼套用、出站簽章通知、冪等 | 解析供應商 payload（provider） |
| `internal/store` | 實體與 repository 介面、in-memory 實作、status 篩選 | 驗證、HTTP、發送 |

每個套件的 `doc.go` 是邊界的權威敘述；本表只是索引。
