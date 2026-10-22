---
reviewer: adversary
target: ARCHITECTURE-SPINE.md (status: draft, 2026-10-22)
reviewed_at: 2026-10-22T17:15:30+08:00
verdict: REVISE（不可直接作為下一層單元的共同基底）
---

# 對手審查：ARCHITECTURE-SPINE.md

方法：假設兩個下一層單元（Worker／Story）各自逐條遵守 AD-1~AD-17，找出仍會做出不相容結果的配對。每項標明「配對」「為何兩邊都合規」「後果」「補洞」。嚴重度 H／M／L。

## 判決

REVISE。狀態機（AD-5）與租戶隔離（AD-4）的骨幹紮實，但「單元之間的接縫」有多處只規定了方向、沒規定形狀：Provider 錯誤分類、通知 outbox 的擁有權與領取語意、租戶設定的擁有者、入佇列與建立訊息的順序、時間來源涵蓋範圍。以下 12 項，其中 6 項為 H。

---

## H-1 `Send` 的錯誤分類沒有契約（暫時失敗 vs 永久失敗）

- **配對：** `provider/mitake` 單元 vs `queue` 單元。
- **為何都合規：** AD-1 只說 `Send`、`ParseCallback`、`Name` 三個方法、不重試；AD-13 說「暫時失敗重試、永久失敗不重試」；AD-8 的 sentinel 清單沒有 `ErrTransient`／`ErrPermanent`。`provider` 的型別由誰定義、`Send` 回傳什麼 error 形狀完全未定。
- **後果：** mitake 作者把 HTTP 5xx／逾時回成帶字串的一般 error，永久失敗回自訂型別；`queue` 作者以 `errors.Is(err, X)` 判斷，未知 error 預設視為永久（直接 `failed`）或視為暫時（重試）。兩邊各自合規，FR-8 的「共 4 次呼叫」就會失準，mock 與 mitake 通過不了同一組契約測試（FR-4）。另有一個更深的變形：逾時但 Provider 其實已接受，重試造成重複簡訊，Interface 沒有冪等鍵或「結果不明」類別。
- **補洞：** 新增／收緊 AD-1：`provider` 套件匯出 `ErrTransient`、`ErrPermanent`（附 `Reason` 原因碼，不含號碼與內容）；`Send` 回傳的 error 必須 wrap 其中之一，未分類的 error 一律視為 `ErrPermanent`（或明訂為暫時，擇一）；明訂逾時／連線錯誤的歸類，並註明「結果不明」v1 視為暫時且接受重複風險 `[ASSUMPTION]`。契約測試套件放在 `provider/providertest` 共用。

## H-2 建立訊息與 `Enqueue` 的順序造成 `queue_full` 孤兒訊息

- **配對：** `httpapi`（依 Structural Seed：先 `store` 建立 `queued`，再 `Enqueue`）vs `queue`（有界，滿載回錯誤）。
- **為何都合規：** AD-14 要求滿載回 503 `queue_full` 且「不建立訊息」；AD-16 只規定 render 在 Enqueue 之前；序列圖是先建立後 Enqueue。httpapi 照序列圖做，Enqueue 失敗後訊息已存在於 `queued`，永遠不會被取走，且 GET 看得到、列表看得到、也不會被保留清除（非終態）。`httpapi` 作者若改成「先 Enqueue 再建立」，則 worker 可能先取走一個 store 裡還不存在的 id。
- **另一變形：** 重試的退避若實作為「重新 Enqueue」，重新入列也會撞上容量上限，訊息卡在 `sending`。AD-13 沒有說退避是佔住 worker 睡覺還是重新入列，AD-14 也沒說容量是否計入重試中的訊息。
- **補洞：** 新增 AD（或收緊 AD-2／AD-14）：`Enqueue` 與建立訊息為 queue 擁有的單一操作（例如 `queue.Submit(ctx, tenantID, draft) (id, error)`，內部先預留名額再寫 store，失敗全撤銷），或明訂順序「預留名額 → 寫 store → 放入」。另規定：重試在同一 worker 內以 `Clock.After` 等待、不重新入列、容量計算含重試中訊息；worker 數量為固定常數 `[ASSUMPTION]`。新增 sentinel `ErrQueueFull` 及其在 AD-8 的對應（`queue_full` 也不在 FR-26 的碼清單，需補文件）。

## H-3 Outbox 的狀態、領取與順序沒有擁有者

- **配對：** `webhook.Dispatcher` 單元 vs `store` 單元（或兩個 Dispatcher worker）。
- **為何都合規：** AD-5 規定 `OutboundNotification` 在轉移時入列、`(message_id,status)` 唯一；AD-3 說 Dispatcher「讀 outbox → POST」；AD-13 說遇非 2xx 重試。沒有任何一條定義：outbox 列是否有 `pending/done/attempts/next_at` 欄位、由哪個介面方法更新、如何「領取」避免兩個 Dispatcher goroutine 重複送、同一訊息的 `sent` 與 `delivered` 是否必須依序送出。
- **後果：** (a) store 作者只提供 `ListOutbox()` 唯讀；Dispatcher 作者自己在記憶體記「已送」，重啟前沒問題但 `-race` 下與 store 各有一份狀態。(b) `sent` 通知遇租戶 5xx 進入 1s/2s/4s 退避，期間 `delivered` 通知已成功送出，租戶先收到 `delivered` 再收到 `sent`，違反 UJ-1「先後收到 sent 與 delivered」。(c) 兩個 worker 同時取到同一列，租戶收到重複通知，雖符合 `(id,status)` 去重語意，但 FR-12 的測試「只收到 1 次」會失敗。(d) 租戶端回 2xx 但回應遺失而重送，v1 應明確接受或排除。
- **補洞：** 新增 AD：outbox 列欄位與狀態機（`pending→delivering→done|abandoned`）定義在 `store`，領取為 CAS（`ClaimNext`），只有 `Dispatcher` 持有該介面；Dispatcher 為單一 goroutine 且同一 `message_id` 的通知嚴格依 `sent` 先於終態順序送出（前一則完成或放棄後才送下一則），或明訂「可亂序，租戶以 status 自行排序」並改寫 UJ-1 的驗收。同時決定通知的 body 欄位（`id`、`status`，是否含 `failure_reason`）寫死在 spine。

## H-4 租戶設定（webhook URL／secret／限額）沒有單一擁有者，且依賴圖不允許取得

- **配對：** `webhook.Dispatcher`、`ratelimit`、`tenant`、`cmd/smsd`（建立 AD-9 允許清單）。
- **為何都合規：** 依賴圖中 `webhook` 只能 import `provider`、`store`；`tenant` 只 import `store`；`ratelimit` 不 import 任何內部套件。AD-11 說設定檔由某處載入一次，但沒說「Tenant」entity 是否在 `store`、由誰載入並寫入、webhook 如何取得 `webhook_url`／`webhook_secret`。ER 圖有 TENANT 與 API_KEY，但 AD-4 的 `TenantScoped`／`System` 兩個介面都未列出租戶查詢方法。
- **後果：** Dispatcher 作者為了拿 secret 自行解析 `SMS_CONFIG`（第二個載入者，違反「載入一次」），或 import `tenant`（違反箭頭）。`template`／`tenant` 與 `httpapi` 對 `store` 的存取也沒規定用哪個介面（`template` 做 CRUD 必須帶 tenantID，但 AD-4 只約束 `httpapi` 持有的 `TenantScoped`；`template` 手上拿的是什麼？）。`ratelimit` 的容量從哪來也未定（httpapi 傳入？）。
- **補洞：** 新增 AD：`cmd` 載入設定一次，轉為不可變的 `store.TenantConfig`（含 api key hash、`disabled`、`webhook_url`、`webhook_secret`、`rate_limit_per_min`）並經單一唯讀介面 `store.TenantDirectory` 提供；`tenant`、`webhook`、`ratelimit`（由 httpapi 傳入 limit，或明訂其建構子參數）皆只經此介面取值。補上依賴圖的 `webhook --> store` 已涵蓋此事的說明，並規定 `template` 持有 `TenantScoped`。順帶澄清 AD-11 的 `disabled` 是指 API key 還是租戶（FR-21 是 key 層級）。

## H-5 時間來源規則沒有涵蓋 `store`（以及誰蓋 `created_at`）

- **配對：** `store` 單元（保留 24 小時的背景清除、訊息時間戳）vs E2E／`queue`。
- **為何都合規：** AD-10 的 Clock 規則點名 `queue`、`ratelimit`、`webhook`、`httpapi`，不含 `store`、`template`、`tenant`。AD-14 要求 `store` 內有背景清除。`store` 作者用 `time.Now` 與 `time.NewTicker` 完全合規。
- **後果：** FR-22 的 E2E「終態訊息 24 小時後 GET 回 404」只能睡真實 24 小時或不測；訊息的 `created_at`／終態時間由 store 蓋章（真實時間），列表排序（FR-3）使用真實時間，而 queue／httpapi 在 FakeClock 下前進，造成排序與保留判定矛盾。清除用 ticker 也會與 `-race` 下的測試並行。
- **補洞：** 收緊 AD-10：所有內部套件（含 `store`）禁用 `time.Now`／`Sleep`／`After`／`NewTicker`，`store` 建構子必須接收 `Clock`；時間戳一律由 `store` 以注入 Clock 蓋章（`created_at`、`terminal_at`）；清除為可被測試直接呼叫的 `PurgeExpired(now)`，背景 goroutine 只是定時呼叫它。

## H-6 `sent` 狀態無期限，保留與容量上限仍可被擊穿

- **配對：** `store`（只清終態）vs Provider 行為（回呼遺失）。
- **為何都合規：** AD-14 只清「終態」訊息；`sent` 是非終態，要等 Provider 回呼才會進入 `delivered`／`undelivered`。AD-5 的轉移表沒有 `sent→` 逾時。
- **後果：** 回呼遺失、被 401 拒絕、或租戶多為 mitake 但回呼密鑰設錯，則所有該 Provider 的訊息永遠停在 `sent`，記憶體無界成長，AD-14 的「Prevents: 記憶體無界成長」失效；租戶也永遠看不到終態。
- **補洞：** 新增 `sent` 的最長等待（例如 24 小時 `[ASSUMPTION]`），到期由 `store` 清除（或新增 `sent→undelivered`，`failure_reason=回呼逾時`，走 AD-5 並產生通知）。需在轉移表與 AD-14 擇一明訂。

## M-1 `ErrIllegalTransition` 沒有 HTTP 對應，回呼重送會得到 500

- **配對：** `webhook.Inbound` vs `httpapi`。
- **為何都合規：** AD-5 說表外轉移回 `store.ErrIllegalTransition`；AD-8 的 sentinel 清單與 `writeError` 對應表沒有它，「未知錯誤一律 500」。FR-10 要求對終態的回呼「不改變 Status」，但沒說回什麼。FR-12 明確預期 Provider 會重送同一狀態 N 次。
- **後果：** Inbound 作者原樣回傳 error，httpapi 把它當未知錯誤回 500，真實 Provider 見到 500 就一直重送；另一作者在 Inbound 吞掉錯誤回 200。兩種行為在 E2E 看起來都「狀態沒變」。
- **補洞：** 收緊 AD-5／AD-7／AD-8：對終態／重複回呼，`Inbound` 視為已處理，回 2xx（冪等），不回錯誤；`ErrIllegalTransition` 只用於內部，若冒到 httpapi 必須映射為明確碼而非 500。並規定回呼回應 body 與狀態碼。

## M-2 Mock 的回呼投遞機制未定義

- **配對：** `provider/mock` 單元 vs `webhook.Inbound`／E2E 單元。
- **為何都合規：** 詞彙表：`mock` 不經網路；AD-1／依賴圖：`mock` 只 import `provider`；AD-9：只有 `cmd` 建 `http.Client`；AD-6：mock 必須在 `Send` 回應後才發回呼。三條合起來，mock 沒有任何合規管道把回呼送給 `Inbound`。
- **後果：** 作者 A 讓 mock 透過建構子注入的 callback func 直接呼叫 `Inbound`（需要 provider 型別裡有 `CallbackSink`）；作者 B 讓 E2E 測試自己呼叫 HTTP 回呼端點。FR-22 要求的「HTTP 入口→佇列→mock→Webhook」路徑，兩邊接法不同，而且 callback 是否也走 `ParseCallback` 驗證也不同。
- **補洞：** 新增 AD：`provider` 定義 `CallbackSink func(ctx, rawRequest)`，由 `cmd` 注入 mock，mock 在 `Send` 回傳之後（獨立 goroutine，經 Clock）呼叫它；該呼叫必須經 `webhook.Inbound`，與 HTTP 路徑共用 `ParseCallback`。

## M-3 長度上限與格式驗證的擁有者與單位不明

- **配對：** `httpapi`（無模板路徑）vs `template`（渲染後長度）。
- **為何都合規：** AD-16 說渲染後「仍須受 70 字元上限檢查」，沒說誰檢查；FR-1 的號碼 E.164 與長度檢查也沒有 AD 指派。「字元」未定義是 rune 還是 byte（中文是 3 bytes）。
- **後果：** httpapi 以 `len(s)`（bytes）檢查直接內容，template 以 `utf8.RuneCountInString` 檢查渲染結果：同一段 25 個中文字，直接送出被拒（75 > 70），經模板送出被接受。兩處都回 400 但碼可能一個是 `invalid_request`、一個被誤映射。
- **補洞：** 新增 AD：驗證集中在 `message` 驗證函式（放在 `httpapi` 或 `store` 的 `ValidateDraft`），單位為 Unicode code point（rune）；無論直接內容或渲染結果都走同一個函式；E.164 正規表示式寫死在 spine。

## M-4 `ProviderRef` 唯一性範圍與回呼對應

- **配對：** `mock`／`mitake` 兩個 Adapter vs `store.System` 的 `ProviderRef` 查詢。
- **為何都合規：** AD-4 只說可「以 ProviderRef 查訊息」；AD-6 回呼對不到回 404。沒說 ref 是否全域唯一、是否與 `{name}` 組成複合鍵。
- **後果：** mock 用從 1 起算的計數器、mitake 用 Provider 回傳的字串；若兩者撞號（切換 `PROVIDER` 後重啟不會，但測試中並存會）或租戶間撞號，查詢回傳到錯的租戶的訊息；`store.System` 作者做成 `map[ref]msg`，另一作者做成 `(name,ref)`。
- **補洞：** 收緊 AD-6：查詢鍵為 `(provider_name, provider_ref)`，在 store 層為唯一索引，衝突寫入回錯誤；回呼路徑的 `{name}` 必須等於啟用的 Provider，否則 404。

## M-5 `failure_reason`／`attempts` 在 `failed` 與 `undelivered` 轉移中的寫入者

- **配對：** `queue`（寫 failed）vs `webhook.Inbound`（寫 undelivered）vs `httpapi`（讀）。
- **為何都合規：** AD-6 只規定 `sent` 轉移寫 `provider_ref`、`attempts`；AD-5 的 CAS 只帶期望舊狀態；沒有規定 `sending→failed` 與 `sent→undelivered` 轉移要同時寫 `attempts`、`failure_reason`。AD-13 只規定 `failed` 的原因文字，`undelivered` 的原因（Provider 回報碼）無人定義，且 `ParseCallback` 的回傳形狀（是否帶 reason）未定。ER 圖中的 `DELIVERY_REPORT` 也沒有任何 AD 指派寫入者。
- **後果：** 一個單元用兩次寫入（先轉移再補原因），另一個在同一 CAS 內寫；`GET` 在兩次寫入的空隙看到 `failed` 而 `failure_reason` 為空，違反 FR-27「非空字串」；`undelivered` 的 `failure_reason` 常為空。
- **補洞：** 收緊 AD-5：每個轉移函式簽章為 `Transition(id, from, to, Patch{Attempts, ProviderRef, FailureReason})`，終態轉移必須在同一次 CAS 內帶 `FailureReason`（`failed`、`undelivered` 必填）；`provider.CallbackResult` 型別含 `Status`、`ProviderRef`、`Reason`；刪除或定義 `DELIVERY_REPORT`。

## L-1 `Enqueue` 的簽章與取走訊息的方式

- 簽章（傳 id？傳整則訊息？）未定，與 H-2 一併處理。

## L-2 分頁游標與清除競態

- FR-3 的 cursor 格式、同一時間戳的穩定排序、遍歷期間 24 小時清除造成的跳號未規定；兩個單元（httpapi 與 store）對游標是 opaque token 還是 `created_at` 各有解讀。補：cursor 為 store 產生的不透明字串，排序鍵 `(created_at, id)`。

## L-3 優雅關機順序

- `cmd` 的關機順序（先停 listener → 排空 queue → 排空 Dispatcher）未規定；`queue.Close` 丟棄的訊息（停在 `queued`/`sending`）與 outbox 中未送的通知無處理規則。程序隨即結束故影響小，但 E2E 對「Close 後狀態」的斷言會各寫各的。

---

## 優先處理順序

1. H-1（Provider 錯誤契約）、H-2（入佇列原子性）、H-3（outbox 擁有權）：最可能導致 E2E 互不相容。
2. H-4（租戶設定擁有者）、H-5（Clock 涵蓋 store）：會讓依賴圖與 AD-10 當場被違反或無法測。
3. M-1、M-2、M-5：把回呼／終態寫入的形狀定死。

## 做得好的部分（避免被誤改）

- AD-5 的單一轉移表加 CAS 加同交易入 outbox，是消除狀態競態的正確骨幹。
- AD-4 將 `TenantScoped` 與 `System` 分成兩個介面，用型別而非慣例防止跨租戶查詢。
- AD-8 單一 `writeError` 與 AD-9 注入受限 Client 都可機械驗證。
