---
schema_version: 1
schema_kind: experiment
id: EXP-ROUTER-001
title: 標準庫 http.ServeMux vs 第三方 router（chi）能否表達 API 草案全部路由
status: CONCLUDED
topic: ROUTER
created_at: "2026-10-22T19:10:17+08:00"
driving_decision: D001
success_criteria:
  - 列出 API 草案每一條路由（/healthz、/v1/messages、/v1/messages/{id}、/v1/messages?status=、/v1/templates、/v1/templates/{id}、/v1/providers/{name}/callbacks），方案 A、B 各自能否直接表達
  - 中介層（X-API-Key、限流）在方案 A、B 各要寫多少程式
  - 結論寫成可決策的一句話，並說明是否推翻 D001
---

# EXP-ROUTER-001-stdlib-mux-vs-chi

## 元資料
- **編號**：EXP-ROUTER-001
- **標題**：標準庫 http.ServeMux vs 第三方 router（chi）能否表達 API 草案全部路由
- **狀態**：CONCLUDED
- **TOPIC**：ROUTER
- **假設**：Go 1.22 起標準庫 `http.ServeMux` 支援「方法＋路徑」與 `{id}` 路徑參數，足以表達 API 草案的全部路由；中介層用函式包裝即可，不需要第三方 router
- **驅動決策**：D001（D001 寫了「標準庫真的不夠用時，再開 EXP 比較」，結論另記新決策，不改寫 D001）
- **建立時間**：2026-10-22T19:10:17+08:00
- **結論時間**：2026-10-22T19:14:02+08:00

## 實驗設計

### 方案
- **[A]** 標準庫 `http.NewServeMux` ＋ 函式包裝的中介層（`func(http.Handler) http.Handler`）
- **[B]** 第三方 router（以 chi 為例）＋ 它的 middleware 串接（`r.Use`、`r.Group`）

### 待比較的路由（API 草案）
| # | 路由 |
|---|------|
| 1 | `GET /healthz` |
| 2 | `POST /v1/messages` |
| 3 | `GET /v1/messages/{id}` |
| 4 | `GET /v1/messages?status=`（query 過濾，是否與 #2、#3 衝突） |
| 5 | `/v1/templates`（集合：建立、列表） |
| 6 | `/v1/templates/{id}`（單筆：讀取、更新、刪除） |
| 7 | `POST /v1/providers/{name}/callbacks` |

每條路由對兩個方案各記：能否直接表達（是／否／需變通）、寫法、`r.PathValue("id")` 與 chi 取參數的差異、不允許的方法時的 405 行為、不存在時的 404 是否能走既有錯誤信封 `{"error":{"code","message"}}`（`internal/httpapi` 的單一出口 `writeError`）。

### 中介層比較
| 中介層 | 範圍 | 行為依據 |
|---|---|---|
| `X-API-Key` 驗證 | `/v1/*`，不含 `/healthz` | D002（只讀 header；缺／無效回 401） |
| 限流 | `/v1/*`，每 API key | story 6（超限 429＋`Retry-After`） |

量測：兩個方案各自「掛上這兩個中介層」的新增／修改程式行數（不含測試、不含 `internal/ratelimit` 與 `internal/tenant` 本身的邏輯，只算串接）、路由群組（`/v1` 前綴一次套用）寫法、`/healthz` 排除寫法。

### 約束（務必遵守）
- 全程在 worktree 的獨立 sandbox 內做，**不改** 主線 `go.mod`、`internal/`、`cmd/`；程式放 `_bmad-output/tmp/exp-router/`（獨立 `go.mod`，AGENTS.md「暫存腳本寫在 `_bmad-output/tmp/`」）。
- **不連外網**（AGENTS.md Policy）。本機 Go module cache 查無 `github.com/*`，**拿不到 chi 原始碼**。方案 B 因此預設只能以 chi 公開 API 的文件知識做「紙上對照」，並在紀錄標明「B 未實測」；方案 A 必須實際編譯並用 `httptest` 跑過。若使用者另行提供離線的 chi 副本（`GOFLAGS=-mod=vendor` 或 `GOPROXY=file://…`），再補 B 的實測。
- 驗證腳本沒成功跑完前，不要在紀錄寫「通過」（AGENTS.md Known pitfalls）。
- 不用真實憑證與手機號碼。

### 預期觀察
- 假設成立的訊號：7 條路由方案 A 皆為「是」，A 的中介層串接行數與 B 同量級（差距不大於一個小 helper，例如 `chain(...)`）。
- 假設不成立的訊號：任何一條路由 A 需要變通（例如 #4 或 #5／#6 同路徑多方法的 405／404 行為無法套用錯誤信封），或中介層的路由群組／排除寫法讓 A 的重複程式明顯變多。

## 實驗紀錄
- **執行時間**：2026-10-22T19:13+08:00（紀錄寫入時取自系統時鐘）
- **環境**：go1.27.3 windows/amd64，離線；程式在 `_bmad-output/tmp/exp-router/`（獨立 `go.mod`，`router.go`＋`router_test.go`，被 `.gitignore` 排除，不進 git）。主線 `go.mod`、`internal/`、`cmd/` 未動。
- **驗證**：`go vet ./...` 乾淨；`go test -count=1 -v ./...` 輸出 `PASS` / `ok example.com/exp-router`（修正一次測試自身的期望字串：JSON map key 依字母排序，與路由行為無關）。

### 方案 A（標準庫 `http.ServeMux`）— 已實測
實作兩個變體：**A1** 單一 mux、每條 `/v1` 路由用 `v1(...)` 包中介層；**A2** 子 mux 掛在頂層 mux 的 `/v1/`，中介層只包一次。stub handler 回報看到的 `PathValue`／query；envelope 沿用 `internal/httpapi` 的 `envelopeFallbacks` 作法（複製進 sandbox）。

| # | 路由 | A 能否直接表達 | 實測 |
|---|------|---------------|------|
| 1 | `GET /healthz` | 是 | A1、A2 皆 200，不帶 key 也通過 |
| 2 | `POST /v1/messages` | 是 | A1、A2 皆 200 |
| 3 | `GET /v1/messages/{id}` | 是 | `r.PathValue("id")` = `m1` |
| 4 | `GET /v1/messages?status=` | 是 | query 不參與路由比對，與 #2、#3 不衝突；`?status=queued` 走列表 handler，`r.URL.Query()` 取得。需另註冊 `GET /v1/messages`（無 `{id}`） |
| 5 | `/v1/templates` | 是 | `POST`、`GET` 兩條各自註冊，皆 200 |
| 6 | `/v1/templates/{id}` | 是 | `GET`／`PUT`／`DELETE` 各一條，皆 200，`PathValue("id")` = `t1`（草案「更新」以 `PUT` 試，未涉及 `PATCH` 語意） |
| 7 | `POST /v1/providers/{name}/callbacks` | 是 | `PathValue("name")` = `mock` |

7 條皆「是」，沒有需要變通的路由。

**404／405 與錯誤信封**
- 標準庫預設的 404／405 是純文字；用 `envelopeFallbacks`（先 `mux.Handler(r)` 看 pattern 是否為空，再用 capture writer 取狀態）可改走信封。實測：`GET /nope` → 404 `not_found`；`POST /healthz` → 405 `invalid_request`，`Allow: GET, HEAD`；`PATCH /v1/templates/t1` → 405，`Allow: DELETE, GET, HEAD, PUT`。405 的 `Allow` 由 mux 自動算出。
- 這是約 25 行的變通 helper（主線已有）；不是一行設定。

**路由衝突偵測**（啟動時 panic）：`GET /v1/messages/{id}` ＋ `GET /v1/messages/search` 可並存（較特定者優先）；`GET /v1/messages/{id}` ＋ `GET /v1/{x}/m1` 註冊時 **PANIC**（歧義）；重複註冊同一 pattern **PANIC**。

**中介層**
- `X-API-Key` 驗證：缺／無效 → 401 信封，實測通過。限流（stub：每 key 固定額度）：第 3 次 → 429 ＋ `Retry-After: 1`，實測 A1、A2 皆 `[200 200 429]`；`/healthz` 連打 5 次不受限，皆 200。
- 串接行數（不含兩個中介層本體、不含測試）：
  - A1：`chain` helper 7 行 ＋ `v1 := …` 1 行 ＋ 9 條路由各改寫 1 處（`mux.Handle(…, v1(h))`）＝ 17 行。`/healthz` 排除寫法：該行不包 `v1(...)`（0 額外行）。
  - A2：`chain` helper 7 行 ＋ 子 mux／頂層 mux 的掛載 3 行（`top.Handle("/v1/", chain(envelopeFallbacks(sub), authenticate(keys), rateLimit(budget)))` 等）＝ 約 10 行，各路由行不用改。`/healthz` 排除寫法：註冊在頂層 mux、不經 `/v1/`。
- A2 觀察到的行為差異：未帶 key 的未知路徑，A1 回 **404**／405（路由先於驗證），A2 回 **401**（`/v1/` 子樹整個先驗證）。例：`GET /v1/nope` 無 key，A1 → 404、A2 → 401；`DELETE /v1/messages` 無 key，A1 → 405＋`Allow`、A2 → 401。A2 需要 `envelopeFallbacks` 套兩層（頂層與子 mux）。
- 其他：`GET /v1/messages/`（結尾斜線）A1、A2 皆 404 信封。

### 方案 B（chi）— **未實測**（紙上對照）
本機無 `github.com/*` 原始碼，不連外網，故只憑 chi 公開 API 文件知識，**未編譯、未跑測試**；下表為預期，不可當觀察數據。
| 項目 | B（chi）預期 |
|------|-------------|
| 7 條路由 | 預期皆可表達：`r.Get("/healthz", …)`、`r.Route("/v1", func(r chi.Router){ r.Use(auth, limit); … })`、`r.Get("/messages/{id}", …)`、`r.Route("/templates", …)` 等 |
| 取參數 | `chi.URLParam(r, "id")`（A：`r.PathValue("id")`） |
| 路由群組／排除 | `r.Group`／`r.Route` ＋ `r.Use` 一次套用；`/healthz` 放群組外。預期串接約 3–5 行，無 `chain` helper |
| 404／405 | 預期有 `r.NotFound(h)`、`r.MethodNotAllowed(h)` 可直接掛信封 handler（需實測確認 405 時 `Allow` 是否自動帶出） |
| 衝突偵測 | 未知 |
| 成本 | 新增外部依賴（違反 AGENTS.md「`go.mod` 只有標準庫」，需新決策與工單明示） |

### 對照 success_criteria 的原始數據
1. 7 條路由：A 皆「是」（實測）；B 未實測，預期皆「是」。
2. 中介層串接行數：A1 約 17 行、A2 約 10 行（實測計數）；B 預期約 3–5 行（未實測）。
3. 結論由塔台撰寫（本節不下結論）。

## 結論
**一句話決策**：路由繼續用標準庫 `http.ServeMux`（Go 1.22+ 方法＋`{id}`），中介層用 `func(http.Handler) http.Handler` 加一個約 7 行的 `chain` helper 串接，不引入 chi。

**是否推翻 D001：否。** D001 的條件是「標準庫真的不夠用」；實測 7 條路由 A 全部直接表達（成功標準 1），沒有需要變通的路由。唯一的額外成本是 404／405 要走錯誤信封，需要約 25 行 `envelopeFallbacks`，但主線已有，且 spine AD-8 早已要求攔截。結論記為 D003，AD-17 維持。

**對照成功標準**
1. 路由：A 7/7「是」（實測）；B 未實測，預期 7/7「是」，不影響決策。
2. 中介層串接：A1 約 17 行、A2 約 10 行（實測）；B 預期約 3–5 行（未實測）。差距約 5–12 行，低於引入外部依賴（`go.mod` 出現 `require`、離線不可得、版本漂移）的成本。
3. 本結論。

**A1 或 A2**：採 A1（單一 mux、`/v1` 路由各自包 `v1(...)`）。理由：A2 會讓未帶 key 的未知路徑與錯方法回 401 而非 404／405，與錯誤信封行為（AD-8）和 `/healthz` 免驗證的直覺不一致，省下的約 7 行不值得；A1 的 17 行可以用 `chain` 吸收。

**限制**：B 為紙上對照，其行數與 404／405、衝突偵測行為沒有實測；本結論不依賴 B 的任何數字——即使 B 的串接行數為 0，也不到推翻「無外部依賴」的程度。若日後路由數量或群組巢狀明顯增加（例如多層版本前綴、每群組不同中介層組合）再開新 EXP。

**建議下一步**：D003 記錄；T9002 把結論回寫 `ARCHITECTURE-SPINE.md`（AD-17 補路由寫法）；不需要新增依賴或改程式。

**清理**：程式在 worktree 的 `_bmad-output/tmp/exp-router/`（`.gitignore` 排除，未進 git）；worktree 移除後即消失，若要留作參考請在移除前自行備份。
