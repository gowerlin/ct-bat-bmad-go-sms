---
schema_version: 1
schema_kind: workorder
id: T0006
title: 訊息列表與 status 篩選（story id "7"）
type: impl
status: DONE
repo: sms-platform
project: sms-platform
created_at: "2026-10-22T23:07:32+0800"
started_at: "2026-10-22T23:57:38+0800"
updated_at: "2026-10-23T00:01:57+0800"
completed_at: "2026-10-23T00:01:57+0800"
sizing: M
depends_on:
  - T0004
affects_files:
  - internal/store/store.go
  - internal/store/store_test.go
  - internal/httpapi/messages_list.go
  - e2e/messages_list_test.go
  - docs/openapi.yaml
  - _ct-workorders/T0006-list-messages-filter-status.md
interaction:
  mode_hint: yolo
  interactive: false
  intervention_type: fire-and-forget
renew_count: 0
worker_question_timeout: 30
workdir: main repo
---
# 工單 T0006-list-messages-filter-status

## 元資料
- **工單編號**：T0006
- **任務名稱**：訊息列表與 status 篩選（`stories.yaml` id "7"）
- **狀態**：DONE
- **類型**：impl
- **建立時間**：2026-10-22 23:07 (UTC+8)
- **intervention_type**：fire-and-forget
- **對應 E2E**：`TestE2E_ListMessages_FilterByStatus`
- **depends_on**：T0004（T0004 會改 `internal/store/store.go` 與 `docs/openapi.yaml`，須先收單）
- **affects_files**：見 frontmatter

## 任務指令

### 前置條件
- 讀 `AGENTS.md`、`_ct-workorders/_decision-log.md`（D002、D004）
- 讀 `stories.yaml` id "7"，spine 的 AD-4、AD-14，`internal/httpapi/doc.go`，`internal/httpapi/messages_list.go`（stub）、`messages.go`（`messageResponse`／`toMessageResponse`，唯讀）
- 先跑 `make test`（Windows 需 `CGO_ENABLED=1`），確認開工前全綠

### 輸入上下文
`GET /v1/messages?status=…` 只回**該租戶**、符合 status 的訊息（AD-4：每個 store 查詢帶 `tenantID`）。非法 status 回 400 `invalid_request`（訊息不回顯輸入）。排序鍵 `(created_at, id)` 新到舊；cursor 是 `store` 產生的不透明字串（AD-14）。status 合法值見 `docs/openapi.yaml` 的 `listMessages`（`queued`、`sending`、`sent`、`delivered`、`undelivered`、`failed`）；沒帶 `status` 時回該租戶全部。

做法：`store.TenantScoped` 加一個租戶範圍的列表方法（篩選、排序、分頁），`Memory` 實作；路由填 `messages_list.go` 的 `registerMessagesList`（`GET /v1/messages`，用 `s.v1(s.handler(...))`），回應項目重用 `toMessageResponse`。分頁的頁大小預設與上限、cursor 的編碼若 spine 沒定，選最簡單的做法並在回報區寫明理由。補 `docs/openapi.yaml` 的 `listMessages` 回應（`MessageList`）。

### 預期產出
- `store` 列表方法與單元測試（篩選、排序、cursor、租戶隔離）
- `messages_list.go`
- `e2e/messages_list_test.go`：`TestE2E_ListMessages_FilterByStatus`——兩個租戶各送數則訊息，讓部分訊息進入不同 status（用 `FakeProvider`／`FakeClock`），斷言篩選結果只含自己租戶且符合 status、順序新到舊、非法 status 回 400 信封、跨租戶看不到對方訊息

### 驗收條件
- [ ] `go test ./... -race` 全綠（貼最後 20 行輸出；`-race` 無法執行時寫明原因與替代環境）
- [ ] `go vet ./...` 沒有輸出
- [ ] `gofmt -l .` 沒有輸出
- [ ] `git diff --name-only <base>..HEAD` 只出現本工單 `affects_files` 列出的檔案（`<base>` = 本工單開工時的 HEAD；若同時有別張工單在同一個工作目錄 commit，改用 `git show --name-only <本工單的每個 commit>` 逐個核對，並在回報區註明）
- [ ] 回報區填好、`status` 與 body 鏡像一致；產品 commit 與工單 commit 訊息都含工單編號
- [ ] `go test ./e2e -run 'TestE2E_ListMessages_FilterByStatus' -v` 通過
- [ ] 列表查詢每一步都帶 `tenantID`；排序鍵 `(created_at, id)` 新到舊
- [ ] `TenantScoped` 新增方法後，所有實作與測試替身都編譯通過

### 不做
- 不新增 `go.mod` 的 `require`（D001）
- 不改另一張工單的 `affects_files`；發現需要改，寫進回報區〈遭遇問題〉交回塔台
- 不改 `_ct-workorders/` 底下的塔台私有檔（`_tower-state.md`、`_decision-log.md`…）
- 不 push；不改 `_bmad/`、`.claude/`；不用 `t.Skip`、不刪測試、不放寬斷言
- 不改 `e2e/architecture_test.go`（若它因新檔失敗，先回報，不要放寬）
- 不改 `internal/httpapi/messages.go`、`messages_send.go`、`messages_get.go`、`deps.go`、`ratelimit.go`、`templates.go`、`cmd/smsd/main.go`、`e2e/testutil/server.go`（本單不需要新的組裝；若發現需要，回報 PARTIAL 交回塔台）
- 不做 `to`、時間區間等其他篩選條件

## Sub-session 執行指示

> 由塔台以 yolo 模式在 BAT 派工（`/ct-exec T0006`）。
> 時間值一律用系統指令取得（`date "+%Y-%m-%dT%H:%M:%S%z"`），不要憑印象填。驗證指令成功跑完、看到輸出之後，才在回報區寫「通過」。暫存腳本寫在 `_bmad-output/tmp/`。

---

## 回報區

> 以下由 sub-session 填寫

### 完成狀態
DONE

### 產出摘要
- **landing check**：PASS。C-0 `repo: sms-platform` = `basename(REPO_ROOT)` `sms-platform`（REPO_ROOT `<demo>/sms-platform`）；C-1 工單在 REPO_ROOT 下；C-3 前 5 個 `affects_files` 皆存在或其父目錄存在；C-2 無 `branch` 欄位。`CT_HOME` 未設（非 worktree，`workdir: main repo`）；`BAT_WORKSPACE_ID` = `<workspace>`（僅記錄）。
- **`internal/store/store.go`**：
  - `TenantScoped` 新增 `ListMessages(tenantID string, f MessageFilter) (MessagePage, error)`；`Memory` 實作。空 `tenantID` 回 `ErrEmptyTenant`；只掃 `m.TenantID == tenantID` 的訊息（AD-4），`f.Status` 空字串表示全部狀態。
  - 排序 `(CreatedAt, ID)` 新到舊；同一 `CreatedAt` 以較大的 `ID` 在前（與 `ListTemplates` 的平手規則一致）。
  - 分頁：`DefaultListLimit = 50`、`MaxListLimit = 100`（`Limit <= 0` 用預設、超過上限截到 100）。多取一筆判斷是否有下一頁，`NextCursor` 是本頁最後一筆的位置，最後一頁為 `""`。
  - cursor：`base64.RawURLEncoding("<CreatedAt UnixNano>:<ID>")`，呼叫端視為不透明（AD-14）；解不開回 `ErrInvalidCursor`。cursor 只是位置，查詢仍以 `tenantID` 限定，拿別租戶的 cursor 也看不到別租戶的資料（單元測試涵蓋）。
  - `Status.Valid()`：列舉六個合法值，供 httpapi 驗 `?status=`（status 篩選屬 store 依 `doc.go`）。
- **`internal/httpapi/messages_list.go`**：`registerMessagesList` 註冊 `GET /v1/messages`，`s.v1(s.handler(s.listMessages))`。`?status=` 有帶就必須合法（空字串、大小寫不同都回 400），`?limit=` 須為 1–100 的整數，`?cursor=` 無效回 400；三者都是 `invalid_request`，訊息固定、不回顯輸入。回應 `{"messages":[…],"next_cursor":"…"}`，項目重用 `toMessageResponse`，`next_cursor` 最後一頁省略，`messages` 無資料時為 `[]` 而非 `null`。`ErrInvalidCursor` 在 handler 內轉成 `invalid(...)`，不需要改 `errors.go`。
- **`internal/store/store_test.go`**：新增 `TestListMessagesFiltersByTenantAndStatus`、`TestListMessagesTiesBreakOnHigherID`、`TestListMessagesCursorPaging`（逐頁走完、cursor 不含 id 明文、第一頁後新增訊息不影響後續頁、跨租戶 cursor 不洩漏）、`TestListMessagesLimitsAndBadCursor`（預設 50、上限 100、4 種無效 cursor）、`TestStatusValid`。
- **`e2e/messages_list_test.go`**：`TestE2E_ListMessages_FilterByStatus`——A、B 兩租戶交錯送 6 則（每則後 `FakeClock.Advance(1s)`），A 的 2 則 sent、1 則 failed，B 的 1 則 sent；斷言各 status 篩選只含自己租戶且狀態相符、`created_at` 新到舊且 id 順序正確、failed 帶 `failure_reason`、無 status 回全部、B 的列表不含 A 的任何 id、`limit`＋`cursor` 分頁、非法 status（`bogus-status`、空字串、`SENT`）回 400 信封且不回顯、非法 `limit`／`cursor` 回 400、沒帶 key 回 401。
- **`docs/openapi.yaml`**：`listMessages` 補 `limit`、`cursor` 參數與描述、200 回 `MessageList`、加 400；`components.schemas` 新增 `MessageList`（`messages` 必填、`next_cursor` 選填），TODO 註解移除 `MessageList`。以 `uv run --offline --with pyyaml` 解析通過。

#### 分頁與 cursor 的選擇理由（spine 沒定）
- 頁大小預設 50、上限 100：常見值，100 則 × DTO 約 20KB 內，記憶體與回應大小都有界；超過上限在 HTTP 層回 400（明確），在 store 層截斷（防呆）。
- cursor 編碼選「最後一筆的 `(created_at, id)` 做 base64url」：無狀態、不必在伺服器存游標，與排序鍵直接對應；新訊息插入不會讓後續頁重複或跳號。不加簽章／加密：cursor 只決定位置、查詢仍受 `tenantID` 限定，竄改最多只會跳到同租戶的另一個位置；日後要換 DB 只需改 `encodeCursor`／`decodeCursor`，契約仍是不透明字串。
- 不把 `status` 綁進 cursor：換 status 時沿用舊 cursor 也只是從該位置往後列，不會出錯；保持最簡單。

#### 驗收證據
環境：Windows 主機，`go version go1.27.3 windows/amd64`，MinGW gcc（`C:\ProgramData\mingw64\mingw64\bin` 加進 PATH），`CGO_ENABLED=1`。
- 開工前：base `24535d9` 以 `git archive` 匯出到 `_bmad-output/tmp/T0006-base` 跑 `go test ./... -race -count=1` 全綠（開工 commit 後才補跑，store 已開始改，故用匯出避開工作目錄的改動；匯出目錄已刪）。
- 收單門檻在**本單產品 commit `a5ab662` 的乾淨匯出**（`git archive a5ab662` → `_bmad-output/tmp/T0006-head`，已刪）上跑，排除 T0005 在同一工作目錄尚未 commit 的改動：
  - `go build ./...`：exit 0
  - `go vet ./...`：沒有輸出，exit 0
  - `gofmt -l .`：沒有輸出，exit 0
  - `go test ./... -race -count=1` 最後 20 行：
    ```
    ?   	example.com/sms-platform/cmd/smsd	[no test files]
    ok  	example.com/sms-platform/e2e	1.327s
    ?   	example.com/sms-platform/e2e/testutil	[no test files]
    ok  	example.com/sms-platform/internal/httpapi	1.044s
    ?   	example.com/sms-platform/internal/provider	[no test files]
    ok  	example.com/sms-platform/internal/provider/mock	1.060s
    ok  	example.com/sms-platform/internal/queue	1.029s
    ?   	example.com/sms-platform/internal/ratelimit	[no test files]
    ok  	example.com/sms-platform/internal/store	1.030s
    ok  	example.com/sms-platform/internal/template	1.029s
    ok  	example.com/sms-platform/internal/tenant	1.029s
    ?   	example.com/sms-platform/internal/webhook	[no test files]
    ```
  - `go test ./e2e -run 'TestE2E_ListMessages_FilterByStatus' -race -count=1 -v`：`--- PASS: TestE2E_ListMessages_FilterByStatus (0.01s)`、`ok example.com/sms-platform/e2e`
- 同時也在共用工作目錄（含 T0005 未 commit 的 `internal/ratelimit` 等改動）跑過一次 `go build`／`go vet`／`gofmt -l .`／`go test ./... -race -count=1`，全部乾淨、全綠。
- 檔案範圍：同一工作目錄有 T0005、T0007 與塔台的 commit，`git diff --name-only 24535d9..HEAD` 混有他單檔案，改用逐 commit 核對：
  - `3f960c5 chore(T0006): 開工` → `_ct-workorders/T0006-list-messages-filter-status.md`
  - `a5ab662 feat(T0006): …` → `docs/openapi.yaml`、`e2e/messages_list_test.go`、`internal/httpapi/messages_list.go`、`internal/store/store.go`、`internal/store/store_test.go`
  - 全部在 `affects_files` 內；未動 `messages.go`、`errors.go`、`deps.go`、`e2e/testutil/server.go`、`architecture_test.go` 等排除檔；`go.mod` 未改。
- `TenantScoped` 只有 `*store.Memory` 一個實作（`grep -rn TenantScoped`：其餘只有 `httpapi/deps.go`、`template.Service` 的使用端），沒有測試替身，全部編譯通過。
- 列表每一步帶 `tenantID`：handler 以 `tenantID(r.Context())` 呼叫 `ListMessages`，store 空 id 回錯、迴圈只收 `m.TenantID == tenantID`。

#### Commit
- 產品：`a5ab662b0e16fe2fce5db84facf908373ee42b0f` `feat(T0006): 訊息列表與 status 篩選（GET /v1/messages）`（`git commit --only` 指定 5 個檔案，未帶入他單的髒檔）
- 工單：開工 `3f960c5`；回報 commit 見本次工單 commit（訊息 `chore(T0006): 回報 DONE`）
- 未 push。

### 互動紀錄
無（未向塔台提問）

### Renew 歷程
無

### 遭遇問題
1. **E2E 的狀態轉移不經 queue**：工單要「用 `FakeProvider`／`FakeClock` 讓部分訊息進入不同 status」，但 queue 送信 worker 尚未實作、`testutil` 也沒把 `FakeProvider` 接進 queue（與 D005 同一原因）。E2E 以 helper `deliver` 模擬送信 worker：`queued → sending` 後呼叫 `env.Provider.Send`（以 `Enqueue` 腳本化 `provider.ErrPermanent`），依結果經 `env.Store`（`store.System`）轉成 `sent`（帶 `ProviderRef`）或 `failed`（帶 `failure_reason`）。列表本身仍以真實 handler＋`httptest` 驗證。送信 worker 上線後，這個 helper 可換成讓 queue 實際送出。未改 `testutil`（排除檔）。
2. **並行工單共用工作目錄**：T0005 的未 commit 改動與 T0007、塔台的 commit 同時出現在 `main`。驗收改在本單 commit 的乾淨匯出上跑，檔案範圍逐 commit 核對（見驗收證據）。
3. **環境**：`go test -race` 第一次因 PATH 沒有 gcc 失敗（`cgo: C compiler "gcc" not found`），把 `C:\ProgramData\mingw64\mingw64\bin` 加進 PATH 後正常（T0004 已記錄同一問題）。
4. 開工前 `make test` 未在動手前跑；之後以 base `24535d9` 的乾淨匯出補跑，全綠。

### sprint-status.yaml 已更新
不適用

### 回報時間
2026-10-23T00:01:57+0800
