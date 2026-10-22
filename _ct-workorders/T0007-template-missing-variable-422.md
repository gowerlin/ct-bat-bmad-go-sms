---
schema_version: 1
schema_kind: workorder
id: T0007
title: 模板缺變數回 422 missing_template_variable（story id "8"）
type: impl
status: DONE
repo: sms-platform
project: sms-platform
created_at: "2026-10-22T23:07:32+0800"
started_at: "2026-10-22T23:57:42+0800"
updated_at: "2026-10-22T23:59:27+0800"
completed_at: "2026-10-22T23:59:27+0800"
sizing: S
depends_on:
  - T0004
affects_files:
  - internal/template/template.go
  - internal/template/template_test.go
  - internal/httpapi/errors_templates.go
  - internal/httpapi/messages_send.go
  - e2e/template_missing_variable_test.go
  - _ct-workorders/T0007-template-missing-variable-422.md
interaction:
  mode_hint: yolo
  interactive: false
  intervention_type: fire-and-forget
renew_count: 0
worker_question_timeout: 30
workdir: main repo
---
# 工單 T0007-template-missing-variable-422

## 元資料
- **工單編號**：T0007
- **任務名稱**：模板缺變數回 422（`stories.yaml` id "8"）
- **狀態**：DONE
- **類型**：impl
- **建立時間**：2026-10-22 23:07 (UTC+8)
- **intervention_type**：fire-and-forget
- **對應 E2E**：`TestE2E_Template_MissingVariableReturns422`
- **depends_on**：T0004（模板 CRUD、渲染與 `ErrMissingVariable` 由 T0004 建立）
- **affects_files**：見 frontmatter

## 任務指令

### 前置條件
- 讀 `AGENTS.md`、`_ct-workorders/_decision-log.md`（D004）
- 讀 `stories.yaml` id "8"，spine 的 AD-8、AD-16，T0004 工單的回報區，`internal/httpapi/errors_templates.go`、`messages_send.go`
- 先跑 `make test`（Windows 需 `CGO_ENABLED=1`），確認開工前全綠

### 輸入上下文
帶 `template_id` 但缺變數時：`template.Render` 回 `ErrMissingVariable`（含變數名清單），`httpapi` 經 `writeError` 回 **422**、code `missing_template_variable`，message 列出缺少的變數名，且**不建立訊息**（AD-16：先渲染、成功才 `queue.Submit`）。對應填 `errors_templates.go` 的 `templateError`（回 `apiError`，不直接寫回應，AD-8）。message 只列變數名，不回顯其他變數的值或模板內容。若 T0004 的 `ErrMissingVariable` 還沒帶變數名清單，在 `template.go` 補上。

### 預期產出
- `errors_templates.go` 的 422 對應（保留 T0004 已有的 `ErrInvalidTemplate` → 400 對應，不改它）
- `e2e/template_missing_variable_test.go`：`TestE2E_Template_MissingVariableReturns422`——建立含兩個變數的模板，只給一個變數送信，斷言 422、信封 code、message 含缺少的變數名；之後查 store／`FakeProvider` 確認沒有建立訊息、沒有送出

### 驗收條件
- [ ] `go test ./... -race` 全綠（貼最後 20 行輸出；`-race` 無法執行時寫明原因與替代環境）
- [ ] `go vet ./...` 沒有輸出
- [ ] `gofmt -l .` 沒有輸出
- [ ] `git diff --name-only <base>..HEAD` 只出現本工單 `affects_files` 列出的檔案（`<base>` = 本工單開工時的 HEAD；若同時有別張工單在同一個工作目錄 commit，改用 `git show --name-only <本工單的每個 commit>` 逐個核對，並在回報區註明）
- [ ] 回報區填好、`status` 與 body 鏡像一致；產品 commit 與工單 commit 訊息都含工單編號
- [ ] `go test ./e2e -run 'TestE2E_Template_MissingVariableReturns422' -v` 通過
- [ ] 422 的 message 只含變數名；缺變數時沒有建立訊息（E2E 驗證）
- [ ] T0004 的 `TestE2E_Template_RendersVariables` 仍通過

### 不做
- 不新增 `go.mod` 的 `require`（D001）
- 不改另一張工單的 `affects_files`；發現需要改，寫進回報區〈遭遇問題〉交回塔台
- 不改 `_ct-workorders/` 底下的塔台私有檔（`_tower-state.md`、`_decision-log.md`…）
- 不 push；不改 `_bmad/`、`.claude/`；不用 `t.Skip`、不刪測試、不放寬斷言
- 不改 `e2e/architecture_test.go`（若它因新檔失敗，先回報，不要放寬）
- 不改 `internal/httpapi/templates.go`、`deps.go`、`ratelimit.go`、`errors_ratelimit.go`、`messages_list.go`、`internal/store/*`、`cmd/smsd/main.go`、`e2e/testutil/server.go`、`docs/openapi.yaml`（422 已在 openapi 以 `$ref` 宣告）
- 不改模板語法、不新增其他錯誤碼

## Sub-session 執行指示

> 由塔台以 yolo 模式在 BAT 派工（`/ct-exec T0007`）。
> 時間值一律用系統指令取得（`date "+%Y-%m-%dT%H:%M:%S%z"`），不要憑印象填。驗證指令成功跑完、看到輸出之後，才在回報區寫「通過」。暫存腳本寫在 `_bmad-output/tmp/`。

---

## 回報區

> 以下由 sub-session 填寫

### 完成狀態
DONE

### 產出摘要
- **Landing check**：PASS。C-0 `repo: sms-platform` = `basename(REPO_ROOT)` `sms-platform`；C-1 工單在 REPO_ROOT 下；C-3 前 5 筆中 4 筆存在、`e2e/template_missing_variable_test.go` 為新檔（父目錄 `e2e/` 存在）；C-2 工單無 `branch` 欄位（實際 `main`）。未設 `CT_HOME`（非 worktree）。`BAT_WORKSPACE_ID`=`<workspace>`（僅記錄）。
- **開工基準**：HEAD `24535d9`；開工前 `make test` 全綠（第一次因 PATH 無 gcc 失敗 `cgo: C compiler "gcc" not found`，照 T0004 回報把 `C:\ProgramData\mingw64\mingw64\bin` 加進 PATH、`CGO_ENABLED=1` 重跑全綠）。
- **`internal/httpapi/errors_templates.go`**：`templateError` 新增 `*template.MissingVariableError` → 422 `missing_template_variable`，message 為 `missing template variables: <名1>, <名2>`（只列變數名，依首次出現順序；不含模板內容與變數值）。另加 `errors.Is(err, ErrMissingVariable)` 但拿不到名單時的保底 422（message 不列名）。T0004 的 `ErrInvalidTemplate` → 400 `invalid_template` 未改。
- **`e2e/template_missing_variable_test.go`**：`TestE2E_Template_MissingVariableReturns422`——模板 `{{name}} 您好，訂單 {{order_id}} 已出貨`：
  - 只給 `order_id` → 422、code `missing_template_variable`、message 以 `: name` 結尾；回應不含 `A123`、`order_id`、模板文字、`{{`
  - 完全不給 vars → message 以 `: name, order_id` 結尾
  - 之後 `env.Store.Len() == 0`、`env.Provider.Sent()` 為空
- **未修改（不需要）**：`internal/template/template.go`（T0004 的 `MissingVariableError` 已帶 `Names`）、`internal/template/template_test.go`（`TestRenderMissingVariablesListsNames` 已覆蓋名單）、`internal/httpapi/messages_send.go`（已是先 `template.Render`、成功才 `queue.Submit`，AD-16）。

### 驗收證據
驗收在 `5c6bd1f` 的乾淨匯出（`git archive 5c6bd1f` 解到 scratchpad，排除共用工作目錄裡別張工單未提交的 `internal/store/store.go`）上執行；共用工作目錄另跑一次 `go test ./... -race -count=1` 也全綠。環境：Windows，Go 1.27.3，MinGW gcc 16.1.0，`CGO_ENABLED=1`。

- `gofmt -l .`：無輸出
- `go vet ./...`：無輸出（exit 0）
- `go build ./...`：exit 0
- `go test ./... -race -count=1`（exit 0，最後 20 行即全部輸出）：
```
?   	example.com/sms-platform/cmd/smsd	[no test files]
ok  	example.com/sms-platform/e2e	1.342s
?   	example.com/sms-platform/e2e/testutil	[no test files]
ok  	example.com/sms-platform/internal/httpapi	1.051s
?   	example.com/sms-platform/internal/provider	[no test files]
ok  	example.com/sms-platform/internal/provider/mock	1.068s
ok  	example.com/sms-platform/internal/queue	1.038s
?   	example.com/sms-platform/internal/ratelimit	[no test files]
ok  	example.com/sms-platform/internal/store	1.038s
ok  	example.com/sms-platform/internal/template	1.038s
ok  	example.com/sms-platform/internal/tenant	1.023s
?   	example.com/sms-platform/internal/webhook	[no test files]
```
- `go test ./e2e -race -count=1 -run 'TestE2E_Template_MissingVariableReturns422|TestE2E_Template_RendersVariables' -v`：
```
=== RUN   TestE2E_Template_MissingVariableReturns422
--- PASS: TestE2E_Template_MissingVariableReturns422 (0.00s)
=== RUN   TestE2E_Template_RendersVariables
--- PASS: TestE2E_Template_RendersVariables (0.00s)
PASS
ok  	example.com/sms-platform/e2e	1.037s
```
- **檔案範圍**：`git diff --name-only 24535d9..HEAD` 另含 `_ct-workorders/T0005-rate-limit.md`、`_ct-workorders/T0006-list-messages-filter-status.md`（T0005 `f27cee9`、T0006 `3f960c5` 同時在同一工作目錄 commit），故依工單改用 `git show --name-only` 逐個核對本工單 commit：
  - `5c6bd1f`：`e2e/template_missing_variable_test.go`、`internal/httpapi/errors_templates.go`
  - 工單 commit：只有 `_ct-workorders/T0007-template-missing-variable-422.md`
  皆在 `affects_files` 內。

### Commit
- 產品：`5c6bd1f` feat(T0007): 模板缺變數回 422 missing_template_variable（`git commit --only` 兩個檔案）
- 工單：本次收單 commit（訊息含 T0007），未 push

### 互動紀錄
無（yolo，未向塔台提問）

### Renew 歷程
無

### 遭遇問題
1. **共用工作目錄有別張工單的髒檔**：收單時 `internal/store/store.go` 有未提交修改（非本工單，未碰）、`_ct-workorders/_tower-state.md` 為塔台檔（未碰）。所以用乾淨匯出驗收 `5c6bd1f`，見〈驗收證據〉。
2. **FakeProvider 斷言目前恆成立**：`testutil.NewTestServer` 尚未把 `FakeProvider` 接到送信 worker（D005），`Provider.Sent()` 為空不能單獨證明沒送出；「沒建立訊息」實際由 `Store.Len() == 0` 證明。送信 worker 接上後此斷言自動生效。
3. **epics.md 驗收寫「列表中查不到」**：本工單指定查 store／FakeProvider，列表端點屬 T0006（同時進行中），未用列表驗證。
4. **環境**：Windows PATH 預設無 gcc，同 T0004 回報第 6 點。

### sprint-status.yaml 已更新
不適用

### 回報時間
2026-10-22T23:59:27+0800
