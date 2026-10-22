# 塔台學習紀錄

> 由塔台自動維護。啟動時讀取，用於改善決策品質。
> 格式為純 markdown，任何 AI Agent 均可讀寫。
> Phase 3（v4.4.0+）起採三區結構，分類規則見 ct-evolve SKILL.md Step 2.6。
> 本檔首次建立：2026-10-23，`*evolve` 萃取 T0008～T0011。`gp-prefix: SMS`。儲存範圍依使用者決定：全部存本專案（Layer 3），不建立全局層；標 `candidate: global` 者待日後 review 是否晉升。

## 🧭 Strategic（跨技術棧決策原則）

### P2: 開單前把工單引用的鍵名、契約名、狀態值與 SPINE 對照
- **Learning**: 塔台自己定案的名稱（決策日誌、工單輸入上下文）若沒回頭對照架構文件，會在 Worker 實作時變成提問或漂移。
- **Trigger**: 工單的輸入上下文引用設定鍵、契約欄位或狀態值，而專案有固定這些名稱的架構／規格文件（本專案為 SPINE AD-x）。
- **Pattern**: 當工單引用的名稱在權威文件裡也有定義 → 開單前逐項 grep 對照，不一致時先修工單或先開決策，再派發。
- **Action**: 開單前對每個工單引用的鍵名、欄位、狀態值 grep 權威文件；不一致就在工單中採用權威名稱，或先記決策取代舊決策。
- **Example**: T0010 Q-5：D014 與工單寫 `MITAKE_BASE_URL`，SPINE AD-18 固定為 `PROVIDER_URL`，最後由 D019 改用 `PROVIDER_URL`；Q-2 的回呼密鑰鍵與狀態值也是依 AD-18、AD-7 修正（D017）。5 題提問中至少 2 題源自此漂移。
- **來源**：2026-10-23 / sms-platform / T0010
- **驗證次數**：1
- **狀態**：🔵

### P3: 工單新增套件時，檢查架構守門測試是否需要新節點
- **Learning**: 守門測試（import 圖、禁用呼叫）會必然擋下新套件，測試檔卻常不在 `affects_files`。
- **Trigger**: 工單新增一個套件或目錄，且專案有檢查依賴圖或禁用清單的架構測試。
- **Pattern**: 當新套件不在守門表內 → 守門測試必紅；需要把守門測試檔納入範圍，或事先決定改法。
- **Action**: 開單時檢查新套件是否會觸發守門測試；是的話把守門測試檔列入 `affects_files`，並在〈做法〉寫明「只加不放寬」。
- **Example**: T0010 新增 `internal/provider/providertest`，`e2e/architecture_test.go` 不在 `affects_files`，Q-4 因此變成 BLOCKED、使用者介入、Renew #1（D020）。
- **來源**：2026-10-23 / sms-platform / T0010
- **驗證次數**：1
- **狀態**：🔵

### P4: 已知兩張並行單會共改同一檔，事先寫「合併注意」並預留模式 ② 合併工單
- **Learning**: 刻意接受的衝突點事先標明，後合併的那張用合併工單解，成本很低。
- **Trigger**: 兩張隔離工單的 `affects_files` 有完全相同路徑，且你決定接受衝突而不序列化。
- **Pattern**: 當衝突是刻意的 → 兩張工單都寫明衝突檔與各自意圖；先收單者走模式 ①，後者開模式 ② 合併工單。
- **Action**: 兩張工單都加「合併注意」段落（衝突檔、兩邊各自改哪幾行）；後者收單時開 `kind: merge` 工單並以乾淨匯出重驗。
- **Example**: D012 預期 T0008 與 T0010 共改 `cmd/smsd/main.go`；T0010 先合（模式 ①），T0008 走 T0011（模式 ②），只有該檔衝突、兩邊皆保留，T0011 約 3 分鐘 DONE。
- **來源**：2026-10-23 / sms-platform / T0008、T0010、T0011
- **驗證次數**：1
- **Playbook**：已納入 `_playbooks/GP-SMS-001-parallel-worktree-merge.md`（🟡，2026-10-23）
- **狀態**：🔵

### P8: 每張 Worker 回報的 followups 集中到一處，不散在狀態檔多個區塊
- **Learning**: followups 分散會讓後續開單時漏撿，狀態檔 Quick Recovery 也變長。
- **Trigger**: Worker 回報留下 followups，而使用者決定不立刻開單。
- **Pattern**: 當 followups 暫不開單 → 集中記在 `_backlog.md`（或單一區塊），Quick Recovery 只留指標。
- **Action**: 建立 `_backlog.md` 收 followups，每條附來源工單；開單時從這裡撿。本專案**目前只記錄，不開單、不建 `_backlog.md`**（使用者決定）。
- **Example**: 目前 `_tower-state.md` 的待辦 0a、0c、1a、1b 與 4 五處各放一批 followups；epics 1 回顧的 F1 就是從其中撿回來開成 T0012。
- **來源**：2026-10-23 / sms-platform / T0008～T0010 回報、`_tower-state.md`
- **驗證次數**：1
- **狀態**：🔵

### A2: 阻塞性提問送出後，在等答覆期間跑超過輪詢窗的前景長指令
- **描述**: yolo Worker 送出阻塞題後，直接跑 `go test -race` 等長前景指令，兩次輪詢間隔超過 120 秒，BAT 把題目標成 `expired`（`endReason: not-polled`）。
- **正確做法**: 送出阻塞題後先輪詢到有答覆；若必須同時跑長指令，放背景並定時輪詢。塔台工單的 Sub-session 指示可明寫這一點。
- **來源**：2026-10-23 / sms-platform / T0010 Q-4（遭遇問題 1）

## 🔧 Tactical（具體技術操作 know-how）

### P1: 驗收「無某字串」的指令，限定追蹤檔與副檔名
- **Learning**: 全 repo `grep -rn` 會命中工單文字與 gitignored 暫存副本，造成字面驗收永遠過不了。
- **Trigger**: 驗收條件要斷言某字串（build tag、禁用呼叫）在程式碼中不存在，而 repo 內有工單、決策日誌或 gitignored 暫存目錄。
- **Pattern**: 當驗收指令掃描範圍比「要驗的程式碼」大 → 誤報；範圍要縮到追蹤中的原始碼檔。
- **Action**: 用 `git grep -n '<pat>' -- '*.go'`，或 `grep -rn --include=*.go --exclude-dir=_bmad-output`，不用 `grep -rn .`。
- **Example**: T0009 驗收 `grep -rn 'go:build red_' .` 因工單文字與 `_bmad-output/tmp/verify-T00xx/` 命中，改以 `git grep -n 'go:build red_' -- '*.go'`（exit 1）判定達成，並由 D023 依意圖判定。
- **來源**：2026-10-23 / sms-platform / T0009
- **驗證次數**：1
- **狀態**：🟡
- **candidate**：global ← 待 review 是否晉升全局

### P5: 與 queue／Provider 配合的 E2E 用 channel 訊號等待，不輪詢、不 sleep
- **Learning**: 注入可阻塞的假時鐘，加上「狀態轉移時發訊號」的等待 helper，讓非同步測試確定性通過。
- **Trigger**: E2E 要等背景 goroutine（worker、dispatcher）完成非同步轉移，且專案禁用 `time.Sleep`／真實時間。
- **Pattern**: 當要等非同步狀態 → 在被觀察的邊界（store 轉移、outbox 完成）發 channel 訊號，helper 等訊號並帶看門狗逾時。
- **Action**: 假時鐘的 `Wait` 阻塞到 `Advance`；提供 `BlockUntilWaiters(t, n)`、`AwaitStatus`、`AwaitOutboxIdle`；看門狗 10 秒只用來讓卡住的測試失敗。
- **Example**: T0008 建 `FakeClock.Wait`／`AwaitStatus`；T0009 沿用於 `AwaitOutboxIdle`；`-count=30` 無 flaky。
- **來源**：2026-10-23 / sms-platform / T0008、T0009
- **驗證次數**：2（同專案）
- **狀態**：🟡
- **candidate**：global ← 待 review 是否晉升全局

### P6: 測試要有牙：以 mutation 或 AST 檢查證明它會失敗
- **Learning**: 「幾乎不可能失敗」的測試是假覆蓋；對關鍵安全或關機行為要證明測試真的會紅。
- **Trigger**: code review 指出某行為沒有測試或測試太弱，或某實作必須符合不可退讓的寫法（constant-time 比對）。
- **Pattern**: 當補一個測試保護關鍵分支 → 暫時拿掉該分支（mutation）確認測試 FAIL；寫法規則用 AST 檢查。
- **Action**: 補測試後做 mutation 並在回報記下失敗訊息；簽章驗證以 AST 檢查恰好呼叫 `hmac.Equal`、沒有 `==`／`bytes.Equal`。
- **Example**: T0008 `TestCloseDuringSendLeavesSending`、`TestRunCtxEndFinishesInFlightSend` 都做 mutation 確認 FAIL；T0009 `Verify` 的 AST 檢查。
- **來源**：2026-10-23 / sms-platform / T0008、T0009
- **驗證次數**：2（同專案）
- **狀態**：🟡
- **candidate**：global ← 待 review 是否晉升全局

### P7: 修改既有建構子簽名前，先 grep 範圍外的呼叫端；用 variadic option 保相容
- **Learning**: 範圍外的測試檔呼叫舊簽名時，改成 variadic option 就不必擴大 `affects_files`。
- **Trigger**: 工單要為既有建構子（`NewX(a)`）加依賴，而呼叫端含不在 `affects_files` 的檔案。
- **Pattern**: 當要加參數但不能動呼叫端 → `NewX(a, opts ...Option)`，新依賴走 option。
- **Action**: 先 `grep -rn 'NewX('` 找呼叫端；有範圍外呼叫端就用 functional option，並在互動紀錄註明代價（例：依賴變選用，缺少時 `Run` 回錯誤）。
- **Example**: T0008 把 `NewInProcess(st)` 改成 `NewInProcess(st, opts ...Option)`，`httpapi_test.go` 不必動；代價是沒有 Provider 時 `Run` 回 `ErrNoProvider`。
- **來源**：2026-10-23 / sms-platform / T0008
- **驗證次數**：1
- **狀態**：🔵
- **candidate**：global ← 待 review 是否晉升全局

## 📜 Episodic 事件日誌（未 distill）

### E1: `--close-worker … --worktree remove` 在模式 ①／② 合併後回 exit 3
- **發生時間**：2026-10-23T02:56:56+0800（T0010）、2026-10-23T03:01:28+0800（T0008）
- **工單**：T0008、T0010
- **事件**：兩次都因整合 worktree 已 detach 後執行，exit `3`，改手動 `git worktree remove` exit `0`。T9004 同指令 exit `0`。`auto-session.md` 沒有說明 exit `3`。
- **嘗試 distill 結果**：Trigger 寫不出——不知道 exit 3 的成因，無法界定何時發生。
- **Review tag**：待 review（查清 exit 3 語意後再 distill；可能回報 CT 塔台）

### E2: BMad `render_skill.py` 在 worktree 未代入 `customize.toml`
- **發生時間**：2026-10-23
- **工單**：T0008
- **事件**：`/bmad-code-review` 渲染結果保留 Jinja，`manifest.json` 的 `resolved_values` 為空；Worker 改從 `customize.toml` 手動取 lens 定義。
- **嘗試 distill 結果**：只發生一次，無法確認是 worktree 特有還是 renderer 問題，Pattern 缺 if-then。
- **Review tag**：待 review

## 工作流程序列

### SEQ1: 並行隔離 → 先合一張 → 模式 ② 合併工單 → 乾淨匯出重驗 → 自動 ff
- **序列**：兩張隔離單並行派發（T0008、T0010）→ 先收單者走模式 ① 合併 → 後者 `--worktree-status` 判準不成立 → 開 `kind: merge` 工單（T0011）→ 塔台乾淨匯出獨立重驗 → 斷點 E 自動 `--ff-only`
- **工單數**：3
- **總耗時**：~40min（02:22 派發至 03:01 合回 `main`）
- **成功率**：100%
- **驗證次數**：1
- **Playbook**：已升為 `_playbooks/GP-SMS-001-parallel-worktree-merge.md`（🟡，2026-10-23）
- **適用場景**：兩張隔離單刻意共改少量檔案（D012 型衝突），且衝突為機械式（兩邊意圖互補）

## 估時校準

| BMad 流程 | 樣本 | 平均(min) | 建議預估 |
|-----------|------|-----------|---------|
| L 級功能單（隔離，含 code review） | 2（T0008、T0010 含 Renew） | 28 | 25～35 |
| L 級功能單（主 checkout，無提問） | 1（T0009） | 16 | 15～20 |
| 合併工單模式 ② | 1（T0011） | 3 | 5 |

## 📚 歷史條目（Phase 3 之前，不 retrofit）

<details>
<summary>展開查看</summary>

（無）

</details>
