# 查證審查：ARCHITECTURE-SPINE.md（sms-platform）

審查範圍：技術主張是否經查證、AD 是否與既有程式碼／文件矛盾。只讀未改 spine。

## 判決：有條件通過（需修訂後再定稿）

技術主張（Go 版本、ServeMux、slog、hmac 等）全部成立，但有 1 項主張遺漏了關鍵行為（ServeMux 的 404/405 純文字）。與既有程式碼的矛盾有 5 項需要在 spine 處理，其中 Clock 與組裝根兩項會直接擋住後續工單。

## 一、技術主張查證

查證方式：本機 `go version` 為 `go1.27.3 windows/amd64`；在 `go.mod` 為 `go 1.27.3` 的暫存模組實測（`C:\Users\<you>\AppData\Local\Temp\vt`）。Dockerfile 也鎖 GO_VERSION=1.27.3 並附 SHA256。

| 主張 | 結果 | 證據 |
| --- | --- | --- |
| Go 1.27.3 | 成立 | 本機工具鏈即此版；`go 1.27.3` 三段式指示詞合法（AGENTS.md 已說明）。Spine 的 Stack 表未標註查證來源，建議註明「本機 `go version` 與 Dockerfile 校驗」。 |
| ServeMux 方法路由 `"POST /v1/..."` 與 `{name}` 萬用字元 | 成立 | 實測 `POST /v1/providers/mock/callbacks` 回 200，`r.PathValue("name")="mock"`。`go.mod` 版本 >= 1.22 即啟用新行為。既有 `handler.go` 已用 `"GET /healthz"`。 |
| 衝突路由在註冊時 panic | 成立（spine 未提） | `GET /v1/{x}/a` 與 `GET /v1/a/{y}` 註冊時 panic。現行 openapi 路徑無此衝突，但新增路由要留意。 |
| `log/slog` JSON 輸出 | 成立 | `slog.NewJSONHandler` 實測可用。注意：預設 `time` 欄位是本地時區（實測 `+08:00`），與 Consistency「時間為 UTC、RFC 3339」不一致，需用 `ReplaceAttr` 轉 UTC 或明定日誌例外。 |
| `crypto/hmac`、`crypto/sha256`、`hmac.Equal` | 成立 | 實測。AD-7「constant-time 比較」可用 `hmac.Equal` 或 `crypto/subtle`，spine 宜指名。 |
| 「標準庫沒有 YAML」 | 成立 | 標準庫只有 `encoding/json`、`xml` 等，無 YAML。 |
| `crypto/rand` 產 128 位元 hex | 成立 | 標準庫能力，無疑義。 |

### 遺漏的行為（影響 AD-8）

ServeMux 對未知路徑與方法不符會自行回應，且不是錯誤信封：

- `GET /nope` 回 404，body `404 page not found\n`，`text/plain`。
- `GET /v1/providers/mock/callbacks` 回 405，body `Method Not Allowed\n`，帶 `Allow: POST`。
- `POST /healthz` 回 405，`Allow: GET, HEAD`。

AD-8 主張「錯誤信封只在 httpapi 產生、單一 writeError」，FR-25/26 要求一致格式。若不處理，未知路徑與錯誤方法會破壞契約。spine 需補一條：以 catch-all `"/"` 與 405 包裝（或前置 wrapper）把這兩類轉成信封，並定下 code（例如 `not_found`、`method_not_allowed`）。

## 二、與既有程式碼／文件的矛盾

### 高：AD-10 的 Clock 與現況不符

- AD-10 規定注入 `Clock`（`Now()`、`After(d)`）。現況：`httpapi.Deps.Now` 是 `func() time.Time`，`mock.NewWithClock(now func() time.Time)`、`FakeProvider` 同樣吃 `func() time.Time`；AGENTS.md「套件邊界」明文寫「時間一律經注入的 `func() time.Time`」。
- `e2e/testutil/clock.go` 的 `FakeClock` 只有 `Now/Advance/Set`，沒有 `After`，也沒有等待者機制。AD-13 的 1s/2s/4s 退避要在「不睡真實時間」下測，需要能被 `Advance` 喚醒的 `After`。
- `FakeProvider.Send` 內用真實 `time.After(delay)`（testutil 不在 AD-10 約束內，但與「不睡真實時間」精神相左）。
- 處置：spine 要明講是「取代」`func() time.Time`，列出受影響檔案（handler.go、mock.go、testutil 三檔、AGENTS.md 該條規則），並指定 `FakeClock.After` 的語意。同時 AD-10 清單漏了 `store`：AD-14 的 24 小時保留清除與 `updated_at` 都要時間，卻不在「queue、ratelimit、webhook、httpapi」之列。

### 高：組裝根與 E2E 的接線

- AD-9／Design Paradigm 規定 `http.Client`、queue、webhook 都在 `cmd/smsd` 組裝，cmd 無邏輯。但 `cmd` 是 `package main`，無法被 `e2e/testutil` import。現況 `NewTestServer` 只呼叫 `httpapi.NewHandler(Deps{Now, Provider})`，AGENTS.md 說它包的是「cmd/smsd 同一個 handler」。
- 加入 store、queue、Dispatcher、受限 `http.Client`、設定後，E2E 要嘛重複一份組裝（測試與正式碼分叉），要嘛需要一個可 import 的組裝函式。AD-9 說「E2E 以此注入點攔截所有對外連線」，卻沒說注入點在哪個可 import 的位置。
- 處置：spine 需指定組裝函式的位置（例如 `internal/app` 的 `Build(Deps)`，cmd 只呼叫它），並更新依賴圖與目錄樹；或明定 `httpapi.Deps` 擴充的欄位。

### 中：AD-1 的「只有 cmd 可 import mock」已被現況違反

`internal/httpapi/handler.go` import `internal/provider/mock`，`Deps.withDefaults` 在 Provider 為 nil 時用 `mock.New()`；`cmd/smsd/main.go` 以 `httpapi.Deps{}` 啟動，即靠這條預設路徑。AD-1 與依賴圖明禁。spine 應列為「待清除的既有違規」，並指定 `Deps.Provider` 改為必填、預設值移到 cmd。

### 中：依賴圖不完整，與規則自相矛盾

- 規則「依賴只能依箭頭方向」，但圖中 `cmd` 只指向 httpapi、queue、webhook，未連到 store、tenant、ratelimit、template、provider 及兩個 Adapter。要組裝就必須 import 這些。
- `httpapi` 現況已 import `provider`（`Deps.Provider`），圖上沒有 `httpapi → provider`。
- 處置：補齊 `cmd` 的邊，或改敘述為「cmd 可 import 所有 internal/*」。

### 中：AD-13 需要的暫時／永久失敗，現行介面沒有

`provider.Provider.Send` 只回 `(ProviderReceipt, error)`，`provider.go` 與 `mock.go` 都沒有區分暫時與永久失敗的型別或 sentinel；`FakeProvider` 用任意 `error`。AD-1 說介面「最小化」，AD-13 卻依賴此區分。spine 需定義契約（例如 `provider.ErrTemporary`／`ErrPermanent`，以 `errors.Is` 判斷，或 `PermanentError` 型別），並說明無法分類的錯誤預設如何處理。同理 AD-7 的 `provider.ErrUnauthorized` 目前不存在，`mock.ParseCallback` 缺 ref 時只回 `fmt.Errorf`，無法對應 AD-8 的 400/422，也沒有密鑰驗證。

### 中：回呼狀態值與 AD-5 轉移表

`provider.Status` 含 `sent`、`delivered`、`undelivered`、`failed` 四值，但 AD-5 只允許回呼驅動 `sent→delivered`、`sent→undelivered`。供應商回 `sent` 或 `failed`，或訊息尚在 `sending` 時的 `delivered`，會得到 `ErrIllegalTransition`，spine 沒說 HTTP 回什麼（openapi 只定義 204 與 404）。需補：非法轉移對回呼端點的回應碼（建議 204 或 409，並說明對供應商重送的影響）。

### 中：Sequence 圖與 AD-14 順序相反

圖中先「H->>S 建立訊息（queued）」再 `Enqueue`。AD-14 規定佇列滿載回 503 且「不建立訊息」。照圖實作會留下無人處理的 queued 孤兒訊息。需改順序，或規定 Enqueue 失敗時刪除該訊息，並指定原子性由誰負責。

### 低：命名與文件漂移

- spine 用 `webhook_url`、`webhook_secret`、`rate_limit_per_min`；`docs/README.md` 資料模型、`webhook/doc.go`、`openapi.yaml` 註解用 `callback_url`、`callback_secret`、`rate_limit`。PRD 用「Webhook URL」。spine 應宣告以哪個為準並列出需同步的檔案。
- `openapi.yaml`、`webhook/doc.go`、README 的 OutboundNotification 都寫「每則訊息最多通知一次／delivered_once」；AD-5 是每個 `(message_id, status)` 一次，一則訊息最多 4 次通知，且實體欄位不同（無 `delivered_once`）。這是有意的演進，但 spine 未標明取代，易讓後續工單依舊文件實作。
- `docs/README.md` 的狀態機畫了 `sending → queued`（重試，attempts+1）；AD-5/AD-13 是重試期間維持 `sending`。spine 比較合理，但應註明該草圖作廢。
- 優雅關機：現況 `main.go` 為 5 秒，Deferred 寫 10 秒 `[ASSUMPTION]`，應列為變更。
- `main.go` 使用 `log`，AD-15 要 `slog`；屬預期的新增工作，不算矛盾，但可列入遷移清單。
- AD-4 的 `httpapi` 持有 `store.TenantScoped`，AGENTS.md 寫「`httpapi` 不碰 store 的資料結構細節」。持有介面不等於碰結構，但 httpapi 要序列化 Message，措辭宜釐清。
- AD-12「每 API key 一桶」但容量取自租戶設定；AD-11 未說明一租戶是否可有多把 key。若多把 key，租戶總額度會變成 N 倍，需明定。

## 三、其他備註

- 引用的 `internal/*/doc.go` 與 `Provider` 三方法（`Send`、`ParseCallback`、`Name`）、`/v1/providers/{name}/callbacks` 路徑、`security: []`、404 語意，與 spine 一致。
- 出站 webhook 的 `X-Signature`、HMAC-SHA256 與 PRD FR-11 一致。
- `http.Client` 的 Transport 允許清單：spine 未提 `CheckRedirect`。轉址可繞過允許清單，建議一併規定禁止跟隨轉址或每跳重驗。

## 建議優先順序

1. 補 ServeMux 404/405 的信封處理（AD-8）。
2. 重寫 AD-10 與 FakeClock 的遷移，並把 store 納入。
3. 指定可被 E2E import 的組裝位置，修正依賴圖。
4. 定義 Provider 錯誤分類契約與 `ErrUnauthorized`。
5. 修正 sequence 圖順序與命名／文件漂移清單。
