---
schema_version: 1
schema_kind: workorder
id: T0002
title: mock Provider 故障注入（story 2）
type: impl
status: DONE
repo: sms-platform
project: sms-platform
created_at: "2026-10-22T18:55:16+0800"
started_at: "2026-10-22T18:57:14+0800"
updated_at: "2026-10-22T18:58:14+0800"
completed_at: "2026-10-22T18:58:14+0800"
sizing: S
affects_files:
  - internal/provider/mock/mock.go
  - internal/provider/mock/mock_test.go
  - e2e/mock_fault_test.go
  - _ct-workorders/T0002-mock-provider-fault-injection.md
interaction:
  mode_hint: off
  interactive: false
  intervention_type: fire-and-forget
renew_count: 0
workdir: main repo
---
# 工單 T0002-mock-provider-fault-injection

## 元資料
- **工單編號**：T0002
- **任務名稱**：mock Provider 故障注入（`stories.yaml` id "2"）
- **狀態**：DONE
- **類型**：impl
- **建立時間**：2026-10-22 18:55 (UTC+8)
- **開始時間**：2026-10-22 18:57 (UTC+8)
- **完成時間**：2026-10-22 18:58 (UTC+8)
- **intervention_type**：fire-and-forget
- **對應 E2E**：`TestE2E_MockAdapter_FaultInjection`（以 e2e 測試名對應 story；epics 編號不同，不要用）
- **affects_files**：
  - `internal/provider/mock/mock.go`、`internal/provider/mock/mock_test.go`
  - `e2e/mock_fault_test.go`
  - `_ct-workorders/T0002-mock-provider-fault-injection.md`
- **與 T0003 的關係**：兩單 `affects_files` 不重疊，可各自獨立完成，無先後相依

## 任務指令

### 前置條件
- 讀 `AGENTS.md`、`_ct-workorders/_decision-log.md`（D001）
- 讀 `_bmad-output/specs/spec-sms-platform/stories.yaml` id "2"、`SPEC.md` CAP-3
- 讀 `internal/provider/provider.go`、`internal/provider/mock/mock.go`、`e2e/architecture_test.go`（架構測試會擋 `time.Sleep(`、`http.Client{`、import 圖）
- 本機 `go version` 須為 go1.27.3，否則回報 BLOCKED，不要自己換版本

### 輸入上下文
目前 `mock.Provider.Send` 永遠成功。這張單讓**測試**可以注入失敗：暫時失敗、永久失敗、timeout、delay。後面的重試與失敗路徑 E2E（story 9、10）會用它。

塔台掃描的事實：`ErrTransient`／`ErrPermanent`／`ErrUnauthorized` **已經存在**於 `internal/provider/provider.go`（AD-1），所以本單**不需要也不可以改 `provider.go`**。mock 直接 wrap 這兩個 sentinel（`fmt.Errorf("mock: …: %w", provider.ErrTransient)`）。若你判斷 `provider.go` 一定要改，停下來回報 BLOCKED／PARTIAL 並寫進〈遭遇問題〉。

設計限制（硬性）：
- `Send` 不重試、不 `time.Sleep`（架構測試禁止）；等待用 timer／channel 並尊重 `ctx` 取消
- delay 的等待要能注入（例如 option 帶一個 `after func(time.Duration) <-chan time.Time`），預設走標準 timer；E2E 測試用注入的等待，不真的睡
- error 的 reason 不得含號碼或內容（`provider.go` 註解）
- 不發任何外部網路請求；不注入時行為與 CH1 骨架相同（成功、`mock-N` 序號 ref、既有 `New`／`NewWithClock` 簽名保持可用）
- 注入設定由測試直接呼叫 mock 的方法或 option 設定；並行安全（`-race`）
- 具體 API 命名由你決定，寫進回報區〈產出摘要〉；至少涵蓋：暫時失敗（N 次後恢復／持續）、永久失敗、timeout（阻塞到 ctx 結束並回 `ctx.Err()`）、delay、清除注入

### 預期產出
- `internal/provider/mock/mock.go`：故障注入能力
- `internal/provider/mock/mock_test.go`：套件單元測試（`errors.Is` 對應 sentinel、ctx 取消）
- `e2e/mock_fault_test.go`：`TestE2E_MockAdapter_FaultInjection`，子測試涵蓋 fail（暫時／永久）、timeout、delay 三種注入；不經網路

### 驗收條件（共用）
- [ ] `go test ./... -race` 全綠（貼最後 20 行輸出；`-race` 無法執行時寫明原因與替代環境）
- [ ] `go vet ./...` 沒有輸出
- [ ] `gofmt -l .` 沒有輸出
- [ ] `git diff --name-only <base>..HEAD` 只出現本工單 `affects_files` 列出的檔案（`<base>` = 本工單開工時的 HEAD；另一張工單可能同時在同一個工作目錄 commit，若 diff 出現對方的檔案，改用 `git show --name-only <本工單的每個 commit>` 逐個核對，並在回報區註明）
- [ ] 回報區填好、`status` 與 body 鏡像一致；產品 commit 與工單 commit 訊息都含工單編號

### T0002 專屬條件
- [ ] `go test ./e2e -run TestE2E_MockAdapter_FaultInjection -v` 通過，涵蓋 fail、timeout、delay 三種注入
- [ ] mock 的 fail／timeout／delay 可由測試直接設定、不需要網路；不注入時行為與 CH1 骨架相同

### 不做
- 不新增 `go.mod` 的 `require`（D001；要加依賴須在工單明示並記新決策）
- 不改另一張工單的 `affects_files`；發現需要改，寫進回報區〈遭遇問題〉交回塔台
- 不改 `_ct-workorders/` 底下的塔台私有檔（`_tower-state.md`、`_decision-log.md`…）
- 不 push；不改 `_bmad/`、`.claude/`；不用 `t.Skip`、不刪測試、不放寬斷言
- 不改 `internal/provider/provider.go`（sentinel 已存在）；不改 `e2e/testutil/*`、`internal/httpapi/*`、`internal/tenant/*`
- 不把 mock 接進 queue、不做重試邏輯（story 9、10）

## Sub-session 執行指示

> 在第二個 Claude Code 終端（同一個 `sms-platform/`）執行 `/ct-exec T0002`。
> Worker 先做落點檢查（C-0 比對 `repo`），再把 `status` 改成 `IN_PROGRESS`、填 `started_at`，最後依序：產品 commit → 回報區 → 狀態與完成時間 → 工單 commit。
> 時間值一律用系統指令取得，不要憑印象填。

---

## 回報區

> 以下由 Worker 填寫，塔台不在這裡編輯

### 完成狀態
DONE

### 產出摘要
- 落點檢查：C-0 PASS（`repo`=sms-platform，`basename(REPO_ROOT)`=sms-platform）；C-1 PASS；C-2 PASS（main）；`CT_HOME`／`BAT_*` 未設定（非 worktree、非 BAT，通知走手動訊息）。`go version` = go1.27.3。
- 產品 commit：`f12b02e`（`feat(mock): T0002 mock Provider 故障注入`）。
- `internal/provider/mock/mock.go`：新增注入 API（皆並行安全，mutex 保護；`New`／`NewWithClock` 簽名不變，不注入時行為同 CH1 骨架）：
  - `FailTransient(n int)`：接下來 n 次 `Send` 回 wrap `provider.ErrTransient` 的錯誤後恢復；`n <= 0` 持續到 `Reset`
  - `FailPermanent()`：持續回 wrap `provider.ErrPermanent`
  - `Timeout()`：`Send` 阻塞到 ctx 結束，回 `ctx.Err()`
  - `Delay(d)`：`Send` 先等 d 再成功／失敗；等待可被 ctx 取消
  - `SetAfter(func(time.Duration) <-chan time.Time)`：注入 delay 的等待（預設 `time.After`），測試不真睡
  - `Reset()`：清除所有注入
  - error reason 固定為 `mock: injected failure`，不含號碼或內容；失敗的 Send 不消耗 `mock-N` 序號。
- `mock_test.go`：預設成功／序號、暫時失敗 N 次後恢復、持續失敗＋Reset、永久失敗（`errors.Is`、不洩漏號碼內容）、timeout（Canceled／DeadlineExceeded）、delay（注入等待＋取消）、已取消 ctx、並行安全。
- `e2e/mock_fault_test.go`：`TestE2E_MockAdapter_FaultInjection`，子測試 fail transient、fail permanent、timeout、delay。
- 未改 `provider.go`、`go.mod`、`e2e/testutil/*` 等排除項。

### 驗收證據
- `go test ./... -race`（`CGO_ENABLED=1`）全綠，最後輸出：
```
ok  	example.com/sms-platform/e2e	1.305s
ok  	example.com/sms-platform/internal/httpapi	(cached)
ok  	example.com/sms-platform/internal/provider/mock	(cached)
ok  	example.com/sms-platform/internal/queue	(cached)
ok  	example.com/sms-platform/internal/store	(cached)
```
  （前一次未 cached 的完整執行同樣全綠；架構測試通過。）
- `go vet ./...`：無輸出。`gofmt -l .`：無輸出（中途 mock.go 一度被 gofmt 列出，已 `gofmt -w` 修正後才 commit）。
- `go test ./e2e -run TestE2E_MockAdapter_FaultInjection -v`：4 個子測試全 PASS。
- `git diff --name-only bc6c577..HEAD`（commit 時）：只有 `e2e/mock_fault_test.go`、`internal/provider/mock/mock.go`、`internal/provider/mock/mock_test.go`，皆在 `affects_files` 內；無他單 commit 混入。

### 互動紀錄
無（fire-and-forget，無提問）。

### 遭遇問題
無。

### 回報時間
2026-10-22T18:58:14+0800
