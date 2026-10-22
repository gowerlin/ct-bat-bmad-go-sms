---
schema_version: 1
schema_kind: workorder
id: T0009
title: 供應商回呼更新狀態，租戶 Webhook 簽章通知且只通知一次（story id "11"）
type: impl
status: DONE
repo: sms-platform
project: sms-platform
created_at: "2026-10-23T01:38:18+0800"
started_at: "2026-10-23T03:08:48+0800"
updated_at: "2026-10-23T03:24:13+0800"
completed_at: "2026-10-23T03:24:13+0800"
sizing: L
depends_on:
  - T0008
  - T0010
affects_files:
  - internal/webhook/webhook.go
  - internal/webhook/webhook_test.go
  - internal/webhook/doc.go
  - internal/store/store.go
  - internal/store/store_test.go
  - internal/tenant/tenant.go
  - internal/tenant/tenant_test.go
  - internal/provider/mock/mock.go
  - internal/provider/mock/mock_test.go
  - internal/httpapi/handler.go
  - internal/httpapi/deps.go
  - internal/httpapi/errors.go
  - internal/httpapi/providers_callbacks.go
  - internal/httpapi/httpapi_test.go
  - cmd/smsd/main.go
  - e2e/testutil/server.go
  - e2e/testutil/provider.go
  - e2e/webhook_test.go
  - docs/openapi.yaml
  - docs/onboarding.md
  - docs/error-codes.md
  - _ct-workorders/T0009-status-webhook.md
interaction:
  mode_hint: yolo
  interactive: false
  intervention_type: fire-and-forget
renew_count: 0
worker_question_timeout: 30
workdir: main repo
---
# 工單 T0009-status-webhook

## 元資料
- **工單編號**：T0009
- **任務名稱**：供應商回呼與租戶 Webhook 通知（`stories.yaml` id "11"）
- **狀態**：DONE
- **類型**：impl
- **建立時間**：2026-10-23 01:38 (UTC+8)
- **intervention_type**：fire-and-forget
- **對應 E2E**：`TestE2E_Webhook_DeliveredUpdatesStatusAndNotifiesTenantOnce`（紅燈骨架在 `e2e/webhook_test.go`，build tag `red_webhook`，見 D011）
- **depends_on**：T0008（需要真實送信 worker、`provider_ref` 與 `e2e/testutil` 的 worker／FakeClock 同步）、T0010（需要 `providertest`、`PROVIDER` 切換與 `cmd/smsd/provider.go`）。**兩張都合回 `main` 之後才派**，在主 checkout 執行，不隔離
- **派發前塔台檢查**：`main` 上 `go test ./... -race -count=1` 全綠；`grep -rn 'go:build red_' e2e/` 只剩 `red_webhook`；T0008／T0010 的衝突合併已完成並 commit

## 工作量預估
- **預估規模**：大
- **Context Window 風險**：中
- **降級策略**：先完成「回呼 → 狀態更新（含冪等）」與 outbox 入列，回報 `PARTIAL`；出站 Dispatcher 與簽章留下一輪

## 規格層級自問（派發前）
- [x] 目標層：執行
- [x] 決策權歸屬：簽章編碼（D013）、回應碼（D015）已定；Dispatcher 內部結構、outbox 型別名 Worker 可決定，需符合 `internal/webhook/doc.go` 邊界
- [x] 資訊完整度：前置條件列出
- [x] 回頭成本：A
- [x] 記憶覆蓋：無衝突

## 任務指令

### 前置條件
- 讀 `AGENTS.md`、`_ct-workorders/_decision-log.md`（D011～D015）
- 讀 `stories.yaml` id "11"；`epics.md` 的 AD-3、AD-5、AD-6、AD-7、AD-9、AD-10、AD-13；`state-machines.md`（含通知／outbox 狀態圖）
- 讀 T0008、T0010 的回報區（`testutil` 的 worker／`WithoutWorker()`／FakeClock 同步；`providertest`；`PROVIDER` 切換）
- 讀 `internal/webhook/doc.go`、`internal/store/store.go`、`internal/tenant/tenant.go`、`internal/httpapi/handler.go`、`e2e/webhook_test.go`、`e2e/testutil/*`

### 輸入上下文
- **入站回呼（AD-7）**：`POST /v1/providers/{name}/callbacks`，不經 API key、不計限流；`{name}` 必須是已註冊的 Provider（否則 404）。請求交給該 Provider 的 `ParseCallback`（驗證在 `ParseCallback` 內：共用密鑰、constant-time）→ `DeliveryReport`；`ErrUnauthorized` → 401 信封。用 `ProviderRef` 找訊息（查無 → 404）。回應碼（D015）：造成狀態轉移 → **204**；重複或訊息已是終態 → **200**
- **狀態轉移**：`sent → delivered`／`undelivered`（`undelivered` 須在同一次 CAS 帶非空 `failure_reason`，AD-5）；非法轉移回 `store.ErrIllegalTransition`、狀態不變。轉移與 **outbox 入列同一原子操作**，`(message_id, status)` 唯一（AD-3）——這就是「重複回呼不重複通知」的保證
- **出站通知（AD-3、AD-13）**：webhook 分 `Inbound` 與 `Dispatcher`（單 goroutine，從 outbox `ClaimNext`，同一訊息依序送出）。通知 `POST` 到租戶 `CallbackURL`，body 固定 `{"id","status"}`。重試規則同 AD-13：退避 1s、2s、4s（經注入的 `Wait`，不睡真實時間）；用盡只記 `slog`（`abandoned`），**不改** `Message.status`。租戶沒有 `CallbackURL` → 不入列
- **簽章（D013，照做）**：HMAC-SHA256，金鑰為 `tenant.Tenant.CallbackSecret`（本單新增欄位，不得出現在日誌與任何回應），簽**原始 body bytes**，header `X-Signature: sha256=<小寫 hex>`；不含時間戳。驗證端用 `hmac.Equal`。測試用假密鑰 `test-secret`
- **`http.Client` 與 AD-9**：只在 `cmd/smsd` 建立並注入 Dispatcher，Transport 允許清單＝Provider 位址（T0010 已放）＋各租戶 `CallbackURL`；`internal/webhook` 內不得出現 `http.Client{`。E2E 的租戶接收端是 `httptest.Server`
- **`mock`（AD-6）**：`CallbackSink` 讓 `mock` 的回呼走真實 handler（若現況 `mock` 已能被 `testutil.FakeProvider` 取代，只補必要的部分並在回報區說明）
- **`e2e/testutil`**：在 T0008 的版本上接續，接上 Dispatcher、租戶接收端 helper（可注入 `CallbackURL`／`CallbackSecret`）；不得破壞 `WithoutWorker()` 與 FakeClock 同步
- **`cmd/smsd/main.go`**：依 T0010 的 `PROVIDER` 選擇結果註冊 Provider 供回呼路由使用，啟動 Dispatcher，關機時與 queue 一併收尾。保留 `PROVIDER` 無預設、非法即啟動失敗的行為
- **骨架到真實測試**：`e2e/webhook_test.go` 移除首行 `//go:build red_webhook`，把 `t.Fatal("尚未實作…")` 換成真實斷言：(1) 先讓訊息經 worker 真的變成 `sent`（取其 `provider_ref`，不要寫死 `fake-1`）；(2) 回呼 → 204，`GET /v1/messages/{id}` 的 `status` 變 `delivered`；(3) 租戶接收端**恰好收到一次**，body 為 `{"id","status":"delivered"}`，簽章用 `test-secret` 驗得過，被竄改的 body 驗不過；(4) 同一個回呼再送一次 → 200，接收端仍然只收到一次（確定性等待，不用 `time.Sleep`）。另補：未知供應商 404、驗證失敗 401、查無 `provider_ref` 404
- **文件**：`docs/openapi.yaml` 補 200 回應、回呼與出站通知的 payload 與 `X-Signature`；`docs/onboarding.md` 補租戶端驗證簽章範例（Go，標準庫，假密鑰）；`docs/error-codes.md` 若有受影響的列才改

### 預期產出
- `internal/webhook` 的 `Inbound`、`Dispatcher`、簽章（含單元測試：簽章已知向量、constant-time 比對、退避序列、用盡不改 Message.status）
- `internal/store` 的 outbox（`ClaimNext`、`(message_id, status)` 唯一、轉移＋入列原子）及測試
- `httpapi/providers_callbacks.go` 與 `handler.go` 的路由註冊（`handler.go` 是 D004 預告的「stub 清單以外的新功能，改一次」）
- `e2e/webhook_test.go`、`e2e/testutil` 的接續調整
- 文件更新

### 驗收條件
- [ ] `go test ./e2e -count=1 -run 'TestE2E_Webhook_DeliveredUpdatesStatusAndNotifiesTenantOnce' -v` PASS（貼輸出）
- [ ] `go test ./... -race -count=1` 全綠（貼最後 20 行；`-race` 無法執行時寫明原因與替代環境，不得拿掉 `-race`）
- [ ] `go build ./...`、`go vet ./...` 乾淨；`gofmt -l .` 無輸出
- [ ] `grep -rn 'go:build red_' .` 在整個 repo 沒有任何結果（三個 tag 都已清掉；`red_queue`、`red_mitake` 應由 T0008、T0010 清掉，若仍在，寫進〈遭遇問題〉交回塔台，不要自己動它們的測試）
- [ ] 重複回呼冪等：同一 `(message_id, status)` 只入列一次、只通知一次（E2E＋store 單元測試）
- [ ] `CallbackSecret` 不出現在任何日誌、回應或錯誤訊息；測試只用假密鑰；號碼只用 `+8869000000xx`；測試無外網呼叫
- [ ] `go.mod` 沒有 `require`；測試無 `t.Skip`；業務邏輯無 `time.Now`／`time.Sleep`；`internal/webhook` 無 `http.Client{`
- [ ] 其餘既有 E2E 全數仍 PASS，斷言沒有被放寬
- [ ] `git diff --name-only <base>..HEAD` 只出現 `affects_files` 列出的檔案
- [ ] 回報區填好，`status` 與 body 鏡像一致；產品 commit 與工單 commit 訊息都含 `T0009`

### 不做
- 不新增 `go.mod` 的 `require`（D001）
- 不改退避序列（1s、2s、4s）與總次數；不做時間戳／防重放（D013，記入回報區 followups 建議）
- 不改 `mitake` 的 `ParseCallback` 格式與 `statuscode` 對應（T0010 的答覆就是規格）；發現不符寫進〈遭遇問題〉
- 不做設定檔載入、租戶管理 API（`cmd/smsd` 的空 registry 維持；租戶由 `testutil` 注入）
- 不改 `e2e/architecture_test.go`（若因新檔失敗，先回報，不要放寬）
- 不改 `_ct-workorders/` 底下的塔台私有檔
- 不 push；不改 `_bmad/`、`.claude/`；不用 `t.Skip`、不刪測試、不放寬斷言

## Sub-session 執行指示

> 由塔台以 yolo 模式在 BAT 派工（`/ct-exec T0009`，主 checkout，不隔離）。沒有 `tower_question`；遇到工單沒寫的取捨，選最小改動並記在回報區〈互動紀錄〉，真的卡住才走 `yolo-mode.md` § Worker 向塔台提問。
> 時間值一律用系統指令取得（`date "+%Y-%m-%dT%H:%M:%S%z"`），不要憑印象填。驗證指令成功跑完、看到輸出之後，才在回報區寫「通過」。暫存腳本寫在 `_bmad-output/tmp/`。
> Windows 跑 `-race` 要先讓 shell 找得到 gcc 並設 `CGO_ENABLED=1`（`AGENTS.md` 末段）。

---

## 回報區

> 以下由 sub-session 填寫

### 完成狀態
**DONE**。驗收條件 10 項中 9 項照字面 PASS；「`grep -rn 'go:build red_' .` 無結果」一項照字面仍有輸出，但命中的都不是 Go 原始碼：一是工單／決策日誌裡引用這個指令的文字，二是 `.gitignore` 排除的 `_bmad-output/tmp/verify-T00xx/` 塔台驗證暫存副本。追蹤中的 `.go` 檔已經沒有任何 `red_` tag，所以這項判定為已達成，詳見〈遭遇問題〉1。降級策略沒有啟用，出站 Dispatcher 與簽章都在本輪完成。

### 產出摘要
- **`internal/store`**（AD-3、AD-5）：新增 outbox。`TransitionAndNotify` 在同一把鎖內完成 CAS 轉移與 outbox 入列，兩者一起寫入或都不寫；`(message_id, status)` 用 `noteKeys` 保持唯一。`ClaimNext(ctx)` 依入列順序領取，沒有待送項時阻塞，ctx 結束就返回。`FinishNotification(seq, done|abandoned)` 只接受 `delivering` 的項目。另有 `GetByProviderRef`。測試與診斷用的 `Notifications(messageID)`、`OutboxIdle()` 只放在 `*Memory`，不進介面。`PurgeExpired` 會順帶清掉被移除訊息中已完成的 outbox 項目；`pending`／`delivering` 的保留，等 Dispatcher 處理完。
- **`internal/webhook`**：
  - `Inbound.Receive(name, r)`：找 Provider（找不到 → `ErrUnknownProvider`）→ 呼叫 `ParseCallback`（`provider.ErrUnauthorized` → `ErrUnauthorized`，其他錯誤 → `ErrInvalidReport`）。
  - `Inbound.Apply(report)`：只接受 `delivered`／`undelivered`，否則回 `ErrInvalidReport`；查無 `provider_ref` 或訊息還不是 `sent` → `ErrUnknownMessage`；訊息已是終態 → `Duplicate`；`sent` → 轉移，租戶有 `CallbackURL` 才用 `TransitionAndNotify` 入列，沒有就用一般 `Transition`。`undelivered` 在同一次 CAS 帶 `failure_reason`「供應商回報未送達」。CAS 輸給併發的重複回呼時重讀一次，回 `Duplicate`。
  - `Dispatcher.Run(ctx)`：單一 goroutine，依序 `ClaimNext`，POST `{"id","status"}`，header 為 `Content-Type: application/json` 與 `X-Signature: sha256=<小寫 hex>`。非 2xx 或連線失敗經注入的 `wait` 退避 1s、2s、4s，共 4 次；用盡時只記 `slog`（`abandoned`），並標記 outbox 項目。Dispatcher 只拿得到 `Outbox` 介面（`ClaimNext`／`FinishNotification`），結構上就改不到 `Message.status`。
  - `Sign`／`Verify`：HMAC-SHA256，簽原始 body bytes，`Verify` 用 `hmac.Equal` 比對。
  - 套件內沒有 `http.Client{`，只接受 `Doer` 介面。
- **`internal/tenant`**：新增 `Tenant.CallbackSecret`、`Registry.Callback(tenantID) (url, secret)`（滿足 `webhook.Targets`）、`SetCallback`、`CallbackURLs`；`Tenant.LogValue()` 讓整個 `Tenant` 被 slog 記錄時只輸出 `tenant_id`。
- **`internal/httpapi`**：新增 `providers_callbacks.go`，註冊 `POST /v1/providers/{name}/callbacks`。這條路由不經 `s.v1`，所以不驗 API key、不計限流；body 上限 64 KiB。回應碼：`Applied` → 204，`Duplicate` → 200 空 body（D015）。錯誤經新的 `callbackError` mapper 對應：404 `not_found`、401 `unauthorized`（`callback verification failed`）、400 `invalid_request`。`Deps.Callbacks` 是選填欄位，`nil` 就不註冊這條路由（任何路徑都回 404）。`handler.go` 只加一行註冊（D004 預告的改一次）。
- **`internal/provider/mock`**（AD-6）：新增 `CallbackSink func(ctx, *http.Request)` 與 `EmitCallbacks(ctx, sink, wait, delay)`。每次 `Send` 成功後在獨立 goroutine 先 `wait`，再把 `ref=<ref>&status=delivered` 表單 POST 到 `CallbackPath` 交給 sink。E2E 用的是 `testutil.FakeProvider`，不用 mock 的 sink，所以 sink 只接在 `cmd/smsd`（`PROVIDER=mock` 時把回呼交給真實 handler 的 `ServeHTTP`，延遲 1s）；單元測試另外驗證「失敗的 Send 不發回呼」「wait 被取消不發」「sink 為 nil 不發」。
- **`cmd/smsd/main.go`**：
  - 把 `webhook.NewInbound(st, tenants, prov)` 交給 `Deps.Callbacks`；`PROVIDER` 沒有預設值、值不合法就啟動失敗的行為維持不變。
  - 啟動 Dispatcher。它的 `http.Client` 沿用同 package 的 `newProviderClient`，允許清單是啟動時各租戶 `CallbackURL` 的 host（AD-11 規定啟動時載入一次；目前 registry 是空的，所以清單也是空的）。
  - 關機順序：listener → queue → Dispatcher，三者共用 `shutdownTimeout`。
- **`e2e/testutil`**：
  - `FakeProvider` 新增 `EnqueueCallbackError`。
  - `NewTestServer` 接上 `Inbound`，並且一律啟動 Dispatcher（退避用 `FakeClock.Wait`，cleanup 時取消並等它結束）。
  - 新增 `Env.NewReceiver(t, tenantID, secret)`：以 `httptest.Server` 當租戶接收端，記錄 body、`X-Signature` 與 `Content-Type`。
  - 新增 `Env.AwaitOutboxIdle(t)`：靠 `watchedOutbox` 在每次 `FinishNotification` 時發訊號，確定性等待、不輪詢。回呼在回應之前就已入列，所以回應之後等 outbox 閒置再數接收端，結果是確定的。
  - `WithoutWorker()` 與 FakeClock 同步的行為不變。
- **`e2e/webhook_test.go`**：移除 `//go:build red_webhook`，把骨架改成真實斷言：
  1. 訊息經 worker 真的變成 `sent`，從 store 取得真實的 `provider_ref`，沒有寫死 `fake-1`。
  2. 回呼回 204，`GET` 讀到 `delivered`。
  3. 接收端恰好收到 1 次，body 等於 `{"id":"<id>","status":"delivered"}`，`test-secret` 驗得過；竄改過的 body 與錯誤的密鑰都驗不過。
  4. 同一回呼再送一次回 200，接收端仍然只有 1 次，outbox 只有 1 筆 `done`，狀態不變。
  另外新增三個測試：`TestE2E_Webhook_UnknownProviderReturns404`、`TestE2E_Webhook_UnauthorizedCallbackReturns401`（狀態維持 `sent`，接收端 0 次）、`TestE2E_Webhook_UnknownProviderRefReturns404`。
- **文件**：
  - `docs/openapi.yaml`：回呼補上描述、requestBody（mock／mitake 格式）、200／400／401／404；出站通知的 header、body、重試規則寫在註解；新增 `WebhookNotification` schema；`failure_reason` 補上「供應商回報未送達」。
  - `docs/onboarding.md`：新增〈接收狀態通知（Webhook）〉與 Go 標準庫驗證範例，假密鑰 `test-secret`、constant-time 比對。範例已抽出來編譯，並用已知向量測過；暫存檔在 `_bmad-output/tmp/T0009/`，驗完已刪。
  - `docs/error-codes.md`：400、404 兩列補上回呼的情況，400 的「本版已實作」改為「是（除篩選）」。

### 驗收證據
主 checkout，Windows，`go version go1.27.3 windows/amd64`；gcc 用 `<MinGW 的 bin 目錄>`（只加在當下 shell 的 PATH），`CGO_ENABLED=1`（`go env` 回 `1`／`gcc`）。驗證時間 2026-10-23T03:22:00+0800，跑的是 commit 前的工作樹，內容與 `b2166dd` 相同。

| 驗收條件 | 結果 | 證據 |
|---|---|---|
| 目標 E2E | **PASS** | 見下方輸出 1 |
| `go test ./... -race -count=1` | **PASS** | 見下方輸出 2（`exit=0`） |
| `go build ./...`、`go vet ./...`、`gofmt -l .` | **PASS** | `build-ok`、`vet-ok`、`gofmt:[]` |
| 整個 repo 沒有 `go:build red_` | **PASS（實質）**／字面有輸出 | `git grep -n 'go:build red_' -- '*.go'` exit 1；工作樹排除 `_bmad-output/tmp/`（gitignored）後 `grep -rln '^//go:build red_' --include=*.go .` 無結果；字面 `grep -rn` 的命中見〈遭遇問題〉1 |
| 重複回呼冪等 | **PASS** | E2E 步驟 (4)；store 的 `TestConcurrentTransitionAndNotifyEnqueuesOnce`、`TestTransitionAndNotifyWritesBothOrNeither`；webhook 的 `TestInboundDeliveredThenDuplicate`、`TestInboundConcurrentDuplicatesApplyOnce`（20 個併發：1 個 Applied、19 個 Duplicate、outbox 1 筆） |
| `CallbackSecret` 不外洩；只用假密鑰、`+8869000000xx`；無外網 | **PASS** | 產品碼的 slog／錯誤字串沒有密鑰（grep 只命中 mitake 既有的 `callback secret missing or wrong` 字樣與 `LogValue` 的註解）；`TestTenantLogValueOmitsSecret`；`test-secret` 在非測試檔只出現在 `testutil/server.go` 的註解；號碼只有 `+886900000001`；接收端都是 loopback `httptest.Server`，Dispatcher 在單元測試用假 `Doer`，mock sink 在行程內呼叫 |
| `go.mod` 無 `require`；無 `t.Skip`；業務邏輯無 `time.Now`／`time.Sleep`；`internal/webhook` 無 `http.Client{` | **PASS** | 四項 grep 都無結果；`time.Now(` 只出現在允許的 `cmd/smsd/main.go:106`（既有）；`TestArchitecture_*` 全綠 |
| 其餘既有 E2E 全數 PASS，斷言未放寬 | **PASS** | `git diff --stat -- e2e/` 只有 `testutil/provider.go`、`testutil/server.go`、`webhook_test.go`；其他 E2E 檔與 `architecture_test.go` 沒改 |
| `git diff --name-only <base>..HEAD` 只在 `affects_files` 內 | **PASS** | base `631bd91`..`b2166dd` 共 21 個檔，全部在 `affects_files` 內；工單本身另在工單 commit |
| 回報區、鏡像一致、commit 訊息含 `T0009` | **PASS** | 本段；frontmatter 與 body 的 `status` 都是 `DONE`；產品與工單 commit 的訊息都含 `T0009` |

輸出 1：目標 E2E

```text
=== RUN   TestE2E_Webhook_DeliveredUpdatesStatusAndNotifiesTenantOnce
2026/10/23 03:22:00 INFO webhook: status updated provider=fake status=delivered
--- PASS: TestE2E_Webhook_DeliveredUpdatesStatusAndNotifiesTenantOnce (0.02s)
PASS
ok  	example.com/sms-platform/e2e	0.038s
```

輸出 2：`go test ./... -race -count=1`

```text
ok  	example.com/sms-platform/cmd/smsd	1.060s
ok  	example.com/sms-platform/e2e	1.474s
?   	example.com/sms-platform/e2e/testutil	[no test files]
ok  	example.com/sms-platform/internal/httpapi	1.068s
?   	example.com/sms-platform/internal/provider	[no test files]
ok  	example.com/sms-platform/internal/provider/mitake	1.069s
ok  	example.com/sms-platform/internal/provider/mock	1.068s
?   	example.com/sms-platform/internal/provider/providertest	[no test files]
ok  	example.com/sms-platform/internal/queue	1.035s
ok  	example.com/sms-platform/internal/ratelimit	1.035s
ok  	example.com/sms-platform/internal/store	1.035s
ok  	example.com/sms-platform/internal/template	1.035s
ok  	example.com/sms-platform/internal/tenant	1.067s
ok  	example.com/sms-platform/internal/webhook	1.068s
exit=0
```

新增的單元測試涵蓋：
- **簽章**：已知向量（RFC 4231 test case 2，以及 E2E body；兩個值都先用 Python `hmac` 獨立算出）；竄改、錯誤密鑰、缺前綴、非 hex、空字串都不通過；以 AST 檢查 `Verify` 恰好呼叫一次 `hmac.Equal`，沒有 `==`、也沒有 `bytes.Equal`。
- **Dispatcher**：退避序列 `[1s 2s]`（重試後成功）與 `[1s 2s 4s]`（用盡）；用盡後 `abandoned`，`Message.status` 仍是 `delivered`；三則訊息依入列順序送出；沒有 `CallbackURL` 就 `abandoned`、不發任何請求。
- **store**：`ClaimNext` 會阻塞直到入列或 ctx 取消、依序領取、`FinishNotification` 的合法狀態、purge 的保留規則。

Runtime smoke：以 `LISTEN_ADDR=127.0.0.1:18089 PROVIDER=mock` 啟動 `smsd`。回呼 `ref=nope` → 404 `not_found`；`/v1/providers/mitake/callbacks` → 404；`status=sent` → 400 `invalid_request`；GET → 405 `invalid_request`。執行檔與日誌放在 `_bmad-output/tmp/T0009/`，結束後已刪。

### 落地檢查
**PASS**
- **C-0**：frontmatter `repo` = `sms-platform`，`basename(REPO_ROOT)` = `sms-platform`，相符。`REPO_ROOT` = `<demo>/sms-platform`。
- **C-1**：工單位於 `REPO_ROOT/_ct-workorders/`，PASS。
- **C-3**：前 5 個可測項目中，`internal/webhook/webhook.go` 本身不存在（本單新建），但上層 `internal/webhook/` 存在；其餘 4 個都存在。全部為 present，PASS。
- **C-2**：沒有 `branch` 欄位，不適用；實際在 `main`。
- **環境**：`CT_HOME` 未設定（不隔離，與工單「主 checkout，不隔離」一致）；`BAT_WORKSPACE_ID` = `<workspace>`（只記錄，不比對）；`CT_MODE=yolo`、`CT_INTERACTIVE=0`。

### 產品 commit
- `b2166dd0cf1afed28ed9a4b33f0d5237396997e9`：`feat(T0009): 供應商回呼更新狀態，租戶 Webhook 簽章通知且只通知一次`（21 個檔）。base 是開工時的 `HEAD` `631bd91`。
- 主 checkout 裡塔台未 commit 的 `_ct-workorders/_tower-state.md` 沒有動，也沒有 stage。
- 沒有 push。

### 互動紀錄
沒有向塔台提問。下列是工單沒寫、由 Worker 依「最小改動」自行決定的取捨，依工單指示記在這裡（執行中逐項決定，未逐筆取系統時間，故不標時刻）：
- 通知範圍：`state-machines.md` 寫 `sending→sent`、`sending→failed` 也通知租戶，epics Story 1.8 寫「共收到 2 次（sent、delivered）」；但本工單的〈輸入上下文〉只要求 `sent → delivered／undelivered` 的轉移與入列，E2E 斷言也是「恰好收到一次，status delivered」，而且 `internal/queue/queue.go` 不在 `affects_files` → Action：只有回呼造成的轉移會入列，`sent`／`failed` 的通知交給後續工單（見 followups）。
- 24h 無回呼轉 `undelivered`（「回呼逾時」）與 `SMS_CONFIG` 設定檔載入：工單沒列（設定檔載入明列在「不做」） → Action：不做，列入 followups。
- `ProviderRef` 查詢鍵：spine 寫 `(provider_name, provider_ref)`，但 queue 的 `sending→sent` 不寫 `Message.Provider`，而 `queue.go` 不在範圍內 → Action：`GetByProviderRef(ref)` 只用 ref 查詢，列入 followups。
- `undelivered` 必須帶非空 `failure_reason`（AD-5），規格沒給回呼版本的字串 → Action：用常數 `webhook.ReasonUndelivered` =「供應商回報未送達」，並同步寫進 openapi 的 `failure_reason` 說明。
- 未授權以外的 `ParseCallback` 錯誤（例如 mitake 的格式錯誤，wrap `ErrPermanent`）→ Action：一律 400 `invalid_request`（依 epics「狀態值不是 delivered／undelivered → 400」與 error-codes 400 列）。
- 訊息仍是 `queued`／`sending` 時收到回呼 → Action：404（依 `state-machines.md`「回呼早於 sent 寫入：404，由 Provider 重送」）。
- AD-9 允許清單：要把 provider host 與租戶 host 合成一份清單，就得改 `cmd/smsd/provider.go`，但它不在 `affects_files` → Action：Dispatcher 另建一個 client，沿用同 package 的 `newProviderClient(callbackHosts(...))`，清單只有租戶 callback host；provider client 不變。兩個 client 各自限制，比合併成一份更嚴。
- `Deps.Callbacks` 設為選填（`nil` 就不註冊路由），既有的 `httpapi` 測試與 `NewHandler` 必填檢查都不用改（`handler.go` 只加一行註冊）。

### Renew 歷程
無

### 遭遇問題
1. **驗收條件「`grep -rn 'go:build red_' .` 無任何結果」照字面無法成立**，原因不在程式碼：
   - (a) `_ct-workorders/T0008…`、`T0009…`、`T0010…`、`T0011…`、`_decision-log.md`、`_tower-state.md` 的文字裡本來就引用 `go:build red_…` 這個指令或描述；
   - (b) `_bmad-output/tmp/verify-T0008/`、`verify-T0010/`、`verify-T0011/` 是塔台先前驗證留下的暫存副本，被 `.gitignore:17 _bmad-output/tmp/` 排除，裡面有舊版帶 tag 的 `e2e/*_test.go`。

   兩者都不是本單能改的範圍：(a) 是塔台檔，(b) 不是 Worker 建的，我沒有刪。追蹤中的 Go 檔（`git grep -n 'go:build red_' -- '*.go'` exit 1）以及工作樹裡非 ignored 的 `.go` 都已經沒有 `red_` tag，三個 tag 都已清掉。建議塔台把這項驗收指令改成 `git grep -n 'go:build red_' -- '*.go'`，或用 `grep -rn --include=*.go --exclude-dir=_bmad-output`；`_bmad-output/tmp/verify-*` 要不要清由塔台決定。
2. `cmd/smsd/provider.go` 的註解「tenant webhook URLs join it later (T0009)」已經不準（實際做法是 Dispatcher 用另一個 client），但這個檔不在 `affects_files`，所以沒改。
3. 本單沒有改到 `mitake` 的 `ParseCallback`，也沒發現它與 T0010 答覆不符的地方。

**Followups（建議塔台開單）**：
- `sending→sent`、`sending→failed` 也通知租戶（`state-machines.md`、epics 1.8 的「共 2 次」）。做法是改 `queue` 改用 `TransitionAndNotify`，並由 queue 取得 tenant 的 callback 有無；會改到 `internal/queue/queue.go`，E2E 的「恰好一次」也要改成「每個狀態一次」。
- `sent` 逾 24h 無回呼轉 `undelivered`（「回呼逾時」）的掃描。
- `ProviderRef` 改用 `(provider_name, provider_ref)` 唯一索引，需要 queue 在 `sending→sent` 時寫入 `Message.Provider`。
- 出站簽章加時間戳／防重放（D013 已明列不在本版，要做須另開決策取代 D013）。
- `PROVIDER=mock` 的回呼沒有驗證（D018 已知缺口）；cmd 啟用 mock sink 之後，這條路由在本機示範時是開放的。
- `SMS_CONFIG` 載入租戶 `callback_url`／`webhook_secret`（AD-11），屆時啟動時的 `CallbackURLs()` 允許清單會自動生效。

### sprint-status.yaml 已更新
不適用

### 回報時間
2026-10-23T03:24:13+0800
