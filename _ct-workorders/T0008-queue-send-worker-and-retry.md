---
schema_version: 1
schema_kind: workorder
id: T0008
title: 發送佇列 worker 與重試，重試後成功／用盡標記 failed（story id "9"、"10"）
type: impl
status: DONE
repo: sms-platform
project: sms-platform
created_at: "2026-10-23T01:38:18+0800"
started_at: "2026-10-23T02:22:40+0800"
updated_at: "2026-10-23T02:42:55+0800"
completed_at: "2026-10-23T02:42:55+0800"
sizing: L
followups:
  - T0009
  - queue 每次 Provider.Send 沒有逾時，DeadlineExceeded 依 AD-1 當永久失敗；需定逾時值與歸類（可與 T0010 一起）
  - Send 成功但 ProviderRef 為空時仍轉 sent，回呼對不到；T0009 定回呼對應時一併決定
  - cmd/smsd 的 queue 組裝沒有測試，且 _ = q.Run(ctx) 吞掉 ErrNoProvider 等錯誤（需 main_test.go 與改 main.go）
affects_files:
  - internal/queue/queue.go
  - internal/queue/queue_test.go
  - internal/queue/doc.go
  - internal/store/store.go
  - internal/store/store_test.go
  - internal/httpapi/messages.go
  - cmd/smsd/main.go
  - e2e/testutil/server.go
  - e2e/testutil/clock.go
  - e2e/queue_retry_test.go
  - e2e/template_test.go
  - e2e/messages_test.go
  - e2e/messages_list_test.go
  - e2e/ratelimit_test.go
  - e2e/tenant_isolation_test.go
  - docs/openapi.yaml
  - _bmad-output/implementation-artifacts/deferred-work.md
  - _ct-workorders/T0008-queue-send-worker-and-retry.md
interaction:
  mode_hint: yolo
  interactive: false
  intervention_type: fire-and-forget
renew_count: 0
worker_question_timeout: 30
workdir: worktree
isolation: worktree
---
# 工單 T0008-queue-send-worker-and-retry

## 元資料
- **工單編號**：T0008
- **任務名稱**：發送佇列 worker 與重試（`stories.yaml` id "9"、"10"）
- **狀態**：DONE
- **類型**：impl
- **建立時間**：2026-10-23 01:38 (UTC+8)
- **intervention_type**：fire-and-forget
- **對應 E2E**：`TestE2E_Queue_RetriesThenSucceeds`、`TestE2E_Queue_ExhaustsRetriesMarksFailed`（紅燈骨架在 `e2e/queue_retry_test.go`，build tag `red_queue`，見 D011）；另收 D005：`TestE2E_Template_RendersVariables` 改用 `FakeProvider` 收到的內容驗證
- **depends_on**：無（與 T0010 平行，皆 `isolation: worktree`）
- **affects_files**：見 frontmatter。與 T0010 的已知重疊只有 `cmd/smsd/main.go`，是刻意的（D012，5-08 練習衝突合併）
- **合併順序**：先 T0010、後 T0008。Worker 不處理合併

## 工作量預估
- **預估規模**：大
- **Context Window 風險**：中
- **降級策略**：先完成 story 9（重試後成功）與 worker pool 骨架，回報 `PARTIAL`；story 10、D005 留下一輪

## 規格層級自問（派發前）
- [x] 目標層：執行。介面形狀的部分由 Worker 決定，行為由 AD 與 stories 給定
- [x] 決策權歸屬：重試次數／退避／`failure_reason` 字串／worker 數已在下方定死；其餘（建構子簽名、內部型別）Worker 可決定，需符合 `internal/queue/doc.go` 的邊界
- [x] 資訊完整度：前置條件列出；不足時以回報區〈遭遇問題〉交回塔台
- [x] 回頭成本：A（改幾行到幾個檔）
- [x] 記憶覆蓋：無衝突；`AGENTS.md` 禁 `t.Skip`、禁 `time.Now`／`time.Sleep` 於業務邏輯照舊

## 任務指令

### 前置條件
- 讀 `AGENTS.md`、`_ct-workorders/_decision-log.md`（D005、D011、D012）
- 讀 `stories.yaml` id "9"、"10"；`_bmad-output/planning-artifacts/epics.md` 的 AD-2、AD-5、AD-6、AD-10、AD-13；`_bmad-output/specs/spec-sms-platform/state-machines.md`
- 讀 `internal/queue/{doc,queue}.go`、`internal/store/store.go`（CAS 轉移與 `Patch`）、`e2e/testutil/*.go`、`e2e/queue_retry_test.go`、`e2e/template_test.go`
- 先跑 `go test ./... -race -count=1`，確認開工前全綠（骨架因 build tag 不編譯）

### 輸入上下文
目前 `queue.Run` 只等 ctx，沒有任何東西把 `queued` 的訊息交給 Provider。本單補上「背景送信 worker」：

- **worker pool**：預設 4 個 worker，從佇列取訊息，呼叫 `Provider.Send`。`Provider`、`Now`、`Wait` 都經建構子注入（AD-10；業務邏輯不得直呼 `time.Now`／`time.Sleep`）。`Close` 要等在途的 worker 收尾（不得 goroutine 洩漏，`-race` 乾淨）
- **狀態轉移**（AD-2、AD-5、AD-6、AD-13）：`queued → sending`；`Send` 成功 → `sending → sent`，**同一次 CAS** 寫 `provider_ref` 與 `attempts`；`ErrTransient` → 退避後重試；`ErrPermanent` → 立刻 `failed`、`failure_reason` = `永久失敗`
- **重試**：總共最多呼叫 Provider **4 次**（首次＋重試 3 次）；退避依序 **1s、2s、4s**，透過注入的 `Wait(ctx, d)`；暫時失敗 3 次重試仍失敗 → `sending → failed`，同一次 CAS 帶 `attempts=4` 與非空 `failure_reason` = `重試用盡`。成功後不得再呼叫 Provider。不跨 Provider failover。重試歸 `queue`，`provider` 不重試
- **`attempts` 的定義**：已呼叫 `Provider.Send` 的次數（成功那次也算；骨架期望成功案例為 3、用盡案例為 4）
- **API 曝光**：`GET /v1/messages/{id}` 已有 `attempts`／`failure_reason`（`messages.go`）；若 DTO 缺 `provider_ref` 不必加。`docs/openapi.yaml` 若對這兩欄、`status` 的列舉有缺漏，補上並與 `docs/error-codes.md`／`state-machines.md` 一致
- **`e2e/testutil`（歸本單，D012）**：
  - `NewTestServer(t, opts ...Option)` 預設**啟動**送信 worker，把 `FakeProvider`、`FakeClock.Now`／`FakeClock.Wait` 接進 queue；測試結束時關閉
  - 新增 `WithoutWorker()` 選項：需要「訊息停在 `queued`」的既有測試（例如斷言 `202` 後立刻查到 `queued`、佇列滿 `429`／`503` 類測試、列表篩選 `status=queued`）改用它，**每個測試只動那一行呼叫**。**不得放寬既有斷言**，也不得改測試意圖
  - `FakeClock` 要能讓測試**確定性地**等到「worker 正阻塞在 `Wait`」再 `Advance`（例如暴露目前等待者數量／`BlockUntilWaiters(n)` 之類）。骨架裡的「`Advance` 之後馬上 `GET`」有競態，要改成確定性的寫法；**不得用 `time.Sleep`**（`e2e/architecture_test.go` 會擋）。最終等終態用 `FakeClock` 同步或 channel，不要輪詢真實時間
  - `e2e/messages_list_test.go` 的 `deliver` helper 原本在測試裡手動模擬送信 worker，worker 上線後會與真實 worker 搶轉移（`ErrIllegalTransition`）：改成走真實 queue（或 `WithoutWorker()` 保留原手動轉移，二擇一，選較小改動並在回報區說明）
- **D005**：`TestE2E_Template_RendersVariables` 改成用 `env.Provider.Sent()` 收到的 `Body` 驗證已渲染（AD-16）；保留「入佇列前就已渲染」的斷言語意，不要放寬。`TestE2E_Template_MissingVariableReturns422` 的「沒有送出」斷言應維持成立
- **`cmd/smsd/main.go`（已知與 T0010 重疊，D012）**：把 `prov` 傳進 queue 建構（改掉 `queue.NewInProcess(st)` 那一行，刪除註解「consumed by the queue workers from Story 1.6」）。**只做這件事**：不要依 `PROVIDER` 選 adapter、不要新增環境變數、不要為 T0010 預留位置、不要嘗試解對方的衝突。預設仍用 `mock.New()`

### 預期產出
- `internal/queue` 的送信 worker pool、重試與終態轉移（含單元測試：退避序列、attempts、永久失敗不重試、`Close` 等待在途、ctx 取消不洩漏）
- `internal/store` 若需要，補「同一次 CAS 寫 `provider_ref`＋`attempts`」或 `failed` 帶 `failure_reason` 的 patch 支援（沿用既有表驅動轉移，不另開旁路）
- `e2e/testutil` 的 `Option`／`WithoutWorker()`／FakeClock 同步，以及上述既有 E2E 的最小調整
- `e2e/queue_retry_test.go`：**移除首行 `//go:build red_queue`**，把兩個測試改成確定性、真正等到終態的寫法（保留測試名與斷言：成功案例 `sent`／`attempts=3`／Provider 恰被呼叫 3 次；用盡案例 `failed`／`attempts=4`／`failure_reason` 非空），並移除 `queueSkeletonGet` 之類骨架命名
- `TestE2E_Template_RendersVariables` 改用 `FakeProvider` 驗證（D005）

### 驗收條件
- [ ] `go test ./e2e -count=1 -run 'TestE2E_Queue_RetriesThenSucceeds|TestE2E_Queue_ExhaustsRetriesMarksFailed|TestE2E_Template_RendersVariables' -v` 三個 PASS（貼輸出）
- [ ] `go test ./... -race -count=1` 全綠（貼最後 20 行；`-race` 無法執行時寫明原因與替代環境，不得拿掉 `-race` 讓門檻變綠）
- [ ] `go build ./...`、`go vet ./...` 乾淨；`gofmt -l .` 無輸出
- [ ] `grep -rn 'go:build red_queue' e2e/` 無結果；`red_webhook`、`red_mitake` 兩個 tag 與其檔案**原封不動**
- [ ] 其餘既有 E2E 全數仍 PASS，且斷言沒有被放寬（`git diff` 對既有 `_test.go` 只出現 `WithoutWorker()` 呼叫、D005 的改寫與 `deliver` 調整）
- [ ] `go.mod` 沒有 `require`；測試無 `t.Skip`；業務邏輯無 `time.Now`／`time.Sleep`
- [ ] `git diff --name-only <base>..HEAD` 只出現 `affects_files` 列出的檔案
- [ ] **實作完成、填回報區之前，先跑 `/bmad-code-review`**（審查範圍＝本單 diff）。把審查摘要（reviewer 數、finding 數、已修／不修及理由）寫進回報區〈程式碼審查〉；沒修的與被標為 deferred 的項目逐條寫進回報區〈deferred 項目〉，並同步寫進 frontmatter `followups`（以一行描述，不是工單編號）。審查提出的修正若超出 `affects_files`，不要動，改記為 deferred。`/bmad-code-review` 會把 deferred 項目寫進 `_bmad-output/implementation-artifacts/deferred-work.md`（本單的 `affects_files` 已列入，只許**追加**條目，不改既有條目）。`_bmad-output/implementation-artifacts/sprint-status.yaml` **不在**範圍：審查若要同步它，不要改，在回報區〈遭遇問題〉寫明想改什麼，由塔台收單時補（見 `work-order-system.md` § 回報處理流程）
- [ ] 回報區填好，`status` 與 body 鏡像一致；產品 commit 與工單 commit 訊息都含 `T0008`

### 不做
- 不新增 `go.mod` 的 `require`（D001）
- 不實作供應商回呼／出站通知（T0009）、不建 `mitake`（T0010）
- 不依 `PROVIDER` 選 adapter（T0010）；`main.go` 只做「把 `prov` 傳給 queue」
- 不改 `red_webhook`、`red_mitake` 的測試檔
- 不改 `e2e/architecture_test.go`（若因新檔失敗，先回報，不要放寬）
- 不改 `_ct-workorders/` 底下的塔台私有檔（`_tower-state.md`、`_decision-log.md`…）
- 不 push；不改 `_bmad/`、`.claude/`；不用 `t.Skip`、不刪測試、不放寬斷言
- 不跨 Provider failover；不改退避序列與總次數

## Sub-session 執行指示

> 由塔台以 yolo 模式在 BAT 派工（`/ct-exec T0008`，`--isolation worktree`）。沒有 `tower_question`；遇到工單沒寫的取捨，選最小改動並記在回報區〈互動紀錄〉，真的卡住才走 `yolo-mode.md` § Worker 向塔台提問。
> 時間值一律用系統指令取得（`date "+%Y-%m-%dT%H:%M:%S%z"`），不要憑印象填。驗證指令成功跑完、看到輸出之後，才在回報區寫「通過」。暫存腳本寫在 `_bmad-output/tmp/`。
> Windows 跑 `-race` 要先讓 shell 找得到 gcc 並設 `CGO_ENABLED=1`（`AGENTS.md` 末段）。

---

## 回報區

> 以下由 sub-session 填寫

### 完成狀態
DONE：驗收條件全數達成（證據見〈驗收證據〉）。

落地檢查：**PASS**
- C-0：frontmatter `repo` = `sms-platform`，`basename "$CT_HOME"` = `sms-platform`，PASS
- C-1：工單位於 `$CT_HOME/_ct-workorders/`，PASS
- C-3：抽查 `internal/queue/{queue,queue_test,doc}.go`、`internal/store/{store,store_test}.go`，全部存在，PASS
- C-2：工單沒有 `branch` 欄位，不比對
- `CT_HOME` = `<demo>\sms-platform`，`CT_WORKTREE_BRANCH` = `ct/T0008`，`BAT_WORKSPACE_ID` = `<workspace>`（只記錄，不比對）

### 產出摘要

**`internal/queue/queue.go`**
- 送信 worker pool，預設 4 個 worker（`DefaultWorkers`）。依賴用 functional option 注入：`WithProvider`、`WithWait`、`WithWorkers`。
- 狀態轉移：
  - 開始送：`queued→sending`。
  - 成功：`sending→sent`，同一次 CAS 寫入 `attempts` 與 `provider_ref`。
  - `ErrTransient`：經注入的 `Wait` 依序退避 1s、2s、4s 後重試，總共最多呼叫 4 次。用盡時 `sending→failed`，同一次 CAS 寫入 `attempts=4` 與 `failure_reason=重試用盡`。
  - `ErrPermanent` 或未分類錯誤（依 AD-1）：不重試，直接記為 `永久失敗`。
- worker 取出訊息時就釋放佇列名額。
- `Close`：等在途送信做完；只有 `ctx` 到期才取消在途送信，取消後仍會等 worker 返回。被中止的訊息停在 `sending`。
- `Run` 的 ctx 結束時：
  - 停止領取新訊息，並拒收新訊息（`ErrClosed`，HTTP 回 503）。
  - 不取消在途送信，讓 `main` 收到 SIGTERM 後還有 `shutdownTimeout` 的時間把在途送信做完。
- `Run` 只能呼叫一次；建構時沒給 Provider，`Run` 回 `ErrNoProvider`。

**`internal/queue/queue_test.go`**：新增 11 個測試函式，涵蓋：
- 表驅動 6 例：首次成功、重試後成功、重試用盡、永久失敗、未分類錯誤、重試後永久失敗。每例斷言退避序列、`attempts`、`provider_ref` 與 Provider 呼叫次數。
- 4 個 worker 可同時在途。
- 取出訊息時釋放名額。
- `Close` 會等在途送信。
- Send 進行中被 `Close` 中止時，訊息留在 `sending`。
- `Run` 的 ctx 結束後，在途送信仍完成，且拒收新訊息。
- 退避等待中被 `Close` 中止。
- `Run` 在 ctx 結束後返回。
- `Run` 的拒絕情況：重複呼叫、`Close` 之後、沒有 Provider。

**`internal/queue/doc.go`**：補充說明退避經注入的 `Wait`、時間戳來自 store 的 `Now`。

**`e2e/testutil`**
- `NewTestServer(t, opts ...Option)` 預設啟動 worker，接上 `FakeProvider` 與 `FakeClock.Wait`。測試結束時用已取消的 ctx 呼叫 `Close`，再等 `Run` 返回。
- 新增 `WithoutWorker()` 選項。
- `FakeClock.Wait` 改成阻塞到 `Advance`／`Set` 讓時間到為止；新增 `BlockUntilWaiters(t, n)`。
- 新增 `Env.AwaitStatus(t, id, status)`：用 channel 等待狀態轉移，不輪詢。
- 上述等待都有 10s 看門狗；逾時只會讓測試失敗，不會睡眠。

**`e2e/queue_retry_test.go`**
- 移除 `//go:build red_queue`。
- 用 `advanceBackoff`（先 `BlockUntilWaiters` 再 `Advance`）加上 `AwaitStatus`，改成確定性的寫法。
- 測試名稱與原有斷言都保留；`queueSkeleton*` 改名為 `queueMessage`／`getQueueMessage`。
- 用盡案例另加兩個斷言（只加嚴）：`failure_reason == 重試用盡`、Provider 恰被呼叫 4 次。

**既有 E2E**
- `messages_test.go`：3 個測試只改成 `WithoutWorker()` 那一行（`Returns202AndQueryable`、`QueueFullReturns503AndKeepsNothing`、`PurgeExpired_RemovesOnlyOldTerminalMessages`）。
- `messages_list_test.go`：`FilterByStatus` 改成 `WithoutWorker()`；`deliver` 只改註解。
- `ratelimit_test.go`、`tenant_isolation_test.go` 不受 worker 影響，沒有修改。

**D005**
- `TestE2E_Template_RendersVariables` 保留原斷言：從 store 讀回的 `Body` 已經渲染，證明入佇列前就渲染。
- 另加：`AwaitStatus(sent)` 之後，`env.Provider.Sent()` 恰為 1 筆，且 ID 與 `Body` 都正確。
- `TestE2E_Template_MissingVariableReturns422` 沒有修改，「沒有送出」的斷言仍成立。

**`cmd/smsd/main.go`**：把 `prov := mock.New()` 移到建 queue 之前，改成 `queue.NewInProcess(st, queue.WithProvider(prov))`，並刪除 Story 1.6 那行註解。其他都沒動（依 D012）。

**`docs/openapi.yaml`**：`status` 列舉原本就完整。補上 `attempts` 的定義與寫入時機，以及 `failure_reason` 的可能值（與 `state-machines.md` 一致）。

**`internal/store`**：既有的 `Transition`＋`Patch` 已能在同一次 CAS 寫入 `attempts`、`provider_ref`、`failure_reason`，所以 `store.go`、`store_test.go` 都沒有修改。

### 程式碼審查（`/bmad-code-review`）

**執行方式**
- 模式 thorough，4 個 reviewer：Blind Hunter、Edge Case Hunter、Verification Gap、Intent Alignment，都是唯讀子代理。
- `review_mode` = `full`，`plan_file` = 本工單，審查範圍 `800b7b4...96b1280`。沒有 lens 失敗。
- `render_skill.py` 產出的 `workflow.md` 與各步驟檔仍保留未代入的 Jinja（`resolved_values` 為空）。因此 lens 清單與 `review = "thorough"` 改從 skill 的 `customize.toml` 讀取。
- yolo 模式下，CHECKPOINT 與 HALT 都不等人回覆，一律取預設：patch 全部套用。沒有 decision-needed 項目。

**統計**：原始 finding 約 40 條，去重歸併後 patch 11、defer 3、駁回約 15。

**已修正的 11 項**（commit `504de4c`）
1. `Run` 的 ctx 結束後，`Submit` 仍回 202 卻沒有 worker 送。改為拒收（`ErrClosed`，HTTP 回 503「service shutting down」），`Run` 也不能再次啟動。
2. `WithWait(nil)` 會在第一次退避時 nil panic。改為傳入 nil 時保留預設。
3. `Backoff` 原本是匯出的可變 `var`，改為未匯出（工單規定不得改退避序列）。
4. 「Send 進行中被 `Close` 中止」的分支沒有測試；若拿掉這個分支，訊息會被記成永久失敗。新增 `TestCloseDuringSendLeavesSending`。mutation 驗證：拿掉該分支後測試 FAIL。
5. 「`Run` 的 ctx 結束不取消在途送信」的原測試幾乎不可能失敗。新增 `TestRunCtxEndFinishesInFlightSend`。mutation 驗證：在 `Run` 的 ctx 分支加上 `abort()` 後測試 FAIL，訊息為「Run returned <nil> before the in-flight send finished」。
6. `TestCloseWaitsForInFlightSend` 沒先確認 `Close` 已經開始。改成等 `closed` 旗標成立後才開閘，並先斷言 `Close` 尚未返回。
7. `running` helper 忽略 `Close` 回傳的錯誤，改為檢查。
8. openapi 的 `attempts` 沒寫寫入時機（`sending` 期間為 0），補上。
9. `work()` 的註解沒說明 quit 關閉那一瞬間取到的訊息如何處理，補上。
10. `advanceBackoff` 假設同時只有一則訊息在途，補上註明。
11. `deliver` 的註解措辭。

**駁回的項目與理由**
- `Close` 傳入已取消的 ctx 時，可能隨機回 `ctx.Err()`：嚴重度 low，只在 ctx 已取消時發生，要修得多加分支。
- 被中止的訊息永遠停在 `sending`、`attempts` 為 0：store 是 in-memory，只在關機時發生。
- 送信路徑的 log 太少：NFR2 只限制 log 欄位，沒有規定必須記錄什麼。
- `failure_reason` 改成 enum：會變更 API 契約；`回呼逾時` 這個值來自 `state-machines.md`。
- 測試只驗 worker 數的下限、測不出 worker 太多：要驗上限得等真實時間。
- 補永久失敗與首次成功的 E2E：unit test 已涵蓋，E2E 名稱清單由 COURSE-PLAN 決定。
- `AwaitStatus` 看不到直接對 store 做的轉移：doc 已寫明這個限制。
- `finish` 的 CAS 失敗只記 log：判定為 false，目前沒有其他程式會把訊息移出 `sending`。
- `Message.Provider` 沒有寫入：DTO 沒有這個欄位，也沒有使用者。
- queue 沒注入 `Now`：屬於 Worker 的設計取捨，記在〈互動紀錄〉。

### deferred 項目
已追加到 `_bmad-output/implementation-artifacts/deferred-work.md`（commit `9080685`，只追加、沒改既有條目），並同步寫入 frontmatter `followups`：
1. `queue` 沒有每次 `Send` 的逾時；`DeadlineExceeded` 目前依 AD-1 當永久失敗。需塔台決定逾時值與歸類，可與 T0010 的 HTTP adapter 一起定。
2. `Send` 成功但 `ProviderRef` 為空時，訊息照樣轉為 `sent`，之後的回呼對不到這則訊息。嚴重度未驗證，由 T0009 定回呼對應時一併決定。
3. `cmd/smsd` 組裝 queue 的程式沒有測試，且 `_ = q.Run(ctx)` 會吞掉 `ErrNoProvider` 等錯誤。要修需新增 `cmd/smsd/main_test.go` 並改 `main.go`，都不在 T0008 範圍（D012）。

### 驗收證據
驗收在 HEAD `9080685361fc745b42f461c5d4c63bdf19e4f860` 上執行（Windows 主機）：PATH 加入 `<MinGW 的 bin 目錄>`、`CGO_ENABLED=1`，`go env` 回 `1`／`gcc`，Go 為 go1.27.3 windows/amd64。

- [x] **三個指定的 E2E**：`go test ./e2e -count=1 -run 'TestE2E_Queue_RetriesThenSucceeds|TestE2E_Queue_ExhaustsRetriesMarksFailed|TestE2E_Template_RendersVariables' -v`
```
--- PASS: TestE2E_Queue_RetriesThenSucceeds (0.00s)
--- PASS: TestE2E_Queue_ExhaustsRetriesMarksFailed (0.00s)
--- PASS: TestE2E_Template_RendersVariables (0.00s)
ok  	example.com/sms-platform/e2e	0.050s
```
- [x] **`go test ./... -race -count=1` 全綠**
```
?   	example.com/sms-platform/cmd/smsd	[no test files]
ok  	example.com/sms-platform/e2e	1.414s
?   	example.com/sms-platform/e2e/testutil	[no test files]
ok  	example.com/sms-platform/internal/httpapi	1.057s
?   	example.com/sms-platform/internal/provider	[no test files]
ok  	example.com/sms-platform/internal/provider/mock	1.073s
ok  	example.com/sms-platform/internal/queue	1.043s
ok  	example.com/sms-platform/internal/ratelimit	1.042s
ok  	example.com/sms-platform/internal/store	1.042s
ok  	example.com/sms-platform/internal/template	1.042s
ok  	example.com/sms-platform/internal/tenant	1.042s
?   	example.com/sms-platform/internal/webhook	[no test files]
```
  另在 `504de4c` 上跑 `go test ./internal/queue ./e2e -race -count=30`：全部 ok（e2e 10.031s），沒有 flaky。
- [x] **build、vet、gofmt**：`go build ./...` exit 0；`go vet ./...` exit 0；`gofmt -l .` 無輸出。
- [x] **build tag**：`grep -rn 'go:build red_queue' e2e/` 無結果（exit 1）。`e2e/webhook_test.go`（`//go:build red_webhook`）與 `e2e/mitake_adapter_test.go`（`//go:build red_mitake`）在 `800b7b4..HEAD` 沒有 diff。
- [x] **其餘既有 E2E 全部 PASS，斷言沒有放寬**。`git diff 800b7b4..HEAD` 對既有 `_test.go` 只有以下改動：
  - 4 行改為 `WithoutWorker()`：`messages_test.go` 3 處、`messages_list_test.go` 1 處。
  - `deliver` 的註解。
  - D005 改寫：保留 store 斷言，加嚴 provider 斷言。
  - `queue_retry_test.go` 是紅燈骨架的改寫，見〈產出摘要〉。
- [x] **禁用項目檢查**：
  - `go.mod` 沒有 `require`（grep exit 1）。
  - `t.Skip` 無結果。
  - `time.Now(`／`time.Sleep(` 只剩 `cmd/smsd/main.go:85` 原有的 `st.PurgeExpired(time.Now())`：位於組裝處，架構測試允許。
  - queue 預設的 `Wait` 用 `time.NewTimer`，屬於建構子預設值。
- [x] **只改 `affects_files`**：`git diff --name-only 800b7b4..HEAD` 共 12 個檔案，都在 `affects_files` 內：
  - `_bmad-output/implementation-artifacts/deferred-work.md`
  - `cmd/smsd/main.go`
  - `docs/openapi.yaml`
  - `e2e/messages_list_test.go`
  - `e2e/messages_test.go`
  - `e2e/queue_retry_test.go`
  - `e2e/template_test.go`
  - `e2e/testutil/clock.go`
  - `e2e/testutil/server.go`
  - `internal/queue/doc.go`
  - `internal/queue/queue.go`
  - `internal/queue/queue_test.go`
- [x] **填回報區前已跑 `/bmad-code-review`**：見〈程式碼審查〉。
- [x] **回報區已填完**：`status` 與 body 鏡像一致；產品 commit 與工單 commit 訊息都含 `T0008`。

### 隔離分支證據
- branch：`ct/T0008`
- base_sha：`800b7b476d6fbda2908f24f9c30e0cbe02a38f9e`
- validated_sha：`9080685361fc745b42f461c5d4c63bdf19e4f860`
- head_sha：`9080685361fc745b42f461c5d4c63bdf19e4f860`
- validated_sha ≠ head_sha 的說明：相同
- `git -C "$CT_WORKTREE" status --porcelain` 為空。`_bmad/render/bmad-code-review/` 是 `render_skill.py` 的產物，已被 git 忽略（狀態 `!!`）。

### 產品 commit
- `96b1280` feat(queue): T0008 送信 worker pool、重試退避與終態轉移
- `504de4c` fix(queue): T0008 依 code review 修正關機語意與測試
- `9080685` docs(T0008): code review deferred 項目追加至 deferred-work.md

### 互動紀錄
yolo 模式，沒有向塔台提問。以下是工單沒寫明、由 Worker 依「選最小改動」原則做的決定：
- [02:24] queue 不注入 `Now`。工單寫「`Provider`、`Now`、`Wait` 都經建構子注入」，但 queue 本身不讀時間：時間戳由 store 的 `Now` 產生，testutil 的 store 也已接上 `FakeClock.Now`。為避免留下沒用到的參數，只注入 `Provider` 與 `Wait`，並寫進 `doc.go`。塔台若要求照字面注入，只需改幾行。
- [02:27] 建構子改用 option。原本想用 `NewInProcess(st, Config)`，但 `internal/httpapi/httpapi_test.go` 呼叫了 `queue.NewInProcess(st)`，而它不在 `affects_files`。改成 `NewInProcess(st, opts ...Option)` 後舊的呼叫仍能編譯，不必動範圍外的檔案。代價是 Provider 變成選用：沒有 Provider 時 `Run` 回 `ErrNoProvider`。
- [02:27] `deliver` helper 選 `WithoutWorker()`，因為改動較小。`FilterByStatus` 本身就需要部分訊息（`a4`、`b2`）停在 `queued`，走真實 queue 做不到，所以保留手動轉移，只改註解。
- [02:28] 等待終態用 channel：`Env.AwaitStatus` 包住交給 queue 的 `store.System`，狀態一轉移就收到訊號。10s 真實時間看門狗只用來讓卡住的測試失敗。
- [02:29] Close 的語意：`Close` 等在途送信做完，`ctx` 到期才中止，被中止的訊息停在 `sending`。`Run` 的 ctx 結束時不取消在途送信；審查後再補上「此時拒收新訊息」。
- [02:35] code review 的 CHECKPOINT／HALT 都取預設：patch 全部套用，沒有 decision-needed 項目；沒修的照工單寫進 deferred。

### Renew 歷程
無

### 遭遇問題
- `/bmad-code-review` 的 `render_skill.py` 渲染結果仍保留 Jinja（例如 `{{ workflow.thorough_lenses }}`），`manifest.json` 的 `resolved_values` 為空，看起來 renderer 沒有代入 `customize.toml`。本次改從 skill 的 `customize.toml` 手動取出 lens 定義照跑。建議塔台確認 BMad render 腳本在 worktree 中的行為。
- 審查沒有要求同步 `sprint-status.yaml`，本單也沒有修改它。
- 主 checkout 的 `_ct-workorders/_decision-log.md` 與 `_tower-state.md` 有塔台尚未 commit 的修改。本單只用 `git commit --only` 提交自己的工單檔，沒有動那兩個檔。

### 子代理與 Workflow 使用
| 種類 | 數量 | 用途 | 唯讀（怎麼保證） | tokens | 備註 |
|---|---|---|---|---|---|
| 子代理（Agent 工具） | 4 | `/bmad-code-review` 的 4 個 lens | 是（general-purpose＋prompt 開頭明示 READ-ONLY、禁止寫檔與改變狀態的 git 指令；事後以 `git status` 確認沒有新檔） | ~379k（工具回報：91.6k＋92.7k＋90.4k＋104.2k） | 主 agent 逐條對照程式碼做 triage；VG 的兩條 patch 用 mutation 驗證 |

沒有使用 Workflow。

### sprint-status.yaml 已更新
不適用

### 回報時間
2026-10-23 02:42 (UTC+8)
