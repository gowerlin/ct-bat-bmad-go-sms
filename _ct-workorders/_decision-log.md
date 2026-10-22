# Decision Log

## 索引

| # | 日期 | 決策 | 相關工單 |
|---|------|------|---------|
| D001 | 2026-10-22 | 只用標準庫 `net/http`＋`httptest`，不引入第三方套件 | T0001 |

| D002 | 2026-10-22 | API key 只從 header `X-API-Key` 讀，query 帶 key 視為沒帶 | T0003 |
| D003 | 2026-10-22 | 路由維持標準庫 `http.ServeMux`，中介層用函式包裝＋`chain` helper（A1），不引入 chi；不推翻 D001 | EXP-ROUTER-001、T9002 |
| D004 | 2026-10-22 | httpapi 每功能一檔，以明確 stub 預留接口（不用 `init()` registry），讓並行工單不撞檔 | T9003 |
| D005 | 2026-10-22 | 模板 E2E 暫以 store 讀回的 `Body` 驗證已渲染；FakeProvider 驗證待 queue 送信 worker 補 | T0004 |
| D006 | 2026-10-22 | `ErrInvalidTemplate` 回 400 `invalid_template`（照 `docs/error-codes.md`），工單的 `invalid_request` 是筆誤 | T0004 |
| D007 | 2026-10-22 | `vars` 為 JSON 物件、值只收字串；非字串回 400 `invalid_request` | T0004 |
| D008 | 2026-10-22 | T9004 以模式 ① 機械合併回 `main`（五條判準全過，塔台直接合併，不派合併工單） | T9004 |
| D009 | 2026-10-22 | 限流：每 API key 一桶，容量＝租戶 `rate_limit_per_min`、每秒補 `rate_limit_per_min/60`；上限放租戶設定、預設 60（使用者決定） | T0005 |
| D010 | 2026-10-22 | `Retry-After` 取 `max(1, ceil(秒數))`（使用者決定） | T0005 |

---

## 詳細紀錄（倒序）

### D010 2026-10-22 — `Retry-After` 取 `max(1, ceil(秒數))`

- **背景**：T0005（yolo）Worker 經 BAT 通道提問 Q-2（提問時間 2026-10-22T23:58:07+0800）：被限流時從「等到下一枚 token 的秒數」換成 `Retry-After` 正整數秒，用哪種取整、最小值多少。工單明定 Worker 不得自行決定。
- **選項**：
  - [A] `max(1, ceil(秒數))`，秒數 = `(1 - 目前 tokens) / 每秒補充率`（AD-12 草案）
  - [B] `floor(秒數) + 1`：剛好整數秒時多等 1 秒
- **決定**：選 A（使用者決定，4-10 錄製前指定；塔台 2026-10-22T23:58:42+0800 以 `--answer --choice 1` 經 BAT 通道答覆，exit `0`，工單不寫答覆行）。理由：ceil 保證客戶端照 header 等完一定有 token、不會再被拒；`max(1, …)` 保證 header 是正整數（`openapi.yaml` 宣告 integer）。
- **影響**：`internal/ratelimit` 的 `Retry-After` 計算與單元測試、`e2e/ratelimit_test.go` 的斷言依此實作；改取整規則須開新決策取代本條。
- **相關工單**：T0005

### D009 2026-10-22 — 限流桶：容量＝租戶 `rate_limit_per_min`，每秒補 `/60`，上限放租戶設定、預設 60

- **背景**：T0005（yolo）Worker 經 BAT 通道提問 Q-1（提問時間 2026-10-22T23:58:07+0800）：桶容量、補充速率、限額來源與未設定時的預設。`internal/tenant/tenant.go` 原本沒有限額欄位，限額來源須一併決定。
- **選項**：
  - [A] `tenant.Tenant` 加 `RateLimitPerMin`；每個 API key 一桶，容量 = 所屬租戶 `RateLimitPerMin`，補充率 `RateLimitPerMin/60` token/秒（連續補充），桶初始為滿；`<= 0`（未設定）用預設 60
  - [B] 只用全域設定（`ratelimit` 建構時給每分鐘上限，預設 60），租戶不帶欄位
- **決定**：選 A（使用者決定：容量等於租戶的每分鐘上限 `rate_limit_per_min`，每秒補 `rate_limit_per_min/60` 個，上限放在租戶設定、預設 60）；塔台 2026-10-22T23:58:42+0800 以 `--answer --choice 1` 經 BAT 通道答覆，exit `0`，工單不寫答覆行。理由：與 AD-12、`docs/README.md` 資料模型（Tenant 有每分鐘 `rate_limit`）及工單「限額由租戶帶入」一致。
- **影響**：`tenant.Tenant` 新增每分鐘上限欄位；同租戶多把 key 各一桶、各用租戶限額；`e2e/testutil` 可把測試租戶設小限額以少送請求。設定檔載入 story 上線後，租戶設定檔要帶這個欄位；分散式限流不在範圍。
- **相關工單**：T0005

### D008 2026-10-22 — T9004 以模式 ① 機械合併回 `main`

- **背景**：T9004 以 `--isolation worktree` 派出（yolo），Worker 回報 DONE，`ct/T9004` 要合併回 `main`。照 `merge-workorder.md` 判準決定模式。
- **判準**（`--worktree-status T9004`，`source: live`）：① `behind 2`，`base..mainHead` 只有 `_ct-workorders/T9004-onboarding-bat-section.md`（放寬版成立）；② `validated_sha` = `head` = `86e3d7e`；③ `outOfScopeFiles []`、`touchesCtWorkorders false`；④ `conflicts []`；⑤ `dirty []`，主 checkout 髒檔只有 `_tower-state.md`，與 `changedFiles`（`docs/dev-workflow.md`）不重疊。ff 前重查仍成立。
- **決定**：模式 ①，塔台在整合 worktree `..\.ct-integration\sms-platform`（本次首次建立）開 `ct-merge/T9004` 自 `base_main_sha` `94fecfa`，`--no-ff` 合併得 merge commit `6107732`（parent1 `94fecfa`、parent2 `86e3d7e`）；yolo 斷點 E 自動 `--ff-only`，ff 後 `main` = `6107732`。合併結果與 `head` 只差 `_ct-workorders/**`，不重跑驗收。
- **影響**：`--close-worker T9004 --worktree remove` exit `0`，Worker 分頁已結束、T9004 worktree 已移除；整合 worktree 已 `switch --detach`。分支 `ct/T9004`、`ct-merge/T9004` 保留，待使用者同意後以 `git branch -d` 刪除。
- **相關工單**：T9004

### D007 2026-10-22 — `vars` 為 JSON 物件、值只收字串

- **背景**：T0004 讓 `POST /v1/messages` 接受 `template_id`＋`vars`；spine 與 `openapi.yaml` 都沒定 `vars` 的形狀。yolo Worker 以 Q-3 經 BAT 通道問塔台（2026-10-22T23:37:23+0800 答覆，塔台代決）。
- **選項**：
  - [A] `object`，`additionalProperties: {type: string}`；數字、布林、`null`、巢狀回 400 `invalid_request`，訊息不回顯內容
  - [B] 收字串、數字、布林，伺服器端轉成字串
- **決定**：選 A，理由：沒有隱性轉換（例如 `1.0` 轉成 `"1"`），呼叫端行為可預期；日後要放寬可相容地擴充，反過來收緊則是相容性破壞。證據（T0004 回報區〈互動紀錄〉）：DTO 用 `map[string]*string`，`null` 也擋下，固定訊息 `field 'vars' must be an object of string values`。
- **影響**：`docs/openapi.yaml` 的 `SendMessageRequest.vars` 為字串值物件；要接受其他型別須開新決策取代本條。
- **相關工單**：T0004

### D006 2026-10-22 — `ErrInvalidTemplate` 回 400 `invalid_template`

- **背景**：T0004 工單寫 `ErrInvalidTemplate` → 400 `invalid_request`，但 `docs/error-codes.md:14` 與 SPEC 的 error-codes 都定為 400 `invalid_template`（錯誤碼變更屬相容性變更）。Worker 以 Q-2 問塔台；塔台查證 `docs/error-codes.md` 後代決。
- **選項**：
  - [A] 照已文件化的契約回 400 `invalid_template`
  - [B] 照工單回 400 `invalid_request`，差異交塔台
- **決定**：選 A，理由：錯誤碼是對外契約，文件與 SPEC 一致；工單寫法是塔台開單筆誤。證據：T0004 `errors_templates.go` 的 `templateError` 對應 400 `invalid_template`，`httpapi_test.go` 驗信封；塔台收單重跑 `go test ./... -race` 全綠。
- **影響**：T0007 只加 `ErrMissingVariable` → 422，保留本條的 400 對應；開單時錯誤碼以 `docs/error-codes.md` 為準。
- **相關工單**：T0004、T0007

### D005 2026-10-22 — 模板 E2E 暫以 store 讀回的 `Body` 驗證已渲染

- **背景**：T0004 驗收條件寫「用 `FakeProvider` 收到的內容驗證」已渲染，但 `queue.Run` 目前只等 ctx（送信 worker 尚未實作），`testutil` 也沒把 `FakeProvider` 接進 queue，provider 收不到訊息。Worker 以 Q-1 問塔台。
- **選項**：
  - [A] 從 `env.Store.Get(testutil.TenantA, id)` 讀 `Body` 驗證，並斷言 `FakeProvider` 沒收到未渲染內容
  - [B] `GET /v1/messages/{id}` 回應加 `body` 欄位（API 契約變更、檔案不在 `affects_files`）
  - [C] 本單補 queue 送信 worker（超出範圍）
- **決定**：選 A，理由：不擴大範圍、不改 API 契約，仍以真實 handler＋`httptest` 驗到「入佇列前已渲染」（AD-16）。證據：`TestE2E_Template_RendersVariables` PASS（塔台收單重跑）。
- **影響**：工單驗收條件「用 FakeProvider 收到的內容驗證」未達成，屬塔台決定的差異；實作 queue 送信 worker 的工單須把這個 E2E 改成以 `FakeProvider` 收到的內容驗證。
- **相關工單**：T0004

### D004 2026-10-22 — httpapi 每功能一檔，用明確 stub 預留接口

- **背景**：第四章 T0005（限流）、T0006（列表）、T0007（模板缺變數 422）要 yolo 同時派出，`affects_files` 不能重疊。現況 `handler.go`（路由表、`Deps`、`v1` 串接）、`errors.go`（`writeError` 單一 switch）、`messages.go`（發送／查單筆／DTO）會被三張同時改。
- **選項**：
  - [A] 明確 stub：`NewHandler` 預先列好 `registerXxx`，`v1()` 預留 `rateLimit` 直通位置，`writeError` 詢問固定清單 `featureErrorMappers`；各功能填自己的檔。程式裡看得到路由、中介層順序、錯誤對應；代價是新增「全新功能」仍要改一次 `handler.go`／`errors.go`（本章已知的功能由 T9003 預先列好）
  - [B] 套件層級 registry＋各檔 `init()` 自行註冊：新增功能完全不改共用檔；但順序（authenticate → ratelimit）要靠 order 數字，註冊散在 `init()`，讀程式碼看不出路由表與錯誤對應，與 AD-8「`writeError` 單一對應處」的可讀性相悖
  - [C] 不重構，三張工單序列派：不撞檔，但放棄並行，也無法示範 yolo 同時派工
- **決定**：選 A，理由：本章要的功能清單已知，預留接口成本低；明確呼叫讓路由表與中介層順序可一眼讀出，AD-8 的 `writeError` 仍是信封唯一輸出處（mapper 只回 `apiError`，不寫回應）。B 的好處只在「未知的未來功能」，不抵可讀性成本。
- **影響**：T9003 只做結構調整、行為不變；T0005 填 `ratelimit.go`／`errors_ratelimit.go`／`deps.go`，T0006 新增 `messages_list.go`，T0007 填 `errors_templates.go`／`messages_send.go`，並行三單 `affects_files` 不重疊（`cmd/smsd/main.go`、`e2e/testutil/server.go` 只有 T0004 與 T0005 會改，T0004 先做完）。日後出現 stub 清單以外的新功能，才改 `handler.go`／`errors.go` 一次。
- **相關工單**：T9003

### D003 2026-10-22 — 路由維持標準庫 `http.ServeMux`，中介層用函式包裝（A1），不引入 chi

- **背景**：D001 寫了「標準庫真的不夠用時，再開 EXP 比較」。API 草案的路由數量增加到 7 組，觸發 EXP-ROUTER-001 比較標準庫 `http.ServeMux` 與第三方 router（chi）。
- **選項**：
  - [A1] 標準庫，單一 mux，`/v1` 每條路由各自用 `v1(...)` 包中介層；串接約 17 行
  - [A2] 標準庫，`/v1/` 子 mux 掛在頂層 mux，中介層只包一次；串接約 10 行，但未帶 key 的未知路徑與錯方法回 401，而非 404／405
  - [B] chi ＋ `r.Use`／`r.Group`；預期串接約 3–5 行，**未實測**（離線，本機無 chi 原始碼）；`go.mod` 會出現 `require`
- **決定**：選 A1，理由：7 條路由（`/healthz`、`/v1/messages`、`/v1/messages/{id}`、`/v1/messages?status=`、`/v1/templates`、`/v1/templates/{id}`、`/v1/providers/{name}/callbacks`）A 方案實測全部直接表達，沒有需要變通的路由；D001 的啟動條件（標準庫真的不夠用）不成立。A2 少約 7 行，但 `/v1` 下的 401 先於 404／405，與 AD-8 的信封行為不一致，不採。B 的優勢只在省 5–12 行串接，不抵外部依賴成本。證據：EXP-ROUTER-001 實驗紀錄（commit `8f6c58e`；A 實測 `go vet` 乾淨、`go test -count=1` PASS；B 未實測，結論不依賴 B 的數字）。
- **影響**：**不推翻 D001，ARCHITECTURE-SPINE AD-17 維持**；`go.mod` 仍不得有 `require`。404／405 必須走錯誤信封（`envelopeFallbacks`，AD-8）。中介層以 `func(http.Handler) http.Handler` 串接，可用小型 `chain` helper。`/healthz` 不包驗證與限流。日後出現多層版本前綴或每群組不同中介層組合再開新 EXP；要引入 router 須新決策取代本條。T9002 把結論回寫 spine。
- **相關工單**：EXP-ROUTER-001、T9002

### D002 2026-10-22 — API key 只從 header `X-API-Key` 讀，query 帶 key 視為沒帶

- **背景**：T0003 以驗證 middleware 取代 `withTenant` 佔位。key 可以從哪裡讀，決定憑證會不會進入存取日誌、瀏覽器歷史、代理伺服器快取與 `Referer`，也決定錯誤處理的行為（帶了無效 key 與沒帶 key 是否要區分）。工單開立時已把規則寫進硬性規則，由塔台收單時記錄。
- **選項**：
  - [A] 只讀 header `X-API-Key`；query 參數（`?api_key=…`）一律忽略，等同沒帶 → 401 `unauthorized`。key 不會出現在 URL，標準庫的 `r.URL` 日誌與 `httptest` 記錄都看不到它；代價是瀏覽器網址列或簡易 `curl` 連結不能直接帶 key
  - [B] header 與 query 都收（header 優先）：手動測試與 webhook 式連結方便；但 key 會進 URL，落到存取日誌、代理與瀏覽器歷史，違反 AD-15「日誌不記 key」的精神，且兩個來源衝突時要多定一條優先規則
  - [C] header 為主，query 帶 key 時回 400 並提示改用 header：對呼叫端更友善、錯誤更明確；但多一個錯誤碼與分支，且「有人把 key 放進 URL」這件事本身已經讓 key 外洩，回應再提醒也補救不了，並與 `docs/error-codes.md`／`openapi.yaml` 已定義的 401 契約不一致
- **決定**：選 A，理由：憑證不進 URL 是最便宜的防線，且與既有 401 契約完全一致，不新增錯誤碼。證據（T0003 回報區）：〈產出摘要〉記 `auth.go` 僅 `r.Header.Get("X-API-Key")`、query 參數一律忽略；〈驗收證據〉記「`auth.go` 僅 `r.Header.Get("X-API-Key")`；e2e 子測試『key in query string counts as missing』驗證 401」，並列 `go test ./e2e -run 'TestE2E_(Auth_MissingKey401|Tenant_IsolationReturns404)' -v` 兩個測試與全部子測試 PASS；`TestE2E_Auth_MissingKey401` 另涵蓋「不回顯 key」與「被拒不寫入 store」。塔台收單時重跑 `CGO_ENABLED=1 go test ./... -race`、`go vet`、`gofmt -l .` 皆乾淨。
- **影響**：之後所有 `/v1/*` 端點（限流 story 6、模板 story 5／8…）都經同一個 `authenticate` middleware，不得另開 query 取 key 的路徑；`docs/openapi.yaml` 與對外文件只描述 header；手動驗證要用 `curl -H 'X-API-Key: …'`。要改成接受 query，須開新決策取代本條，不改寫本條。
- **相關工單**：T0003

### D001 2026-10-22 — 只用標準庫 `net/http`＋`httptest`，不引入第三方套件

- **背景**：T0001 建 sms-platform 的第一個 Go module 與 `GET /healthz`。這是之後每一章長功能的地基：HTTP 路由與測試寫法在這裡定下來，後面的工單都會沿用。Go 1.22 起標準庫 `http.ServeMux` 已支援「方法＋路徑」（`GET /healthz`）與 `{id}` 路徑參數；`net/http/httptest` 可以在本機起真實的 HTTP server 跑 E2E，不需要網路或帳號。
- **選項**：
  - [A] 只用標準庫：`net/http` 路由、`httptest` 跑 E2E、`testing` 斷言。`go.mod` 沒有 `require`，clone 下來不必下載任何東西就能 `go test`；代價是路由群組、middleware 串接要自己多寫幾行
  - [B] 第三方 router（例如 chi）＋標準庫測試：路由群組與 middleware 現成；但多一個依賴要跟版本，課程錄影後套件改版，學員畫面就會和影片不一致
  - [C] 第三方 router＋斷言套件（例如 testify）：測試寫起來較短；依賴再多一個，學員要同時認兩套 API，偏離「第一章不教 Go 語法」的目標
- **決定**：選 A，理由：骨架只有一條路由，標準庫足夠；沒有依賴，「`go.mod` 無 `require`」就能直接寫成驗收條件（`go list -m all` 只列出 `example.com/sms-platform`），離線、在 dev container 裡都能重現。標準庫真的不夠用時，再開 EXP 比較「標準庫路由 vs 第三方 router」（3-06），結論另記新的決策，不改寫本條
- **影響**：`go.mod` 不得有 `require`；`internal/httpapi` 用 `http.NewServeMux` 註冊路由；`e2e/testutil` 用 `httptest.NewServer` 包真實 handler；之後的工單要加依賴，必須在工單明示並記一條新的決策
- **相關工單**：T0001
