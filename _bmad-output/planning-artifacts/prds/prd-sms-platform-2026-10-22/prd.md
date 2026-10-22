---
title: "PRD: 內部 SMS 閘道（sms-platform）"
status: final
created: 2026-10-22
updated: 2026-10-22
---

# PRD: 內部 SMS 閘道（sms-platform）

## 0. 文件目的

本 PRD 供五個租戶的負責人、架構與後續拆 Epic／Story 的人使用。輸入為 `briefs/brief-sms-platform-2026-10-22/` 的 `brief.md` 與 `addendum.md`。功能依 §4 分為八項（4.1–4.8），另有橫跨所有功能的統一錯誤格式（4.9）。FR 全域編號，編號是穩定 ID，不依閱讀順序（FR-27 至 FR-29 為審查後新增，分別放在 4.1、4.3、4.9）。推論而來的內容以 `[ASSUMPTION]` 標註，並彙整在 §9。技術實作取捨放在同資料夾的 `addendum.md`，不放在本文。

風險等級：內部工具。

## 1. 願景

sms-platform 是公司內部的 SMS 閘道。各團隊透過統一的 REST API 送信，並透過簽章 Webhook 接收狀態。重試與狀態回呼由閘道集中執行，上游仍使用國內第三方簡訊供應商，我們不自己當供應商。

目前 5 個團隊、3 種語言各自串接供應商，重試與回呼行為不一致，出事時要逐團隊追查。v1 把這兩件事收斂到一處，讓所有團隊行為一致，問題可在單一地方查證。

v1 刻意**不承諾高可用**，也**不得承載驗證碼類關鍵簡訊**。這個限制是範圍取捨，不是永久排除（見 §5、§6）。

## 2. 目標使用者

### 2.1 Jobs To Be Done

- 作為租戶的後端工程師，我換接閘道後，不必再自己寫重試與回呼邏輯。
- 作為同一位工程師，我要在不同團隊之間得到可預期、語意一致的狀態。
- 作為負責事故排查的人，出事時我只需要查閘道一處就能找到原因。
- 作為法遵單位，我要確認發送紀錄不外送（對應 FR-29）。
- 作為維運者，我要用設定檔管理租戶、API key、Webhook URL 與限流上限，並切換 Provider。

### 2.2 非使用者（v1）

- 需要驗證碼類關鍵簡訊的團隊（v1 不得承載，目前僅為政策約束，見 §8 Q7）。
- 需要 Web 主控台的操作人員（v1 只有 API）。

### 2.3 關鍵使用者旅程

- **UJ-1. 小林的服務送出一則通知並確認它被送達。**
  - **Persona：** 小林，訂單服務團隊的 Go 後端工程師，剛把原本直連 Provider 的程式碼改接閘道。
  - **進入狀態：** 已持有租戶的 API key；維運者已在設定檔中為小林的租戶設定好 Webhook URL 與簽章密鑰。
  - **路徑：** (1) 用 `POST /v1/messages` 帶 `template_id` 與變數送信，收到 202 與 `id`。(2) 閘道把訊息入佇列並送往 Provider，必要時重試。(3) Provider 回呼後，閘道更新狀態，並用 HMAC 簽章通知小林的 Webhook。(4) 小林用 `GET /v1/messages/{id}` 核對狀態。
  - **高潮：** 小林的服務先後收到簽章驗證通過的 `sent` 與 `delivered` 通知（一則正常訊息共 2 次通知），狀態與 GET 結果一致，他不用寫任何重試或輪詢程式碼。
  - **結局：** 訊息停在終態，同一個狀態不會再被通知。
  - **邊界情境：** Provider 連續失敗，閘道重試 3 次後標記 `failed` 並通知小林；他用 `GET /v1/messages/{id}` 即可看到失敗原因與重試次數，由他的服務決定後續處置。

## 3. 詞彙表

後續章節必須逐字使用下列詞彙，不得引入同義詞。

- **租戶（Tenant）** — 一個接入團隊。租戶之間的資料互相不可見。
- **API key** — 租戶呼叫 API 的憑證，以 `X-API-Key` header 帶入。一個 API key 屬於一個租戶。
- **訊息（Message）** — 一次發送請求。有全域唯一的 `id`，屬於一個租戶。
- **狀態（Status）** — 訊息目前的狀態。集合為 `queued`（已入佇列）、`sending`（正送往 Provider）、`sent`（Provider 已接受）、`delivered`（Provider 回報送達）、`undelivered`（Provider 回報送達失敗）、`failed`（閘道端失敗：重試用盡或永久失敗）。`delivered`、`undelivered`、`failed` 為終態。
- **Provider** — 實際送出簡訊的國內第三方供應商，透過 Provider 介面接入；PRD 內一律稱 Provider，不另用「供應商」「呼叫端」等同義詞（§1 的背景描述除外）。v1 有 `mock` 與 `mitake` 兩種實作。
- **Adapter** — 實作 Provider 介面的程式模組。
- **佇列（Queue）** — 暫存待送訊息的元件，介面抽象化。v1 為 in-process 實作。
- **模板（Template）** — 含 `{{變數}}` 佔位的訊息內容，屬於一個租戶。
- **Webhook** — 閘道主動通知租戶 Status 變化的 HTTP 請求（閘道 → 租戶）。會通知的 Status 見 FR-11。
- **維運者（Operator）** — 閘道團隊中負責設定與部署的人。透過設定檔管理租戶、API key、Webhook URL、簽章密鑰、限流上限與 `PROVIDER`。租戶不能自助設定這些項目。
- **Provider 回呼（Provider Callback）** — Provider 向閘道回報送達結果的 HTTP 請求（Provider → 閘道）。與 Webhook（閘道 → 租戶）是兩件事。
- **假對端（Fake Peer）** — 模擬 Provider 行為的測試替身伺服器，`mitake` Adapter 只連它。與 `mock` Adapter 不同：`mock` 不經網路，假對端經網路。
- **簽章（Signature）** — 以 HMAC 計算的 Webhook 驗證值，讓租戶確認通知來自閘道。

## 4. 功能

通用規則：**所有錯誤回應**一律為 `{"error":{"code":"…","message":"…"}}`（見 FR-25、FR-26）。

### 4.1 REST 發送 API

**說明：** 租戶透過 API 送出訊息、查詢單則狀態，並列出自己的訊息。送出是非同步的：API 只負責接收並入佇列，回 202。實現 UJ-1。

#### FR-1：送出訊息

租戶可用 `POST /v1/messages` 送出一則訊息，並取得 `id`。收件號碼格式 `[ASSUMPTION: E.164]`，內容長度上限 `[ASSUMPTION: 70 個字元；超長簡訊 v1 不支援，回 400]`。

**驗收（可量測）：**
- 請求包含合法的收件號碼與內容（或 `template_id`）時，回應狀態碼為 202，body 含非空的 `id`，且訊息 Status 為 `queued`。
- 回應時間 `[ASSUMPTION: p95 < 200 ms，不含 Provider 往返]`，因為 API 只入佇列，不等待 Provider。
- 缺少必填欄位、號碼格式不符或內容超過上限時回 400，error code 為 `invalid_request`。

#### FR-2：查詢單則訊息狀態

租戶可用 `GET /v1/messages/{id}` 查詢自己訊息的 Status。

**驗收：**
- 查詢存在且屬於該租戶的 `id` 時回 200，body 含 `id` 與 `status`。
- 查詢不存在的 `id` 時回 404，error code 為 `not_found`。

#### FR-3：列出訊息並依狀態篩選

租戶可用 `GET /v1/messages?status=…` 列出自己的訊息，並以 Status 篩選。

**驗收：**
- 帶 `status=failed` 時，回應只含該租戶 Status 為 `failed` 的訊息。
- 未帶 `status` 時回該租戶全部訊息，不含其他租戶的訊息。
- `[ASSUMPTION: 列表依建立時間由新到舊排序；以 limit 與 cursor 參數分頁，limit 預設 50、上限 200。]`
- 帶不合法的 `status` 值時回 400，error code 為 `invalid_request`。

#### FR-27：失敗原因與重試次數

租戶查詢單則訊息時，可看到該訊息失敗的原因與嘗試次數，不必逐團隊追查。實現 UJ-1。

**驗收：**
- `GET /v1/messages/{id}` 的 body 對所有訊息含 `attempts`（對 Provider 的呼叫次數，整數）。
- Status 為 `failed` 或 `undelivered` 時，body 另含 `failure_reason`（非空字串，內容為 Provider 回報的原因碼，或閘道的分類，例如「重試用盡」「永久失敗」）。
- 假對端連續暫時失敗的測試中，`attempts` 為 4，且 `failure_reason` 指出重試用盡。
- `failure_reason` 不含收件號碼與簡訊內容。

### 4.2 Provider Adapter

**說明：** 閘道透過 Provider 介面與 Provider 溝通。v1 提供 `mock`（測試與本機用）與 `mitake`（國內 Provider 風格）兩個 Adapter。`mitake` Adapter 只連假對端，不綁真實帳號。

#### FR-4：定義 Provider 介面

系統以單一 Provider 介面隔離 Provider 差異，其餘模組不得依賴特定 Provider 的細節。

**驗收：**
- 新增一個 Adapter 只需實作 Provider 介面並登錄，不需修改 API、佇列、Webhook 模組的程式碼。
- 兩個 Adapter 都通過同一組介面契約測試。

#### FR-5：以設定切換 Provider

維運者可用設定 `PROVIDER=mock|mitake` 切換使用的 Provider。

**驗收：**
- 設定為 `mock` 時，所有送信都經由 `mock` Adapter，不發出任何外部網路請求。
- 設定為 `mitake` 時，送信經由 `mitake` Adapter，並指向可設定的（假）對端位址。
- 設定值不在 `mock|mitake` 範圍內時，程序啟動失敗並印出明確錯誤，不使用預設值。
- 切換 Provider 不需要修改任何租戶的程式碼。

#### FR-6：`mitake` Adapter 對應 Provider 回應

`mitake` Adapter 將 Provider 回應轉換為閘道的 Status，並能接收 Provider 回呼。

**驗收：**
- 對假對端的四種回應，各自對應如下（以測試證明）：(a) 送出時「成功」→ `sent`；(b) 送出時「暫時失敗」→ 依 FR-8 重試；(c) 送出時「永久失敗」→ `failed`；(d) 之後的 Provider 回呼回報送達或未送達 → `delivered` 或 `undelivered`。
- `[ASSUMPTION: 「暫時失敗／永久失敗」的分類依 mitake 風格假對端的定義；真實 Provider 的錯誤碼對照留待接入真實帳號時補。]`

### 4.3 發送佇列與重試

**說明：** 訊息入佇列後由背景工作送出。同一個 Provider 失敗時重試，仍失敗則標記 `failed`。v1 為單一程序加 in-process 佇列，**重啟會丟失佇列**，這是已接受的限制。

#### FR-7：訊息入佇列並由背景送出

系統將接受的訊息放入佇列，並由背景工作透過 Provider 送出。

**驗收：**
- 訊息 Status 依序變化：`queued` → `sending`（背景工作開始送往 Provider）→ `sent`（Provider 接受後）→ 終態（`delivered` 或 `undelivered`，由 Provider 回呼決定）；重試用盡或永久失敗則由 `sending` 直接進入 `failed`。
- 佇列介面為抽象介面，in-process 實作與介面分離，更換實作不需修改使用佇列的模組（以替換為測試用實作的測試證明）。

#### FR-8：同一 Provider 重試 3 次

Provider 暫時失敗時，系統對同一個 Provider 重試最多 3 次。

**驗收：**
- 假對端連續回暫時失敗時，Provider 共被呼叫 4 次（首次加 3 次重試），之後 Status 變為 `failed`。
- 假對端在第 2 次重試成功時，Status 不是 `failed`，且 Provider 不再被呼叫。
- 永久失敗（例如號碼無效）不重試，直接標記 `failed`。
- `[ASSUMPTION: 重試間隔採固定或指數退避，初始 1 秒；確切值於架構階段決定。]`

#### FR-9：不做跨 Provider failover

**驗收：**
- 假對端連續暫時失敗至重試用盡時，`mitake` 假對端共被呼叫 4 次，`mock` Adapter 被呼叫 0 次，Status 為 `failed`。
- 重試用盡後，系統不會改用另一個 Provider。

#### FR-28：重啟後的訊息命運

程序重啟會丟失佇列與記憶體內的訊息紀錄，這是已接受的限制，v1 需讓租戶可預期。

**驗收：**
- 重啟後，重啟前尚未進入終態的訊息不會被送出，租戶用 `GET /v1/messages/{id}` 查詢這些 `id` 回 404，error code 為 `not_found`（以測試證明）。
- 此限制寫入租戶接入文件，並在 §5 載明；租戶需自行處理「已收到 202 但訊息遺失」的情況。
- SM-2 的適用範圍僅限程序存活期間。
- `[ASSUMPTION: 終態訊息的保留期限與佇列上限於架構階段決定，見 §8 Q8。]`

### 4.4 狀態回報 Webhook

**說明：** 系統接收 Provider 回呼以更新 Status，再以 HMAC 簽章的 Webhook 通知租戶。同一個狀態只通知一次。實現 UJ-1。

#### FR-10：接收 Provider 回呼並更新狀態

系統提供端點接收 Provider 的狀態回呼，並更新對應訊息的 Status。

**驗收：**
- Provider 回呼只能回報 `delivered` 或 `undelivered`；收到其他值時回 400，error code 為 `invalid_request`，Status 不變。
- 收到對應已知、非終態訊息的有效回呼後，該訊息的 Status 更新為回呼所指的值。
- 回呼對應不到任何訊息時回 404，error code 為 `not_found`，不影響其他訊息。
- 終態（`delivered`、`undelivered`、`failed`）不可再改變：對終態訊息的任何回呼不改變 Status（以 `failed` 後收到 `delivered` 的測試證明）。

#### FR-11：以 HMAC 簽章通知租戶

Status 變成 `sent`、`delivered`、`undelivered` 或 `failed` 時，系統向租戶的 Webhook URL 發出 POST，並在 header 帶 HMAC 簽章。`queued` 與 `sending` 不通知。

**驗收：**
- 通知 body 含 `id` 與新 `status`。
- 一則正常送達的訊息，租戶的 Webhook 共收到 2 次通知（`sent`、`delivered`）；重試用盡的訊息收到 1 次（`failed`）。
- 用租戶的簽章密鑰對 body 重算 HMAC，結果與 header 簽章一致；用錯誤密鑰則不一致（以測試證明）。
- `[ASSUMPTION: 演算法為 HMAC-SHA256，header 名稱為 X-Signature；每個租戶有各自的密鑰與 Webhook URL，由維運者在設定檔中設定。]`

#### FR-12：同一 Status 只通知一次

**驗收：**
- Provider 對同一訊息、同一 Status 重複回呼 N 次，租戶的 Webhook 只收到 1 次該狀態的通知。
- 租戶的 Webhook 端點暫時回非 2xx 時，系統重試通知，且重試不產生重複的有效通知（以 `id` 加 `status` 去重）。`[ASSUMPTION: 通知重試最多 3 次；用盡後記錄失敗，不影響訊息 Status。]`

### 4.5 模板

**說明：** 租戶可管理自己的模板，發送時以 `template_id` 帶入變數渲染內容。

#### FR-13：模板 CRUD

租戶可建立、讀取、更新、刪除自己的模板。

**驗收：**
- 建立成功回 201 與模板 `id`；讀取、更新、刪除對象不存在時回 404，error code 為 `not_found`。
- 租戶只能操作自己的模板，操作其他租戶的模板回 404，error code 為 `not_found`。
- 模板內容無法解析（例如佔位符括號不成對）時回 400，error code 為 `invalid_template`。

#### FR-14：以 `{{變數}}` 渲染

系統將模板中的 `{{變數}}` 以送信時提供的值取代。

**驗收：**
- 模板「訂單 {{order_id}} 已出貨」加變數 `order_id=A123`，送出的內容為「訂單 A123 已出貨」。
- 渲染發生在入佇列之前，Provider 收到的是渲染後的內容。

#### FR-15：缺變數回 422

**驗收：**
- 送信時帶 `template_id` 但缺少模板需要的任一變數，回 422，error code 為 `missing_template_variable`，message 列出缺少的變數名稱，且不建立訊息。
- 帶有模板未使用的多餘變數時不報錯。
- 帶不存在或不屬於該租戶的 `template_id` 時回 404，error code 為 `not_found`。

### 4.6 速率限制

**說明：** 每個 API key 各自限流，避免單一租戶壓垮閘道或耗盡 Provider 額度。

#### FR-16：每租戶設定、每 API key 計數的限流

系統對每個 API key 獨立計算請求速率。

**驗收：**
- 同一 API key 在一分鐘內超過所屬租戶的上限時，後續請求回 429，error code 為 `rate_limited`。
- 一個 API key 被限流時，另一個 API key 的請求不受影響。
- 上限不寫死：每個租戶在設定檔中各自設定「每分鐘請求上限」，不同租戶可有不同數值；租戶 A 調高上限不影響租戶 B。
- 未設定上限的租戶使用 `[ASSUMPTION: 全域預設值，數值於實作時決定]`。
- 限流演算法留給架構與實作階段決定。
- `[ASSUMPTION: 計數單位為 API key，上限取自該 API key 所屬租戶的設定。]`

#### FR-17：429 帶 Retry-After

**驗收：**
- 每個 429 回應都帶 `Retry-After` header，值為正整數秒。
- 租戶等待 `Retry-After` 所指秒數後重送，請求成功（以測試證明）。

### 4.7 多租戶 API Key

**說明：** 每個租戶使用自己的 API key，租戶之間的訊息、模板與狀態互相不可見。

#### FR-18：驗證 API key

**驗收：**
- 請求沒有 `X-API-Key` header 時回 401，error code 為 `unauthorized`。
- 帶無效或已停用的 API key 時同樣回 401，error code 為 `unauthorized`。
- 所有 `/v1/*` 端點都需要有效 API key（Provider 回呼端點除外，見 FR-19）。

#### FR-19：Provider 回呼端點的驗證

系統驗證 Provider 回呼的來源，避免任何人偽造送達結果。

**驗收：**
- 未通過驗證的 Provider 回呼回 401，error code 為 `unauthorized`，且對應訊息的 Status 不變（以測試證明）。
- 通過驗證的回呼依 FR-10 處理。
- 驗證機制交由架構階段決定；`[ASSUMPTION: 不使用 API key，而以 Adapter 定義的方式，例如共用密鑰或來源驗證。]`

#### FR-20：租戶隔離

**驗收：**
- 租戶 A 用 `GET /v1/messages/{id}` 查詢租戶 B 的訊息，回 404，error code 為 `not_found`（不是 403，避免洩漏存在性）。
- 租戶 A 的列表結果中，不含租戶 B 的任何訊息。
- 租戶 A 對租戶 B 的模板做 CRUD，一律回 404，error code 為 `not_found`。
- 上述每一條都有自動化測試覆蓋。

#### FR-21：以設定檔建立與停用租戶、API key

維運者以設定檔管理租戶與 API key；v1 不提供管理 API。

**驗收：**
- 設定檔中新增的租戶與 API key，在載入後即可通過 FR-18 的驗證。
- 設定檔中標記為停用的 API key，載入後帶它的請求回 401，error code 為 `unauthorized`（以測試證明）。
- 設定檔中的 API key 以不可逆形式（雜湊）儲存，日誌中不出現 key 明文。
- `[ASSUMPTION: 設定變更需重啟程序才生效；重啟會丟失佇列（見 FR-28），因此變更需排在低流量時段。]`

### 4.8 E2E 測試門檻

**說明：** 這是流程性需求，由使用者指定：每個功能上線都要有自動化端對端測試。它是「完成」的定義，不是產品行為。

#### FR-22：每個功能都有 E2E 測試

**驗收：**
- 4.1 至 4.7 每一項功能，至少有一個自動化測試。
- 跨模組的行為（FR-1、FR-7、FR-8、FR-10、FR-11、FR-12）至少有一個端對端測試，從 HTTP 入口經佇列、`mock` Adapter 到 Webhook 出口走完整路徑。
- 沒有對應自動化測試的功能，不得視為完成。

#### FR-23：完成門檻

**驗收：**
- `go test ./... -race` 全部通過（零失敗、零 data race）才算完成。實作語言為 Go 是由此指令推得的約束 `[ASSUMPTION]`。
- 此命令納入 CI，失敗時阻擋合併。`[ASSUMPTION: 以現有 CI 平台為準，未決定前以本機命令為準。]`

#### FR-24：測試不依賴外部網路

**驗收：**
- E2E 測試只使用 `mock` Adapter 或假對端，在無外部網路的環境中也能全綠。

### 4.9 錯誤格式（橫跨所有功能）

#### FR-25：統一錯誤格式

**驗收：**
- 任何 4xx／5xx 回應的 body 都是 `{"error":{"code":"…","message":"…"}}`，`code` 為穩定的小寫底線字串，`message` 為人可讀說明。
- 以測試遍歷所有已定義的錯誤路徑，確認格式一致。

#### FR-26：錯誤碼清單

**驗收：**
- 至少涵蓋（各 FR 的 404 皆為 `not_found`，401 皆為 `unauthorized`）：`invalid_request`、`unauthorized`、`not_found`、`rate_limited`、`missing_template_variable`、`invalid_template`、`queue_full`（503，佇列滿載或關機中；對應 §8 Q8）、`internal_error`。
- 錯誤碼清單有文件，變更視為相容性變更。

**跨功能 NFR（本 PRD 規定的少數項目）：**
- 內部服務錯誤時回 500 且 error code 為 `internal_error`，body 不洩漏堆疊或內部資訊。
- 日誌不記錄簡訊內容明文與 API key `[ASSUMPTION: 簡訊內容視為敏感資料；可記錄 id 與 Status]`。

#### FR-29：發送紀錄不外送

法遵要求發送紀錄（收件號碼、內容、Status）不得外送；這是排除國外 CPaaS 的原因。

**驗收：**
- 除設定指定的 Provider 位址與租戶的 Webhook URL 外，系統不對任何其他目的地發出網路請求（以 E2E 測試攔截所有對外連線證明）。
- v1 的 Adapter 只能連設定指定的位址；真實 Provider 位址在公司外部時需經法遵確認 `[ASSUMPTION: 真實 Provider 接入時另行檢核]`。

## 5. 非目標

- **不承諾 99.9% 可用性**：重啟會丟失佇列與記憶體內的訊息紀錄（見 FR-28），也無法多副本。
- **不得承載驗證碼類關鍵簡訊**：這是對租戶的明確限制，v1 僅為政策約束，沒有偵測或攔截機制（見 §8 Q7）。
- 不做跨 Provider 自動 failover。
- 不與真實 Provider 帳號整合。
- 不使用國外 CPaaS：法遵排除（發送紀錄不得外送）。
- 不做 Web 主控台（純範圍取捨，不是永久排除）。
- 不做持久化佇列、多副本與監控（屬 v1 之後）。

## 6. MVP 範圍

### 6.1 範圍內

- §4 的八項功能與統一錯誤格式。
- 單一程序、in-process 佇列、佇列介面抽象。
- `mock` 與 `mitake`（假對端）兩個 Adapter。

### 6.2 範圍外

- 持久化佇列、多副本、監控、跨 Provider failover：屬 99.9% 目標階段，啟動條件尚未定義（見 Open Questions 第 5 題）。
- 真實 Provider 帳號整合。
- Web 主控台。
- 租戶與 API key 的管理 API。

`[NOTE FOR PM: v1 之後的階段若沒有明確的啟動條件，v1 可能變成永久的臨時方案。]`

## 7. 成功指標

**主要**
- **SM-1**：行為一致。每個換接的租戶，在換接後移除自己程式碼中的重試與狀態回呼邏輯，改由閘道的同一組行為（同一 Provider 重試 3 次、同一 Status 語意）取代；以接入檢核表逐租戶確認，基準為 §8 Q6 的差異清單。驗證 FR-8、FR-10、FR-11、FR-12。
- **SM-2**：單點查證。針對任一 Status 為 `failed` 或 `undelivered` 的訊息，只靠 `GET /v1/messages/{id}` 即可看到失敗原因與重試次數，不需查任何租戶的程式碼或日誌。僅限程序存活期間（FR-28）。驗證 FR-2、FR-27。
- **SM-3**：租戶隔離可驗證。FR-20 的所有隔離測試通過。驗證 FR-18、FR-20。
- **SM-4**：遷移準備度。v1 完成時，5 個租戶各自完成接入檢核表，並能以 `mock` Adapter 通過自己的整合驗證。v1 沒有真實 Provider，因此「5 個團隊兩季內全部改走閘道」的採用率目標移到真實 Provider 接入之後的階段，起算點與先後順序見 §8 Q4、Q5。驗證 FR-1、FR-5、FR-18。

**次要**
- **SM-5**：測試門檻。`go test ./... -race` 在主分支持續全綠。驗證 FR-22、FR-23。

**反指標（不要為了它們而最佳化）**
- **SM-C1**：不要為了提高遷移準備度（SM-4），讓關鍵簡訊（驗證碼）提早改走閘道。v1 不得承載這類簡訊；目前為政策約束，無法量測（見 §8 Q7）。
- **SM-C2**：不要為了測試全綠（SM-5），刪除或跳過 E2E 測試，或降低 `-race` 的檢查。

## 8. 待決問題

1. **（遷移前待辦）向五個租戶說明 v1 沒有解決原始事故**：驗證碼斷線 2 小時的事故不會因 v1 而消失。負責人待指定，須在第一個租戶換接前完成。（來自 brief Q1，不阻擋 PRD。）
2. **自建對比現成方案**：開源閘道、具 failover 的國內聚合商均未評估。這是最大的決策風險。**負責人：閘道團隊 tech lead。期限：架構開工前。** 若結論為採用現成方案，本 PRD 轉為遷移與包裝規格，並重新檢視 FR 範圍。（來自 brief Q2。）
3. **數字未查證**：自建 TCO 與「約 30 萬則／月」皆未查證；FR-16 各租戶的每分鐘上限應參考此數字。（來自 brief Q3。）
4. **遷移順序與兩季起算點**：未定義。採用率目標已移出 v1，起算點應在真實 Provider 接入之後。（來自 brief Q4。）
5. **後續階段的啟動條件**：持久化佇列與多副本何時啟動。（來自 brief Q5。）
6. **五個租戶現行行為差異清單**：addendum 建議補查，作為 SM-1 與遷移的基準，目前沒有。
7. **「不得承載驗證碼」的機制**：v1 維持純政策，不做偵測或攔截；之後再討論機制（例如租戶接入承諾欄位或檢核表）。
8. **訊息保留與佇列上限**：終態訊息在記憶體中的保留期限、佇列上限與滿載時的回應（429 或 503）、峰值吞吐假設。依賴 Q3 的數字，於架構階段決定。

## 9. 假設索引

- FR-1：收件號碼 E.164；內容長度上限 70 字元；API 回應時間 p95 < 200 ms。
- FR-3：列表排序與 limit／cursor 分頁，limit 預設 50、上限 200。
- FR-6：`mitake` 錯誤分類依假對端定義。
- FR-8：重試間隔與退避方式。
- FR-11：HMAC-SHA256、`X-Signature`、每租戶密鑰與 Webhook URL 由維運者設定。
- FR-12：Webhook 通知最多重試 3 次。
- FR-16：全域預設上限數值；計數單位為 API key。
- FR-19：Provider 回呼端點的驗證機制。
- FR-21：設定變更需重啟才生效。
- FR-23：CI 平台；實作語言為 Go。
- FR-28：保留期限與佇列上限（Q8）。
- FR-29：真實 Provider 接入時另行檢核。
- §4.9 NFR：簡訊內容視為敏感資料。
