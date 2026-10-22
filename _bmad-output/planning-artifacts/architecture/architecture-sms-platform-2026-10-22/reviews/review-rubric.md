# 審查：ARCHITECTURE-SPINE.md（對照 PRD 與 addendum）

審查日期：2026-10-22。範圍：spine 全文；對照 `prd.md`（FR-1~29、§4.9 NFR、§8）、`addendum.md`；另抽查 repo 現況（`internal/*/doc.go`、`e2e/testutil/*`、`Dockerfile`、`compose.yaml`、`go.mod`），因為 spine 的 sources 與部署段落引用了它們。未修改 spine。

## 判決

**有條件通過（需修訂後才能當實作基底）。** 結構與 FR 追蹤相當完整：29 條 FR 全在 `binds`，addendum 的 5 項「留給架構階段的決定」全部有決定。但有 3 處「規則不足以防止它宣稱防止的事」，以及 4 個會讓兩個獨立建構的單元分歧的契約缺口，另外部署／營運維度幾乎沉默。

---

## 1. 逐條 AD：Rule 可否執行、能否防住 Prevents

| AD | 判定 | 說明 |
| --- | --- | --- |
| AD-1 | 部分 | 「不重試、不 sleep」是行為規則，但**沒有定義 Adapter 如何回報暫時／永久失敗**（錯誤型別或結果欄位）。AD-13 依賴這個分類，queue 與兩個 Adapter 各自建構時必然各猜各的。「其他套件不得 import mock/mitake」沒有任何檢查機制（見 §A）。 |
| AD-2 | 可執行 | 介面隔離清楚。小缺口：`Enqueue` 帶什麼（只有 id，還是完整內容）、worker 數量、佇列上限被重試中的訊息佔用與否，都沒定。 |
| AD-3 | 部分 | 去重鍵清楚。但 Webhook body 欄位、簽章編碼（hex 或 base64、有無 `sha256=` 前綴）未定，Dispatcher 與 E2E 驗章／租戶接入文件會分歧。同一 `(message_id,status)` 的「只送一次」實為至少一次（回應遺失時重送），應明寫。 |
| AD-4 | 部分 | 型別分離（`TenantScoped`／`System`）是少見的可機械檢查設計，但「`httpapi` 不得取得 `System`」靠慣例，沒有測試或 lint。空字串回錯誤這條可測。 |
| AD-5 | 大致可 | CAS＋同交易寫 outbox 是強規則。缺口：(1) 表外轉移回 `ErrIllegalTransition`，但**對回呼端回什麼 HTTP**（對終態訊息的回呼、重複回呼）沒定，也不在 AD-8 的 sentinel 映射裡；FR-10 只規定「Status 不變」。(2) `attempts`、`failure_reason`、`provider_ref` 不是 status 欄位，「不得自行改 status 欄位」管不到它們；`sending→failed` 如何寫入 `attempts=4` 與 `failure_reason` 未說（FR-27 驗收需要）。 |
| AD-6 | **不成立** | Prevents 是「回呼對得到訊息卻狀態仍是 sending」。Rule 要求假對端「在 Send 回應之後才發回呼」，但對端無從得知 queue 的 CAS 何時完成，Send 回應與 CAS 之間仍有競態窗口；E2E 會變成偶發失敗，真實 Provider 則會遺失 `delivered`（回呼 404，訊息永遠停在 `sent`）。Deferred 提到「需重評」，但 v1 的假對端／E2E 就會踩到。需要一條可執行的規則：例如假對端收到 404 回呼時以 FakeClock 退避重送，或 Inbound 對「找不到 ref」做有界等待，並明寫哪個。 |
| AD-7 | 部分 | 驗證位置明確。缺：密鑰 header 名稱與環境變數名稱；`{name}` 與目前啟用的 `PROVIDER` 不符時回應（404？）；`ParseCallback` 的簽名與回傳型別（見 §B-3）；回呼 `status` 非 delivered／undelivered 時由誰回 400。 |
| AD-8 | **有漏洞** | `net/http` `ServeMux` 對未知路由／不允許的方法回的是 text/plain 的 404／405，違反 FR-25「任何 4xx／5xx body 都是錯誤信封」。規則只說領域錯誤經 `writeError`，沒有涵蓋 mux 預設回應、JSON 解析失敗、body 過大、panic recover。「錯誤碼清單文件化」沒指定文件位置（openapi？）。 |
| AD-9 | 部分 | 注入點設計合理（Transport 白名單同時涵蓋 redirect）。「其他程式碼不得建立 `http.Client`」無檢查。HTTP 逾時未定（Provider 或租戶端掛住會卡 worker／Dispatcher）。 |
| AD-10 | **有漏洞** | 清單列了 queue、ratelimit、webhook、httpapi，**漏了 `store`**，但 AD-14 的 24 小時保留與背景清除、`created_at` 時戳都在 store；E2E 無法用 FakeClock 驗證過期。另外 `Clock` 型別定義在哪個套件沒說，且與現況衝突（見 §D）。 |
| AD-11 | 部分 | 啟動失敗規則清楚可測。缺：`disabled` 是租戶層還是 API key 層（FR-21 說的是 API key）；Provider 位址、回呼密鑰、監聽位址的環境變數名；設定載入程式放哪個套件（`cmd/smsd` 被限定「無業務邏輯」，樹中又沒有 `config`）。`webhook_secret` 明文放設定檔，未說明保管方式。 |
| AD-12 | 可執行 | 演算法、單位、順序、Retry-After 公式都明確。小缺口：`rate_limit_per_min` 為 0 或負數的驗證；是否涵蓋模板 CRUD 與 GET（隱含為全部 `/v1/*`，未明說）。 |
| AD-13 | 大致可 | 數值明確。缺：Provider 暫時失敗的判定來源（見 AD-1）；重試是 worker 睡眠還是重新入佇列（影響 AD-14 容量與吞吐）；出站通知遇網路錯誤／逾時是否同樣重試（只寫非 2xx）；Dispatcher 輪詢頻率與併發，重試中的 `sent` 通知可能晚於 `delivered` 通知到達，順序性未說（UJ-1 寫「先後收到」）。 |
| AD-14 | **與 Seed 矛盾** | 「滿載回 503 且不建立訊息」，但 Structural Seed 的順序是先 `H->>S 建立訊息` 再 `H->>Q Enqueue`；若 Enqueue 失敗，訊息已存在，違反「不建立訊息」。反過來先 Enqueue 再建立，worker 可能對不存在的訊息做 CAS。需決定順序或補償。另外「無界成長」未真正防住：**非終態的 `sent` 訊息永不過期**（回呼沒來就一直留著）、OutboundNotification 沒有清除規則、模板數量無上限。 |
| AD-15 | 可執行 | 欄位白名單明確。無自動檢查，建議以 E2E 攔截 slog 輸出斷言（FR-21 要求日誌無 key 明文）。 |
| AD-16 | 大致可 | 順序規則清楚。缺：FR-1 的「內容或 template_id」兩者同時出現、兩者皆無時的行為；E.164 與 70 字元驗證放哪（只提渲染後長度）。 |
| AD-17 | 可執行且可機檢 | `go.mod` 無 `require` 可由測試斷言，是全文唯一天然可機檢的規則。 |

### §A 共通問題：規則沒有執行機制
AD-1、AD-4、AD-9、AD-10 與依賴圖（「queue 與 webhook 不得互相 import」等）都是禁止性規則，但 spine 的完成門檻只有 `go test ./... -race`（FR-23）。沒有任何一條說明由誰、用什麼檢查。建議新增一條 AD 或 Consistency Convention：以 `go list -deps` 或 `go/packages` 寫一個架構測試（例如 `internal/archtest`），斷言依賴圖、`http.Client` 與 `time.Now` 的使用位置、`go.mod` 無 require。標準庫即可，符合 AD-17。

## 2. Deferred 項目：是否讓兩個單元分歧

Deferred 清單本身多為合理的 v1 之後項目，沒有直接導致分歧的項目，但有兩項「應從 Deferred 升級為 v1 決定」：

1. **「真實 Provider 的錯誤碼對照與回呼驗證」**：PRD FR-6 的 (a)~(d) 分類「依假對端定義」，但 spine 與 PRD 都沒有人定義假對端的線上協定（請求格式、回應碼、暫時／永久失敗的表示法、回呼 payload）。`mitake` Adapter 與假對端若由兩個單元分別建構，必然分歧。這不是 Deferred（那是真實 Provider），是 v1 必須先定的契約，目前既不在 AD 也不在 Deferred，也無假對端的目錄位置。
2. **「優雅關機排空 10 秒」**：只影響 `cmd`，無分歧風險。

另有「OpenAPI 細節以 docs/openapi.yaml 為準」：可接受，但 FR-3 的 cursor 格式、GET body 欄位（`attempts` 何時出現、`failure_reason` 何時出現）、模板路徑都只能靠它，而 openapi.yaml 現況是否涵蓋需確認；建議 spine 至少點名 Message 的 JSON 表示由哪個單元擁有。

## 3. FR-1~29 與 addendum 決定項覆蓋

- **FR 追蹤**：29 條皆在 `binds` 與 Capability Map，無遺漏。
- **覆蓋薄弱（被 bind 但沒有實質決定）**：FR-3（排序、分頁、cursor 無 AD）、FR-14（`{{ }}` 語法細節，如空白、轉義）、FR-22（只靠 Testing 慣例）、FR-23（CI 平台完全未提，只剩本機指令；PRD 標為 ASSUMPTION，spine 應延後或決定）。
- **addendum「留給架構階段的決定」**：重試與退避（AD-13）、回呼驗證（AD-7）、限流演算法與數值（AD-12，預設 60）、去重鍵與通知重試（AD-5、AD-13）、保留／上限／滿載回應（AD-14，選 503 `queue_full`）。**5/5 已決定。** 唯 `queue_full` 不在 PRD FR-26 的最低錯誤碼清單，需同步進錯誤碼文件。
- **PRD §8**：Q2 以 `[ASSUMPTION]` 前提處理並列於 Deferred（正確）；Q3 延後（正確）；Q8 已決定；Q1、Q4、Q5、Q6 非架構議題，不處理可接受；**Q7（不得承載驗證碼）未被任何段落提及**，應至少列在 Deferred 或非目標，並說明 v1 無結構性支撐（例如租戶設定沒有「允許類別」欄位）。
- **下游文件義務遺漏**：FR-28 要求「寫入租戶接入文件」；spine 的樹與 Map 沒有此文件的位置。

## 4. 整個維度沉默

**部署與環境（最大缺口）**：spine 寫「部署：單一程序、單副本（既有 `Dockerfile`、`compose.yaml`）」，但實查這兩個檔是 **BAT 開發容器**（基底映像 `bat-server`、僅開發用，埠 9880 對應 BAT server），**不是 smsd 的執行環境**。因此：沒有 smsd 的執行映像、沒有監聽位址／埠、沒有環境變數清單與範例設定、沒有健康檢查端點（`/healthz` 在 `e2e/healthz_test.go` 已存在，但 spine 的 Auth 規則「`/v1/*` 需 key」未說明它的路徑與是否免驗證）、沒有 secrets 管理（`webhook_secret` 在設定檔明文）、沒有設定變更如何低流量重啟的流程（FR-21 假設）。
**營運**：日誌只有識別資訊且無 request id／關聯欄位，事故排查（PRD 的核心動機）只能靠 message_id；沒有說明 slog 等級、關機行為對使用者的可見影響；「出站通知用盡只記 slog」意味 Webhook 永久失敗對維運者幾乎不可見，而監控又被整體延後，建議至少定義一個必記的 slog 事件名稱。**CI**：未提。

## 5. PRD 安靜的需求是否被 AD 結構丟掉

- **風險等級「內部工具」、不承諾高可用**：spine 有接受重啟遺失，但沒有把「內部工具」的取捨轉成對複雜度的限制（例如 AD 數量已達 17，部分如 token bucket 的 ceil 公式對內部工具偏精細，但不傷害）。可接受。
- **詞彙表紀律**：PRD §3 要求只用「Provider」，spine AD-3 與 Deferred 仍用「供應商回呼」；且 spine 用「outbox」「Dispatcher」等新詞而未在 spine 內定義。小問題。
- **FR-28／SM-2 的相互作用**：AD-14 的 24 小時保留使 SM-2「只靠 GET 看失敗原因」在 24 小時後失效，spine 未提醒需寫入接入文件。
- **SM-4（5 個租戶以 `mock` 通過整合驗證）**：`mock` 被當成租戶可用的環境，但 spine 未定義 mock 的可程式化行為（如何觸發暫時失敗、永久失敗、何時回呼 delivered）。FR-22 要求「經 `mock` 走完 HTTP→佇列→Webhook 全程」，而 PRD 詞彙說 `mock` 不經網路，AD-6 又要求 mock 在 Send 之後發回呼，**mock 如何發出回呼**（`provider/mock` 只能 import `provider`）沒有答案。
- **SM-C2**：不為全綠而降低 `-race` 或跳過 E2E——spine 的 Testing 慣例支持，但沒有防呆；與 §A 的架構測試同理。

## B. 結構缺口（兩個單元會分歧）

1. **依賴圖不完整**：圖中 `cmd` 只指向 httpapi、queue、webhook，但 `cmd` 必須組裝 store、tenant、ratelimit、template、兩個 Adapter；依 Rule「依賴只能依箭頭方向」,這些 import 全被禁止。`httpapi` 沒有指向 `provider`，卻要映射 `provider.ErrUnauthorized`。`Clock` 型別與 config 載入的套件不在圖與目錄樹內（若 `Clock` 定義在 queue，ratelimit 就得 import queue）。
2. **租戶解析位置**：AD-4 說 `tenant` 在 middleware 解析 key；現有 `internal/httpapi/doc.go` 寫 httpapi 擁有 API key 擷取、tenant 擁有查找。需定一個。
3. **Provider 契約型別**：Send／ParseCallback 的參數、回傳（receipt、暫時／永久錯誤、DeliveryReport）未在 spine 定義。repo 已有 `provider.OutboundMessage`、`ProviderReceipt`、`DeliveryReport` 與 `e2e/testutil/FakeProvider`，spine 的 sources 只列 doc.go，沒有對齊既有程式碼。
4. **Clock 與現況衝突**：AD-10 規定 `Clock{Now, After}`，但既有 `e2e/testutil.FakeClock` 只有 `Now/Advance/Set`，註解寫的是注入 `func() time.Time`；`FakeProvider` 內直接用 `time.After`。需決定是改 testutil 還是改 AD-10，否則 FR-8 的 E2E（退避 1s、2s、4s）無法在假時間下執行。

## C. 前 5 項優先修正

1. 補 Provider 契約（錯誤分類、回呼型別）與假對端／mock 的行為與回呼協定；指定假對端目錄位置（AD-1、AD-6、FR-6）。
2. 修 AD-6 的競態：給出假對端與 Inbound 對「早到回呼」的可執行處理。
3. 解決 AD-14 與 Structural Seed 的建立／入佇列順序矛盾，並補 `sent` 訊息與 outbox 的清除。
4. 補部署段落：smsd 的執行映像／埠／環境變數／健康檢查，並更正「既有 Dockerfile 即部署」的說法（其實是開發容器）。
5. 補依賴圖（cmd、Clock、config、provider→httpapi）、AD-8 的 mux 預設回應，以及一個架構測試作為所有禁止性規則的執行機制。
