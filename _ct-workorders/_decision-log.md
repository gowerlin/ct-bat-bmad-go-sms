# Decision Log

## 索引

| # | 日期 | 決策 | 相關工單 |
|---|------|------|---------|
| D001 | 2026-10-22 | 只用標準庫 `net/http`＋`httptest`，不引入第三方套件 | T0001 |

| D002 | 2026-10-22 | API key 只從 header `X-API-Key` 讀，query 帶 key 視為沒帶 | T0003 |
| D003 | 2026-10-22 | 路由維持標準庫 `http.ServeMux`，中介層用函式包裝＋`chain` helper（A1），不引入 chi；不推翻 D001 | EXP-ROUTER-001、T9002 |

---

## 詳細紀錄（倒序）

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
