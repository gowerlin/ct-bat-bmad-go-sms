---
title: 'Story 1.1：送出訊息並可查詢'
type: 'feature'
created: '2026-10-22'
status: 'done'
baseline_commit: 'cd7649a34267f5e79dc78cba5abcae9004a36b60'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
  - '{project-root}/_bmad-output/planning-artifacts/architecture/architecture-sms-platform-2026-10-22/ARCHITECTURE-SPINE.md'
  - '{project-root}/_bmad-output/specs/spec-sms-platform/error-codes.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** 現有骨架只有 `GET /healthz`；租戶無法送出訊息或查詢狀態，也沒有 `store`、`queue`、錯誤信封與架構測試。

**Approach:** 補齊 Epic 1 的骨架：`store`（in-memory、狀態轉移表、`TenantScoped`／`System`）、`queue.Queue` 的 `Submit`（預留→寫 store→放入）、`httpapi` 的 `POST /v1/messages`、`GET /v1/messages/{id}`、單一 `writeError`，並以架構測試與 E2E 把關。以固定預設租戶運作，無 `X-API-Key`。

## Boundaries & Constraints

**Always:** 只用標準庫；時間與等待只經注入的 `Now`、`Wait`；ID 用 `crypto/rand`（`msg_` ＋128 位元 hex）；`httpapi` 不得 import `provider/mock`，`Deps` 不對 `Provider` 設預設值；`store` 內部加鎖、`-race` 無 data race；`queue` 與 `webhook` 不互相 import；`cmd` 只 import `internal/*`；驗證（`^\+[1-9]\d{1,14}$`、內容 ≤70 code point）集中單一函式；日誌只用 `slog` JSON 與 AD-15 白名單欄位；工單只改本 spec 列出的檔案。

**Never:** 不做背景送出／worker／重試（Story 1.6）；不做完整 `mock` 故障注入（1.2）；不做列表、模板、回呼、Webhook、設定檔載入、API key、限流；不新增外部依賴；不用 `t.Skip` 或放寬斷言；不動 `_bmad/`。

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| 送出成功 | `POST /v1/messages` `{"to":"+886900000001","body":"hi"}` | 202，`{"id":"msg_…","status":"queued"}` | N/A |
| 欄位缺漏／號碼非 E.164／內容 >70 code point | 同上但違規 | 400 `invalid_request`，不建立訊息 | 無效 JSON 亦 400 |
| 佇列滿載 | 容量用盡 | 503 `queue_full`，不留訊息 | 預留名額撤銷 |
| 查詢存在 | `GET /v1/messages/{id}` | 200，含 `id`、`status`、整數 `attempts` | N/A |
| 查詢不存在／重啟後／過期 | 未知 id | 404 `not_found` | N/A |
| 未知路徑／方法不符 | `GET /nope`、`DELETE /v1/messages` | 404 `not_found`／405 `invalid_request` 並帶 `Allow` | 一律信封 |
| 內部錯誤 | handler 回未知 error | 500 `internal_error`，message 為固定字串 | 不洩漏內部資訊 |
| 清除過期 | 終態且 `now` 前進 >24h，呼叫 `PurgeExpired(now)` | 該訊息 GET 404；未滿 24h 或非終態者保留 | N/A |

</frozen-after-approval>

## Code Map

- `internal/store/` -- 新增：`Message`、狀態常數、單一轉移表、`Patch{Attempts,ProviderRef,FailureReason}`、CAS `Transition`、`ErrNotFound`／`ErrIllegalTransition`、`TenantScoped`（首參 tenantID，空字串回錯誤）與 `System` 介面、in-memory 實作（注入 `Now`）、`PurgeExpired(now)`；葉節點，不 import 其他內部套件。
- `internal/queue/` -- 新增：`Queue{Submit,Run,Close}`、`NewInProcess`、`ErrQueueFull`、容量 1000、`Submit` 預留→寫 store→放入、失敗撤銷；`Run` 本 Story 只阻塞至 ctx 結束（worker 在 1.6）。
- `internal/provider/provider.go` -- 補 `ErrTransient`／`ErrPermanent`／`ErrUnauthorized` sentinel；`Reason` 欄位留 1.2／1.8。
- `internal/provider/mock/mock.go` -- 已滿足三方法、`Send` 固定成功；本 Story 不改。
- `internal/httpapi/handler.go` -- 改：`Deps` 移除 `Provider` 預設與 `mock` import，改持 `Queue`、`store.TenantScoped`、`Now`；加路由。新增 `errors.go`（`writeError`、`ServeMux` 404／405 攔截）、`messages.go`、`validate.go`。
- `cmd/smsd/main.go` -- 改：組裝 store、queue、mock；讀 `LISTEN_ADDR`（預設 `:8080`）；定時呼叫 `PurgeExpired`；優雅關機順序停 listener→`queue.Close`（最多 10 秒）。
- `e2e/testutil/{clock,server}.go` -- 改：`FakeClock` 補 `Wait(ctx,d)`（前進時鐘，不睡）；`NewTestServer` 改組裝新 `Deps`。
- `e2e/{architecture_test,messages_test}.go` -- 新增：架構測試與 `TestE2E_SendMessage_Returns202AndQueryable` 及錯誤路徑、重啟、清除案例。
- `docs/{openapi.yaml,README.md}`、`docs/error-codes.md`、`docs/onboarding.md` -- 改／新增：涵蓋本 Story 端點與 schema、錯誤碼清單（變更＝相容性變更）、租戶接入文件含 FR-28 重啟限制。

## Tasks & Acceptance

**Execution:**
- [x] `internal/store/*.go`、`*_test.go` -- 實作 store 與轉移表、`PurgeExpired`、`-race` 單元測試 -- AD-4、AD-5、AD-14
- [x] `internal/queue/*.go`、`*_test.go` -- `Submit` 單一操作、滿載撤銷 -- AD-2
- [x] `internal/provider/provider.go` -- 補 sentinel error -- AD-1
- [x] `internal/httpapi/*.go`、`*_test.go` -- 路由、驗證函式、`writeError`、404／405 攔截 -- AD-8
- [x] `cmd/smsd/main.go` -- 組裝根、`LISTEN_ADDR`、purge 迴圈、關機順序 -- AD-18
- [x] `e2e/testutil/*.go`、`e2e/*_test.go` -- `FakeClock.Wait`、E2E、架構測試（import 圖、`http.Client{`／`http.DefaultClient`／`time.Now(`／`time.Sleep(` 位置、`go.mod` 無 `require`）-- AD-19
- [x] `docs/*` -- OpenAPI、錯誤碼清單、接入文件 -- 文件 AC

**Acceptance Criteria:**
- Given 專案骨架，when `make test`，then 全綠（含架構測試）、`go build ./...` 與 `go vet ./...` 乾淨
- Given 任何 4xx／5xx，when 回應，then body 為信封，405 帶 `Allow`，500 message 固定
- Given `/healthz`，when GET，then 200 `{"status":"ok"}`（沿用既有 `TestE2E_Healthz`）

## Implementation Notes

- 實作新增 `internal/store`、`internal/queue`、`httpapi` 的 messages／errors／validate、`cmd/smsd` 組裝與 purge 迴圈、架構測試與 E2E、三份文件。
- `queue` 目前無消費者，約 1000 筆送出後即滿載，直到 Story 1.6 加 worker。
- 本機驗證需把 `C:\ProgramData\mingw64\mingw64\bin` 加進 PATH 並設 `CGO_ENABLED=1`，`go test ./... -race` 全綠。

## Design Notes

`Submit(ctx, tenantID, draft) (id, error)`：先 `reserve()`（計數器，滿 1000 回 `ErrQueueFull`），再 `store.Create`，成功才放入內部 channel；任一步失敗釋放名額。預設租戶 ID 為常數 `default`，由 `httpapi` 放進 request context，Epic 2 換成 middleware 解析。

## Verification

**Commands:**
- `go build ./... && go vet ./...` -- expected: 無輸出
- `CGO_ENABLED=1 go test ./... -race` -- expected: 全數 PASS（本機無 gcc 時無法執行 `-race`，須如實回報，改跑 `go test ./...`）
