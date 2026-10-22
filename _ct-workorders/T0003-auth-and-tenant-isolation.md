---
schema_version: 1
schema_kind: workorder
id: T0003
title: API key 驗證與租戶隔離（story 3＋4）
type: impl
status: DONE
repo: sms-platform
project: sms-platform
created_at: "2026-10-22T18:55:16+0800"
updated_at: "2026-10-22T19:06:06+0800"
started_at: "2026-10-22T19:04:04+0800"
completed_at: "2026-10-22T19:06:06+0800"
sizing: M
affects_files:
  - internal/tenant/tenant.go
  - internal/tenant/tenant_test.go
  - internal/httpapi/handler.go
  - internal/httpapi/auth.go
  - internal/httpapi/errors.go
  - internal/httpapi/httpapi_test.go
  - cmd/smsd/main.go
  - e2e/testutil/server.go
  - e2e/messages_test.go
  - e2e/auth_test.go
  - e2e/tenant_isolation_test.go
  - docs/error-codes.md
  - _ct-workorders/T0003-auth-and-tenant-isolation.md
interaction:
  mode_hint: off
  interactive: false
  intervention_type: fire-and-forget
renew_count: 0
workdir: main repo
---
# 工單 T0003-auth-and-tenant-isolation

## 元資料
- **工單編號**：T0003
- **任務名稱**：API key 驗證與租戶隔離（`stories.yaml` id "3"＋"4"）
- **狀態**：DONE
- **類型**：impl
- **建立時間**：2026-10-22 18:55 (UTC+8)
- **開始時間**：2026-10-22 19:04 (UTC+8)
- **完成時間**：2026-10-22 19:06 (UTC+8)
- **intervention_type**：fire-and-forget
- **對應 E2E**：`TestE2E_Auth_MissingKey401`、`TestE2E_Tenant_IsolationReturns404`（以 e2e 測試名對應 story；epics 編號不同，不要用）
- **affects_files**：
  - `internal/tenant/tenant.go`、`internal/tenant/tenant_test.go`
  - `internal/httpapi/handler.go`、`internal/httpapi/auth.go`、`internal/httpapi/errors.go`、`internal/httpapi/httpapi_test.go`
  - `cmd/smsd/main.go`
  - `e2e/testutil/server.go`、`e2e/messages_test.go`、`e2e/auth_test.go`、`e2e/tenant_isolation_test.go`
  - `docs/error-codes.md`
  - `_ct-workorders/T0003-auth-and-tenant-isolation.md`
- **與 T0002 的關係**：兩單 `affects_files` 不重疊，可各自獨立完成，無先後相依

## 任務指令

### 前置條件
- 讀 `AGENTS.md`、`_ct-workorders/_decision-log.md`（D001）
- 讀 `_bmad-output/specs/spec-sms-platform/stories.yaml` id "3"、"4"，`SPEC.md` CAP-9、CAP-10，`docs/error-codes.md`、`docs/openapi.yaml`（401 已在 openapi 定義，不用改）
- 讀 `internal/httpapi/handler.go`（`withTenant` 是 Epic 2 認證 middleware 的佔位）、`internal/store/store.go`、`internal/tenant/doc.go`、`e2e/architecture_test.go`
- 本機 `go version` 須為 go1.27.3，否則回報 BLOCKED

### 輸入上下文
目前所有 `/v1/*` 路由都由 `withTenant` 固定塞入 `DefaultTenantID`，沒有驗證。這張單做兩件事：

1. **驗證（story 3）**：以 middleware 取代 `withTenant`。從 header `X-API-Key` 取 key，雜湊（SHA-256 hex）後查 `internal/tenant` 的 registry。缺、無效、停用都回 `401`、code `unauthorized`，走既有 `writeError`／信封（見 `docs/error-codes.md`）。`/healthz` 不驗證。查到的 tenant_id 放進 request context（`tenantID(ctx)` 的來源只有這裡）。
2. **隔離（story 4）**：租戶 A 用自己的 key 查租戶 B 的訊息 id → `404` `not_found`（不是 403）。`store.Memory.Get(tenantID, id)` 已對跨租戶回 `ErrNotFound`，重點是 E2E 證明整條路徑（兩把 key、兩個租戶）成立，且 `httpapi` 不碰 store 的系統級介面（AD-4）。

硬性規則：
- **API key 只從 header `X-API-Key` 讀取；query 參數（例如 `?api_key=…`）帶 key 一律視為沒帶**（回 401）。這個選擇由塔台收單時記成 D002，你不用寫決策日誌
- key 只存 SHA-256 hex；明文不得進日誌、錯誤訊息、回應；比對用 `crypto/subtle.ConstantTimeCompare`（或等價）
- `internal/tenant` 只能 import `store`（`e2e/architecture_test.go` 的 import 圖）；`httpapi` 可 import `tenant`
- 租戶實體含 `callback_url` 欄位（Webhook URL 的固定名稱）；本單不使用它，但結構要有；key 要有「停用」狀態
- registry 本單是記憶體內實作，由組裝處（`cmd/smsd/main.go`、`e2e/testutil/server.go`）建立後經 `httpapi.Deps` 傳入；`NewHandler` 缺 registry 時沿用現有風格 panic
- **`cmd/smsd/main.go` 的 key 來源**：設定檔載入屬後續 story，本單不做。main 先組裝一個空的 registry（行為：所有 `/v1/*` 回 401，直到設定檔載入功能上線），並在程式註解與回報區〈產出摘要〉標明這是暫時狀態；不要自創環境變數（環境變數清單是固定的）
- `e2e/testutil/server.go`：註冊兩個測試租戶（A、B）與各自的假 key（例如 `test-key-tenant-a`，不得是真實格式的憑證），把 key 暴露在 `Env` 上讓測試取用；`NewTestServer` 簽名保持相容
- 因為 `/v1/*` 現在要 key，`e2e/messages_test.go` 與 `internal/httpapi/httpapi_test.go` 既有測試要改成帶 key——**只加 key，不得改寬斷言、不得刪測試**
- `docs/error-codes.md` 的 401 列「本版已實作」改為「是」
- 日誌只記識別欄位，不記 key、號碼、內容（AD-15）

### 預期產出
- `internal/tenant/tenant.go`：Tenant 實體、key hash、registry 查詢（含停用）；`tenant_test.go` 單元測試
- `internal/httpapi/auth.go`：驗證 middleware；`handler.go`、`errors.go` 對應修改（`unauthorized` → 401）
- `e2e/auth_test.go`：`TestE2E_Auth_MissingKey401`，涵蓋缺 key、無效 key、停用 key、query 參數帶 key（皆 401 且是錯誤信封）、有效 key 通過、`/healthz` 不需 key
- `e2e/tenant_isolation_test.go`：`TestE2E_Tenant_IsolationReturns404`，A 建立訊息，B 查詢回 404 `not_found`，A 自己查仍 200

### 驗收條件（共用）
- [ ] `go test ./... -race` 全綠（貼最後 20 行輸出；`-race` 無法執行時寫明原因與替代環境）
- [ ] `go vet ./...` 沒有輸出
- [ ] `gofmt -l .` 沒有輸出
- [ ] `git diff --name-only <base>..HEAD` 只出現本工單 `affects_files` 列出的檔案（`<base>` = 本工單開工時的 HEAD；另一張工單可能同時在同一個工作目錄 commit，若 diff 出現對方的檔案，改用 `git show --name-only <本工單的每個 commit>` 逐個核對，並在回報區註明）
- [ ] 回報區填好、`status` 與 body 鏡像一致；產品 commit 與工單 commit 訊息都含工單編號

### T0003 專屬條件
- [ ] `go test ./e2e -run 'TestE2E_(Auth_MissingKey401|Tenant_IsolationReturns404)' -v` 通過
- [ ] API key 只從 header `X-API-Key` 讀取；query 參數帶 key 一律視為沒帶（這個選擇收單時記成 D002）

### 不做
- 不新增 `go.mod` 的 `require`（D001；要加依賴須在工單明示並記新決策）
- 不改另一張工單的 `affects_files`；發現需要改，寫進回報區〈遭遇問題〉交回塔台
- 不改 `_ct-workorders/` 底下的塔台私有檔（`_tower-state.md`、`_decision-log.md`…）
- 不 push；不改 `_bmad/`、`.claude/`；不用 `t.Skip`、不刪測試、不放寬斷言
- 不做設定檔載入、租戶／key 管理 API、限流（story 6）、模板（story 5、8）、Webhook 簽章
- 不改 `internal/provider/*`、`e2e/mock_fault_test.go`（屬 T0002）；不改 `docs/openapi.yaml`

## Sub-session 執行指示

> 在第二個 Claude Code 終端（同一個 `sms-platform/`）執行 `/ct-exec T0003`。
> Worker 先做落點檢查（C-0 比對 `repo`），再把 `status` 改成 `IN_PROGRESS`、填 `started_at`，最後依序：產品 commit → 回報區 → 狀態與完成時間 → 工單 commit。
> 時間值一律用系統指令取得，不要憑印象填。

---

## 回報區

> 以下由 Worker 填寫，塔台不在這裡編輯

### 完成狀態
DONE

### 產出摘要
- 落點檢查：C-0 PASS（frontmatter `repo` = `sms-platform`，`basename(REPO_ROOT)` = `sms-platform`）；C-1 PASS；C-3 PASS（`affects_files` 既有路徑存在）；C-2 無 `branch` 欄位，略過。非 worktree（`CT_HOME` 未設）；未偵測到 `BAT_*` 環境變數，`CT_MODE` 未傳入，依 `ask` 處理。
- `internal/tenant/tenant.go`：`Tenant{ID, CallbackURL}`、`HashKey`（SHA-256 hex）、記憶體 `Registry`（`AddTenant`／`AddKey`／`DisableKey`／`Authenticate`）。只存雜湊，比對用 `subtle.ConstantTimeCompare`（掃過全部 key，不用 map 查找）；空、未知、停用皆回同一個 `ErrUnauthorized`。
- `internal/httpapi/auth.go`：`Authenticator` 介面、`authenticate` middleware（取代 `withTenant`），只讀 header `X-API-Key`，query 參數一律忽略（視為沒帶，D002）；`tenantID(ctx)` 的唯一來源。`/healthz` 不經 middleware。`handler.go`：`Deps.Tenants` 必填（缺則 panic），移除 `DefaultTenantID`；`errors.go`：`errUnauthorized` → 401 `unauthorized`，訊息固定、不回顯 key。
- `cmd/smsd/main.go`：**暫時狀態**——組裝空的 `tenant.NewRegistry()`，所以在設定檔載入（後續 story）上線前，所有 `/v1/*` 都回 401；程式內已加註解標明。未新增環境變數。
- `e2e/testutil/server.go`：註冊租戶 A、B 與假 key（`testutil.KeyA`／`KeyB`＝`test-key-tenant-a`／`-b`），`Env.Tenants` 暴露 registry；`NewTestServer` 簽名不變；`Env.Get` 預設帶 `KeyA`。
- 既有測試只加 key：`e2e/messages_test.go` 的 `send` 與兩處 `env.Do` 加 `testutil.KeyA`，斷言未動、未刪測試。`httpapi_test.go` 只新增 401 信封單元測試與 panic 訊息文字。
- `docs/error-codes.md`：401 列「本版已實作」改為「是」。
- 新測試：`TestE2E_Auth_MissingKey401`（缺 key、無效、停用、query 帶 key、不回顯 key、被拒不寫入 store、有效通過、`/healthz` 免 key）、`TestE2E_Tenant_IsolationReturns404`（A 建、B 查 404 `not_found` 且不含 id、A 自查 200）。`httpapi` 只用 `store.TenantScoped`，未碰系統級介面（AD-4）。
- 產品 commit：`4302695f7250d48c3ca76da73d157f592067bfa9`（`feat(auth): T0003 …`）。開工 base：`1210da3`。

### 驗收證據
- `go version`：go1.27.3 windows/amd64
- `CGO_ENABLED=1 go test ./... -race`（最後 20 行；全綠）：
```
?   	example.com/sms-platform/cmd/smsd	[no test files]
ok  	example.com/sms-platform/e2e	1.333s
?   	example.com/sms-platform/e2e/testutil	[no test files]
ok  	example.com/sms-platform/internal/httpapi	(cached)
?   	example.com/sms-platform/internal/provider	[no test files]
ok  	example.com/sms-platform/internal/provider/mock	(cached)
ok  	example.com/sms-platform/internal/queue	(cached)
?   	example.com/sms-platform/internal/ratelimit	[no test files]
ok  	example.com/sms-platform/internal/store	(cached)
?   	example.com/sms-platform/internal/template	[no test files]
ok  	example.com/sms-platform/internal/tenant	(cached)
?   	example.com/sms-platform/internal/webhook	[no test files]
```
- `go build ./...`、`go vet ./...`：無輸出；`gofmt -l .`：無輸出
- `go test ./e2e -run 'TestE2E_(Auth_MissingKey401|Tenant_IsolationReturns404)' -v`：兩個測試與全部子測試 PASS
- `git diff --name-only 1210da3..HEAD`：僅 `cmd/smsd/main.go`、`docs/error-codes.md`、`e2e/auth_test.go`、`e2e/messages_test.go`、`e2e/tenant_isolation_test.go`、`e2e/testutil/server.go`、`internal/httpapi/{auth,errors,handler,httpapi_test}.go`、`internal/tenant/{tenant,tenant_test}.go`，皆在 `affects_files` 內（無對方工單檔案）
- 只從 header 讀 key：`auth.go` 僅 `r.Header.Get("X-API-Key")`；e2e 子測試「key in query string counts as missing」驗證 401
- 無 `go.mod` 變動、無 `t.Skip`、無放寬斷言

### 互動紀錄
無（fire-and-forget）。

### 遭遇問題
- 無阻礙。備註：`/nope` 與方法不符的未路由請求仍由 `envelopeFallbacks` 回 404／405，不經驗證（與既有行為一致），故 `TestE2E_Errors_UnroutedRequestsUseEnvelope` 的帶 key 只是保持原請求形狀。
- 備註：`Env.Get` 預設帶 `KeyA` 是為了讓既有測試只加 key 而不改簽章；要以其他身分請用 `Env.Do`。

### 回報時間
2026-10-22T19:06:06+0800
