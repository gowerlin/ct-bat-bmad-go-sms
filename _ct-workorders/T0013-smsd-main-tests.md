---
schema_version: 1
schema_kind: workorder
id: T0013
title: 補 cmd/smsd/main_test.go：關機順序、purgeLoop、q.Run 錯誤、provider 組裝（epic 1 回顧 F10）
type: test
status: PENDING
repo: sms-platform
project: sms-platform
created_at: "2026-10-23T07:13:08+0800"
sizing: M
depends_on:
  - T0012
affects_files:
  - cmd/smsd/main_test.go
  - cmd/smsd/main.go
  - cmd/smsd/app.go
  - _bmad-output/implementation-artifacts/deferred-work.md
  - _ct-workorders/T0013-smsd-main-tests.md
interaction:
  mode_hint: yolo
  interactive: false
  intervention_type: fire-and-forget
renew_count: 0
worker_question_timeout: 30
workdir: main repo
---
# 工單 T0013-smsd-main-tests

## 元資料
- **工單編號**：T0013
- **任務名稱**：補 `cmd/smsd/main_test.go`（組裝根的自動化驗證）
- **狀態**：PENDING
- **類型**：test
- **建立時間**：2026-10-23 07:13 (UTC+8)
- **intervention_type**：fire-and-forget
- **來源**：`_bmad-output/implementation-artifacts/epic-1-retro-2026-10-23.md` Action item 6（F10）；決策 D025
- **depends_on**：T0012（需要它抽出的組裝函式；兩張都動 `cmd/smsd/main.go`，T0012 合回 `main` 之後才派，在主 checkout 執行，不隔離）
- **派發前塔台檢查**：T0012 `DONE` 且 commit 在 `main`；`go test ./... -race -count=1` 全綠；讀 T0012 回報區〈產出摘要〉取得組裝函式名稱與 seam

## 工作量預估
- **預估規模**：中
- **Context Window 風險**：低
- **降級策略**：先完成關機順序與 `purgeLoop` 測試，回報 `PARTIAL`；`q.Run` 錯誤記錄與 provider 組裝（含變異驗證）留下一輪

## 規格層級自問（派發前）
- [x] 目標層：執行
- [x] 決策權歸屬：測試設計與為可測性做的最小重構 Worker 可決定；行為不得改變（`q.Run` 錯誤記錄除外，見下）
- [x] 資訊完整度：T0012 完成後足夠
- [x] 回頭成本：A
- [x] 記憶覆蓋：無衝突

## 任務指令

### 前置條件
- 讀 `AGENTS.md`、`_ct-workorders/_decision-log.md`（D025）
- 讀 `epic-1-retro-2026-10-23.md` F10；`_bmad-output/implementation-artifacts/deferred-work.md` 中 spec-1-1 一筆與 T0008 第 3 筆
- 讀 T0012 回報區；`cmd/smsd/main.go`、`cmd/smsd/app.go`、`cmd/smsd/provider_test.go`（沿用其 helper 風格）、`internal/queue/queue.go` 的 `Queue` 介面與 `Run`／`Close`

### 輸入上下文
**問題（F10）**：`cmd/smsd` 的組裝沒有自動化驗證：`run` 的關機順序、`purgeLoop`、`go func() { _ = q.Run(ctx) }()` 丟掉的錯誤，T0008～T0010 三次動到 `main.go` 都沒有測試擋。實測：把 `queue.NewInProcess(st, queue.WithProvider(prov))` 改成 `queue.NewInProcess(st)`（不傳 provider）時，全部測試仍過。

**為可測性允許的最小重構**（不得改變正常路徑的行為）：
- `purgeLoop` 目前直接用 `time.NewTicker`／`time.Now`，真實間隔是 1 分鐘，不能在測試裡等。改成注入 tick channel 與 `now func() time.Time`（或等價做法），`main`／組裝處傳真實的
- `go func() { _ = q.Run(ctx) }()` 改成呼叫一個小 helper：`Run` 回傳非 nil 且不是 `context.Canceled` 的錯誤時，以 `slog.Error("queue stopped", "error_class", "queue")` 記錄（AD-15：不放內容、號碼、密鑰）。**這是本單唯一允許的行為變更**，在回報區明列
- 其餘 seam 沿用 T0012 的組裝函式

### 預期產出
`cmd/smsd/main_test.go`，至少包含：
1. **關機順序**：呼叫 `run`，注入記錄呼叫順序的假 `queue.Queue` 與假 `stopDispatcher`；取消 ctx 後斷言順序為「listener 停止接受連線 → `q.Close` → `stopDispatcher`」（`run` 的註解寫的就是這個順序）；另測 `shutdownTimeout` 到期時三個錯誤被 `errors.Join` 回傳（`shutdownTimeout` 若為常數，改成可注入的參數，預設值不變）；`Serve` 自己回錯（非 `ErrServerClosed`）時 `run` 回傳該錯誤
2. **`purgeLoop`**：假 `store.System` 記錄 `PurgeExpired(now)` 的參數；送 tick → 以注入的時鐘值被呼叫一次；ctx 取消 → 函式返回（用 channel／`WaitGroup` 同步，**不 `time.Sleep`**）
3. **`q.Run` 錯誤要有 log**：假 `Queue` 的 `Run` 回傳固定錯誤 → 以暫時替換的 `slog` handler（寫進 `bytes.Buffer`，測完還原）斷言有 `queue stopped` 與 `error_class=queue`、不含該錯誤的原文以外的敏感字；`Run` 回 `context.Canceled` 或 `nil` → 不記錯誤
4. **provider 組裝確實傳入**：透過 T0012 的組裝函式，`PROVIDER=mock` 時送一則訊息，最終狀態為 `sent` 且 `provider_ref` 非空（用確定性等待：channel／通知／條件，不 `time.Sleep`、不吃真實時間）；`PROVIDER` 非法或空 → 組裝函式回錯誤
5. **變異驗證（必做，貼結果）**：暫時把 `queue.WithProvider(prov)` 拿掉 → 第 4 項測試**必須失敗**；暫時把 `run` 內 `q.Close` 與 `stopDispatcher` 對調 → 第 1 項**必須失敗**；每次變異後還原並重跑確認回到全綠。若某個變異沒有被擋下，補測試直到擋下
6. `deferred-work.md`：把已由本單處理的 spec-1-1 與 T0008 第 3 筆條目標記為已處理（只追加／標註，不刪原文）

### 驗收條件
- [ ] `go test ./cmd/smsd -race -count=1 -v` 全 PASS（貼輸出，列出新測試名）
- [ ] `go test ./... -race -count=1` 全綠（貼最後 20 行；`-race` 無法執行時寫明原因與替代環境，不得拿掉 `-race`）
- [ ] `go build ./...`、`go vet ./...` 乾淨；`gofmt -l .` 無輸出
- [ ] 預期產出第 5 項的兩個變異各自「測試變紅 → 還原 → 轉綠」的輸出貼在回報區（成功跑完看到輸出才寫「通過」）；最後 `git diff` 確認變異已還原
- [ ] 正常路徑行為不變：除 `q.Run` 錯誤記錄外，`main.go`／`app.go` 的 diff 只有為注入 seam 的簽名調整；`purgeInterval`、`shutdownTimeout` 預設值不變
- [ ] `go.mod` 沒有新增 `require`；`git grep -n 't\.Skip' -- '*.go'` exit `1`；`cmd/smsd` 測試無 `time.Sleep(`；測試無外網呼叫（用 `httptest`／`net.Listen("127.0.0.1:0")`）
- [ ] 既有 `e2e/`、`features/`、`cmd/smsd` 測試全數仍 PASS，斷言沒有被放寬
- [ ] `git diff --name-only <base>..HEAD` 只出現 `affects_files` 列出的檔案
- [ ] 回報區填好，`status` 與 body 鏡像一致；產品 commit 與工單 commit 訊息都含 `T0013`

### 不做
- 不新增 `go.mod` 的 `require`（D001）
- 不改 `internal/*` 任何套件；`queue.Queue` 介面不動
- 不加新功能、不改設定載入（T0012）、不改退避／逾時值；不處理 F8（`Provider.Send` 逾時）與 F9（空 `ProviderRef`），那是另案
- 不改 `e2e/architecture_test.go`（`cmd/smsd` 的 `ForbiddenCalls` 矛盾是 D020 followup，不在本單）；若測試需要 `http.Client`，沿用 `newProviderClient` 或 `httptest` 的 client
- 不改 `_ct-workorders/` 底下的塔台私有檔；不 push；不改 `_bmad/`、`.claude/`；不用 `t.Skip`、不刪測試、不放寬斷言

## Sub-session 執行指示

> 由塔台以 yolo 模式在 BAT 派工（`/ct-exec T0013`，主 checkout，不隔離）。遇到工單沒寫的取捨，選最小改動並記在回報區〈互動紀錄〉，真的卡住才走 `yolo-mode.md` § Worker 向塔台提問。
> 時間值一律用系統指令取得（`date "+%Y-%m-%dT%H:%M:%S%z"`），不要憑印象填。驗證指令成功跑完、看到輸出之後，才在回報區寫「通過」。暫存腳本寫在 `_bmad-output/tmp/`。
> Windows 跑 `-race` 要先讓 shell 找得到 gcc 並設 `CGO_ENABLED=1`（`AGENTS.md` 末段）。

---

## 回報區

> 以下由 sub-session 填寫

### 完成狀態
（DONE / FAILED / BLOCKED / PARTIAL）

### 產出摘要

### 互動紀錄

### Renew 歷程
無

### 遭遇問題

### sprint-status.yaml 已更新
不適用

### 回報時間
