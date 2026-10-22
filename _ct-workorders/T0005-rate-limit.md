---
schema_version: 1
schema_kind: workorder
id: T0005
title: 每 API key 速率限制，超量 429＋Retry-After（story id "6"）
type: impl
status: DONE
repo: sms-platform
project: sms-platform
created_at: "2026-10-22T23:07:32+0800"
started_at: "2026-10-22T23:57:17+0800"
updated_at: "2026-10-23T00:01:29+0800"
completed_at: "2026-10-23T00:01:29+0800"
sizing: M
depends_on:
  - T0004
affects_files:
  - internal/ratelimit/ratelimit.go
  - internal/ratelimit/ratelimit_test.go
  - internal/httpapi/ratelimit.go
  - internal/httpapi/errors_ratelimit.go
  - internal/httpapi/deps.go
  - internal/tenant/tenant.go
  - internal/tenant/tenant_test.go
  - cmd/smsd/main.go
  - e2e/testutil/server.go
  - e2e/ratelimit_test.go
  - _ct-workorders/T0005-rate-limit.md
interaction:
  mode_hint: yolo
  interactive: false
  intervention_type: fire-and-forget
renew_count: 0
worker_question_timeout: 30
workdir: main repo
---
# 工單 T0005-rate-limit

## 元資料
- **工單編號**：T0005
- **任務名稱**：每 API key 速率限制（`stories.yaml` id "6"）
- **狀態**：DONE
- **類型**：impl
- **建立時間**：2026-10-22 23:07 (UTC+8)
- **intervention_type**：fire-and-forget（但有兩個「不得自行決定」的項目，見下，必須向塔台提問）
- **對應 E2E**：`TestE2E_RateLimit_Returns429WithRetryAfter`
- **depends_on**：T0004（T0004 會改 `deps.go`、`cmd/smsd/main.go`、`e2e/testutil/server.go`，須先收單）
- **affects_files**：見 frontmatter

## 任務指令

### 前置條件
- 讀 `AGENTS.md`、`_ct-workorders/_decision-log.md`（D002、D003、D004）
- 讀 `stories.yaml` id "6"，spine 的 AD-10、AD-12，`internal/ratelimit/doc.go`，`internal/httpapi/doc.go`，`internal/httpapi/ratelimit.go`（stub）
- 先跑 `make test`（Windows 需 `CGO_ENABLED=1`），確認開工前全綠

### 輸入上下文
T9003 在 `v1()` 預留了順序 authenticate → rateLimit → handler（無效 key 沒有桶，直接 401），本單把 `ratelimit.go` 的直通 stub 換成真實限流；429 的對應填 `errors_ratelimit.go` 的 `rateLimitError`（回 `apiError`，帶 `Retry-After` header 與信封 `rate_limited`，`docs/error-codes.md` 已有此列）。`Deps` 在 `deps.go` 加 `Limiter`，`cmd/smsd/main.go`、`e2e/testutil/server.go` 傳入。時間只經注入的 `Now`（AD-10），E2E 用 `FakeClock`，不睡真實時間。每 API key 一桶；限額由租戶帶入。

### 🔴 不得自行決定，必須問塔台（`tower_question`）
下面兩項**即使 AD-12 看起來已有公式，Worker 也不得自行採用或推導**，須依 `ct-exec` 與 `yolo-mode.md` § Worker 向塔台提問，在回報區〈待塔台決策〉寫成 Q-1、Q-2，並在 frontmatter 加 `tower_question: open`，commit 後送出提問，**等塔台答覆才實作這兩項**：

1. **桶容量與補充速率**：容量是多少、每秒補多少、限額的值從哪裡來（租戶欄位？全域預設？）、未設定時的預設
2. **`Retry-After` 怎麼取整**：從「等到下一枚 token 的秒數」到正整數秒，用哪種取整、最小值多少

等答覆期間，可以先做不受這兩項影響的部分（`apiError` 的 header 管線、`Deps` 欄位、組裝、E2E 骨架）；答覆之前不得寫死任何容量、速率或取整規則。塔台的答覆就是本單這兩項的規格。

### 預期產出
- `ratelimit` 套件（演算法、時間注入、`Retry-After` 計算）與單元測試
- `ratelimit.go`、`errors_ratelimit.go`、`deps.go`、組裝
- `e2e/ratelimit_test.go`：`TestE2E_RateLimit_Returns429WithRetryAfter`——在 `FakeClock` 下連續請求直到被拒、斷言 429、信封 `rate_limited`、`Retry-After` 為正整數；推進 `FakeClock` 後恢復放行；不同 API key 的桶互不影響；`/healthz` 與無效 key（401）不受限流影響

### 驗收條件
- [ ] `go test ./... -race` 全綠（貼最後 20 行輸出；`-race` 無法執行時寫明原因與替代環境）
- [ ] `go vet ./...` 沒有輸出
- [ ] `gofmt -l .` 沒有輸出
- [ ] `git diff --name-only <base>..HEAD` 只出現本工單 `affects_files` 列出的檔案（`<base>` = 本工單開工時的 HEAD；若同時有別張工單在同一個工作目錄 commit，改用 `git show --name-only <本工單的每個 commit>` 逐個核對，並在回報區註明）
- [ ] 回報區填好、`status` 與 body 鏡像一致；產品 commit 與工單 commit 訊息都含工單編號
- [ ] `go test ./e2e -run 'TestE2E_RateLimit_Returns429WithRetryAfter' -v` 通過
- [ ] 回報區〈待塔台決策〉有 Q-1、Q-2，且程式中的容量、補充速率、取整規則與塔台答覆一致
- [ ] 順序為 authenticate → rateLimit → handler；無效 key 回 401 而不是 429；測試不睡真實時間、不直接呼叫 `time.Now`

### 不做
- 不新增 `go.mod` 的 `require`（D001）
- 不改另一張工單的 `affects_files`；發現需要改，寫進回報區〈遭遇問題〉交回塔台
- 不改 `_ct-workorders/` 底下的塔台私有檔（`_tower-state.md`、`_decision-log.md`…）
- 不 push；不改 `_bmad/`、`.claude/`；不用 `t.Skip`、不刪測試、不放寬斷言
- 不改 `e2e/architecture_test.go`（若它因新檔失敗，先回報，不要放寬）
- 不改 `internal/httpapi/messages_*.go`、`templates.go`、`errors_templates.go`、`internal/store/*`、`docs/openapi.yaml`（429 已在 openapi 以 `$ref` 宣告）
- 不做租戶管理 API、設定檔載入、分散式限流

## Sub-session 執行指示

> 由塔台以 yolo 模式在 BAT 派工（`/ct-exec T0005`）。遇到上面兩項決定時走 `tower_question`，不要向使用者提問。
> 時間值一律用系統指令取得（`date "+%Y-%m-%dT%H:%M:%S%z"`），不要憑印象填。驗證指令成功跑完、看到輸出之後，才在回報區寫「通過」。暫存腳本寫在 `_bmad-output/tmp/`。

---

## 回報區

> 以下由 sub-session 填寫

### 待塔台決策

#### Q-1：限流桶的容量與補充速率是多少、限額的值從哪裡來、未設定時的預設是什麼？
- 提問時間：2026-10-22T23:58:07+0800
- 選項：[A] 照 AD-12：`tenant.Tenant` 加 `RateLimitPerMin int`；每個 API key 一桶，容量 = 所屬租戶 `RateLimitPerMin`，補充率 `RateLimitPerMin/60` token/秒（連續補充），桶初始為滿；`RateLimitPerMin <= 0`（未設定）用全域預設 60 / [B] 只用全域設定（`ratelimit` 建構時給每分鐘上限，預設 60），租戶不帶欄位 / [C] 其他：________
- Worker 建議：[A]，因為 AD-12、`docs/README.md` 資料模型（Tenant 有 `rate_limit` per-minute）與工單「限額由租戶帶入」一致；`internal/tenant/tenant.go` 已在 affects_files。附帶：同一租戶的兩把 key 各有一桶、各自用租戶的限額；E2E 在 `testutil` 把 TenantA 設成小限額（例如 3）以少送請求，TenantB 留空走預設 60
- 影響範圍：`internal/ratelimit/ratelimit.go`、`internal/tenant/tenant.go`、`internal/httpapi/ratelimit.go`、`e2e/testutil/server.go`、`e2e/ratelimit_test.go`
- 阻塞：是（演算法參數與限額來源要等答覆；先做 `apiError` header 管線、`Deps` 欄位、組裝、E2E 骨架）

#### Q-2：`Retry-After` 從「等到下一枚 token 的秒數」換成正整數秒，用哪種取整、最小值多少？
- 提問時間：2026-10-22T23:58:07+0800
- 選項：[A] 照 AD-12：`max(1, ceil(秒數))`，秒數 = `(1 - 目前 tokens) / 每秒補充率` / [B] `floor(秒數) + 1`（剛好整數秒時多等 1 秒）/ [C] 其他：________
- Worker 建議：[A]，因為 ceil 保證客戶端照 `Retry-After` 等完一定有 token，不會再被拒；`max(1, …)` 確保 header 為正整數（`openapi.yaml` 宣告 integer）
- 影響範圍：`internal/ratelimit/ratelimit.go` 的 Retry-After 計算與單元測試、`e2e/ratelimit_test.go` 的斷言
- 阻塞：是（Retry-After 計算要等答覆）

### 完成狀態
DONE

### 產出摘要
- **ratelimit**（`internal/ratelimit/ratelimit.go`）：token bucket，每 API key 一桶（D009／Q-1）。`New(now, limitFor)`：時間只經注入的 `now`（AD-10），限額由注入的 `limitFor(tenantID)` 取得（套件不擁有限額存放處）；`limitFor` 回 `<= 0` 用 `DefaultPerMinute = 60`。容量 = 限額，補充率 = 限額/60 token/秒、連續補充，新桶初始為滿；限額調低時桶立刻縮到新容量。被拒回 `*LimitedError{RetryAfter}`（`errors.Is(err, ErrLimited)` 為真）；`RetryAfterSeconds(wait) = max(1, ceil(秒數))`（D010／Q-2），等待秒數 = `(1 - tokens) × 60 / 限額`。加鎖，`-race` 無 data race
- **tenant**（`internal/tenant/tenant.go`）：`Tenant.RateLimitPerMin int`（`<= 0` = 未設定）；`Registry.RateLimitPerMin(tenantID) int`（未知租戶回 0），簽章即 `ratelimit.New` 要的 `limitFor`
- **httpapi**：`ratelimit.go` 以 `Limiter` 介面（`Allow(tenantID, key string) error`，與 `Authenticator` 放 `auth.go` 同寫法）取代直通 stub；桶的 key 是 `tenant.HashKey(X-API-Key)`，不保存明文 key。順序沿用 `v1()`：authenticate → rateLimit → handler，無效 key 在 authenticate 就 401、不建桶。`errors_ratelimit.go` 的 `rateLimitError`：`*ratelimit.LimitedError` → 429 `rate_limited`、message `rate limit exceeded, retry later`、header `Retry-After: <整數秒>`（經 `apiError.Header`，信封仍只由 `writeError` 寫，AD-8）。`deps.go` 加 `Deps.Limiter`
- **組裝**：`cmd/smsd/main.go` `Limiter: ratelimit.New(time.Now, tenants.RateLimitPerMin)`；`e2e/testutil/server.go` `ratelimit.New(clock.Now, reg.RateLimitPerMin)`，TenantA／TenantB 設 `TestRateLimitPerMin = 1_000_000`（見〈遭遇問題〉2）
- **測試**：`ratelimit_test.go`（滿桶後拒絕且 Retry-After 20、連續補充 5s 後 15、補充封頂、未設定／負數用 60、每 key 獨立桶、調低限額、`RetryAfterSeconds` 取整表、200 goroutine 並行只放行 50）；`tenant_test.go`（`RateLimitPerMin` 讀取與替換）；`e2e/ratelimit_test.go` `TestE2E_RateLimit_Returns429WithRetryAfter`（TenantA 限額 3：3 次 200 → 429 `rate_limited`、`Retry-After` 20；被拒的 `POST /v1/messages` 不建立訊息；KeyB 與同租戶第二把 key 不受影響；`/healthz` 不限流；無效／缺 key 連續請求都 401 不是 429；`FakeClock` 推進 10s 仍 429 且 `Retry-After` 10，再推進 10s 放行一次後又 429）

### 驗收證據
- 環境：Windows 主機，Go 1.27.3，MinGW gcc（`C:\ProgramData\mingw64\mingw64\bin` 加進 PATH），`CGO_ENABLED=1`
- 開工前 `make test`（HEAD `24535d9`）：exit 0，全綠
- `go test ./... -race -count=1`（工作樹 = `0776289` 的內容，當時 T0006 `a5ab662`、T0007 `5c6bd1f` 已 commit，工作樹除本單檔案外乾淨；最後 20 行，exit 0）：
  ```
  ?   	example.com/sms-platform/cmd/smsd	[no test files]
  ok  	example.com/sms-platform/e2e	1.351s
  ?   	example.com/sms-platform/e2e/testutil	[no test files]
  ok  	example.com/sms-platform/internal/httpapi	1.054s
  ?   	example.com/sms-platform/internal/provider	[no test files]
  ok  	example.com/sms-platform/internal/provider/mock	1.068s
  ok  	example.com/sms-platform/internal/queue	1.040s
  ok  	example.com/sms-platform/internal/ratelimit	1.040s
  ok  	example.com/sms-platform/internal/store	1.040s
  ok  	example.com/sms-platform/internal/template	1.040s
  ok  	example.com/sms-platform/internal/tenant	1.040s
  ?   	example.com/sms-platform/internal/webhook	[no test files]
  ```
- `go test ./e2e -run 'TestE2E_RateLimit_Returns429WithRetryAfter' -v`：`--- PASS: TestE2E_RateLimit_Returns429WithRetryAfter`，4 個子測試都 PASS，`ok example.com/sms-platform/e2e`
- `go build ./...`、`go vet ./...`、`gofmt -l .`：都沒有輸出
- 變更檔案核對：本單執行期間 T0006、T0007 與塔台在同一工作目錄 commit，所以改用 `git show --name-only` 逐個核對本單 commit：`f27cee9`（只有 `_ct-workorders/T0005-rate-limit.md`）、`0776289`（`cmd/smsd/main.go`、`e2e/ratelimit_test.go`、`e2e/testutil/server.go`、`internal/httpapi/deps.go`、`internal/httpapi/errors_ratelimit.go`、`internal/httpapi/ratelimit.go`、`internal/ratelimit/ratelimit.go`、`internal/ratelimit/ratelimit_test.go`、`internal/tenant/tenant.go`、`internal/tenant/tenant_test.go`），都在 `affects_files`；收尾 commit 只有本工單檔
- 與塔台答覆一致：容量 = 租戶 `RateLimitPerMin`、補充 `/60` 每秒、未設定（`<= 0`）60、初始為滿（Q-1／D009）；`Retry-After = max(1, ceil(秒數))`（Q-2／D010），單元測試與 E2E 驗 20、15、10、1 等值
- 順序與時間：`v1()` 未改（authenticate → rateLimit → handler），E2E 驗無效 key 401；測試只用 `FakeClock`／測試內假時鐘，`e2e/architecture_test.go` 的 `time.Now(`、`time.Sleep(` 檢查通過（全套中的 `TestArchitecture_*`），`ratelimit` 不 import 任何 internal 套件

### 落地檢查
- 結果：PASS
- C-0：frontmatter `repo` = `sms-platform`，`basename(REPO_ROOT)` = `sms-platform` → PASS（`REPO_ROOT` = `<demo>/sms-platform`）
- C-1：工單在 `REPO_ROOT/_ct-workorders/` 下 → PASS
- C-3：前 5 個 `affects_files` 都在（或祖先在）→ PASS（資訊性）
- C-2：工單沒有 `branch` 欄位，不適用；目前分支 `main`
- `CT_HOME` 未設（非 worktree）；`CT_MODE=yolo`、`CT_INTERACTIVE=0`；`BAT_WORKSPACE_ID` = `<workspace>`（僅記錄）

### 產品 commit
- `0776289` feat(T0005): 每 API key token bucket 限流，超量 429 rate_limited＋Retry-After
- 工單 commit：`f27cee9`（開工＋提問）；收尾 commit 見 git log（訊息含 T0005）

### 互動紀錄
- [23:59] Q-1: 限流桶容量、補充速率、限額來源與未設定預設 → A（塔台，BAT 通道，choice 1）: [A] 每個 API key 一桶；容量 = 所屬租戶 `rate_limit_per_min`（Go 欄位 `tenant.Tenant.RateLimitPerMin`），每秒補 `rate_limit_per_min/60`（連續補充），桶初始為滿；上限放租戶設定，未設定（`<= 0`）預設 60（使用者決定）→ Action: `tenant.Tenant.RateLimitPerMin`＋`Registry.RateLimitPerMin`，`ratelimit.Limiter` 照此實作，`DefaultPerMinute = 60`
- [23:59] Q-2: `Retry-After` 取整與最小值 → A（塔台，BAT 通道，choice 1）: [A] `max(1, ceil(秒數))`，秒數 = `(1 - 目前 tokens) / 每秒補充率`（使用者決定）→ Action: `ratelimit.RetryAfterSeconds`，單元測試含取整邊界表

### Renew 歷程
無

### 遭遇問題
1. **`Deps.Limiter` 為 nil 時不限流（沒有做成啟動即失敗）**：必填檢查放在 `NewHandler`（`handler.go`）或 `rateLimit` 註冊時 panic 都有問題——前者不在 affects_files；後者會讓 `httpapi_test.go` 的 `TestNewHandlerRequiresTemplates`（不在 affects_files）因 Limiter 先 panic 而通過，等於讓它失去檢查 Templates 的意義。所以 nil = 關閉限流，`deps.go` 註解寫明，`cmd/smsd` 與 `testutil` 都有設。**建議**：下一張會動 `handler.go`／`httpapi_test.go` 的工單把 `Limiter` 併入 `NewHandler` 的必填檢查並補一個測試。
2. **testutil 的測試租戶限額設為 1,000,000**：預設 60 會讓既有的 `TestE2E_SendMessage_QueueFullReturns503AndKeepsNothing`（同一把 KeyA、時鐘不動、送 `queue.Capacity`+1 = 1001 次）變成 429。這是測試夾具的值，不是產品規則；產品預設 60 由 `ratelimit` 單元測試 `TestUnsetLimitUsesDefault` 驗證。限流 E2E 以 `env.Tenants.AddTenant` 把 TenantA 換成限額 3。之後新增的 E2E 若需要預設限額的行為，自行新增未設限額的租戶。
3. **範圍外、沒改**：`internal/ratelimit/doc.go` 仍寫「token bucket vs fixed window is the T0005 tower_question」與「tenant.rate_limit」，現已定為 token bucket（AD-12／D009）、欄位是 `RateLimitPerMin`；該檔不在 affects_files。`docs/error-codes.md` 429 列「本版已實作」仍為「否」，也不在 affects_files。
4. **桶不回收**：桶以 key hash 保存在記憶體，只會對通過驗證的 key 建立，數量以已發的 key 為上限；停用 key 的桶不清除。目前 in-memory 單機，影響可忽略；分散式限流本單不做。
5. **並行工單**：本單執行期間 T0006、T0007 與塔台在同一工作目錄 commit；本單 commit 都用 `git commit --only` 指定檔案，未動到他人檔案。

### sprint-status.yaml 已更新
不適用

### 回報時間
2026-10-23T00:01:29+0800
