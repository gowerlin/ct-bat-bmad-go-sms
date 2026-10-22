# Decision Log

## 索引

| # | 日期 | 決策 | 相關工單 |
|---|------|------|---------|
| D001 | 2026-10-22 | 只用標準庫 `net/http`＋`httptest`，不引入第三方套件 | T0001 |

| D002 | 2026-10-22 | API key 只從 header `X-API-Key` 讀，query 帶 key 視為沒帶 | T0003 |
| D003 | 2026-10-22 | 路由維持標準庫 `http.ServeMux`，中介層用函式包裝＋`chain` helper（A1），不引入 chi；不推翻 D001 | EXP-ROUTER-001、T9002 |
| D004 | 2026-10-22 | httpapi 每功能一檔，以明確 stub 預留接口（不用 `init()` registry），讓並行工單不撞檔 | T9003 |
| D005 | 2026-10-22 | 模板 E2E 暫以 store 讀回的 `Body` 驗證已渲染；FakeProvider 驗證待 queue 送信 worker 補 | T0004 |
| D006 | 2026-10-22 | `ErrInvalidTemplate` 回 400 `invalid_template`（照 `docs/error-codes.md`），工單的 `invalid_request` 是筆誤 | T0004 |
| D007 | 2026-10-22 | `vars` 為 JSON 物件、值只收字串；非字串回 400 `invalid_request` | T0004 |
| D008 | 2026-10-22 | T9004 以模式 ① 機械合併回 `main`（五條判準全過，塔台直接合併，不派合併工單） | T9004 |
| D009 | 2026-10-22 | 限流：每 API key 一桶，容量＝租戶 `rate_limit_per_min`、每秒補 `rate_limit_per_min/60`；上限放租戶設定、預設 60（使用者決定） | T0005 |
| D010 | 2026-10-22 | `Retry-After` 取 `max(1, ceil(秒數))`（使用者決定） | T0005 |
| D011 | 2026-10-23 | CH5 紅燈骨架以每張工單一個 build tag（`red_queue`／`red_webhook`／`red_mitake`）commit，預設 `go test ./...` 維持綠；移除 tag 是各單驗收條件 | T0008、T0009、T0010 |
| D012 | 2026-10-23 | `cmd/smsd/main.go` 由 T0008、T0010 刻意共改（5-08 衝突合併），T0009 之後接手；`e2e/testutil` 歸 T0008，T0009 在其後接續，T0010 不碰 | T0008、T0009、T0010 |
| D013 | 2026-10-23 | 出站通知簽章：HMAC-SHA256、金鑰＝租戶 `CallbackSecret`、簽原始 body bytes、header `X-Signature: sha256=<小寫 hex>`，body 固定 `{"id","status"}`，本版不含時間戳／防重放 | T0009 |
| D014 | 2026-10-23 | 三竹風格 send 的表單欄位與回應格式由塔台定案（假帳號、假端點）；statuscode 對應暫時／永久失敗與回呼格式留給 T0010 Worker 問塔台 | T0010 |
| D015 | 2026-10-23 | 供應商回呼回應碼：造成狀態轉移回 204；重複或已終態回 200（照 AD-7），`openapi.yaml` 由 T0009 補 200 | T0009 |
| D016 | 2026-10-23 | mitake `Send` 失敗類別：5xx／429／網路錯誤／明列 statuscode（t、u）→ 暫時；其他 4xx、其他非 1、200 無法解析 → 永久；ctx 錯誤同時包 `ErrTransient` | T0010 |
| D017 | 2026-10-23 | mitake 回呼：表單 POST、header `X-Mitake-Secret` constant-time、密鑰鍵 `PROVIDER_CALLBACK_SECRET`（AD-18）；只接受 4→delivered、5／6／8→undelivered（AD-7），其他值包 `ErrPermanent` | T0010、T0009 |
| D018 | 2026-10-23 | 共用契約的「未授權回呼」為選填：`mock` 不提供、`mitake` 必須提供；`mock` 回呼無驗證記為已知差距 | T0010 |
| D019 | 2026-10-23 | mitake 位址設定鍵改用 AD-18 的 `PROVIDER_URL`（取代 `MITAKE_BASE_URL`）；帳密仍 `MITAKE_USERNAME`／`MITAKE_PASSWORD`，待回寫 spine | T0010 |
| D020 | 2026-10-23 | `e2e/architecture_test.go` 納入 T0010：補 `provider/providertest` 節點與「只限 `_test.go`」的邊，不放寬其他規則（使用者決定） | T0010 |
| D021 | 2026-10-23 | T0010 以模式 ① 機械合併回 `main`（五條判準全過，塔台直接合併，merge `1b0dabb`） | T0010 |
| D022 | 2026-10-23 | T0008 以模式 ② 衝突解決合併回 `main`（合併工單 T0011，merge `abc1eba`，`main` = `7df6703`） | T0008、T0011 |
| D023 | 2026-10-23 | T0009 收單 DONE：驗收「`grep -rn 'go:build red_' .` 無結果」依意圖判為 Go 原始碼沒有 `red_` tag（字面命中只在塔台文件與 gitignored 驗證暫存） | T0009 |
| D024 | 2026-10-23 | `sprint-status.yaml` 以工單為 SoT 補同步：1-2～1-9、2-1～2-3 → `done`，epic-2 → `done`；1-1 無工單證據維持 `review`，epic-1 維持 `in-progress` | T0002～T0010 |

| D025 | 2026-10-23 | epic 1 回顧 F1／F10 開單：T0012 補 `SMS_CONFIG` 載入、T0013 補 `main_test.go`；`SMS_CONFIG` 必填、沿用 `tenant.Registry` 不引入 `store.TenantDirectory`、T0013 序列在 T0012 之後 | T0012、T0013 |
| D026 | 2026-10-23 | `SMS_CONFIG` JSON 欄位名與驗證規則由塔台定案（Spine AD-11 只給概念） | T0012 |
| D027 | 2026-10-23 | `*evolve`／`*import` 產物全存本專案（不建全局層），P1、P5、P6、P7 標 `candidate: global`；GP-SMS-001 以 🟡 寫入；P8 只記錄不開單；新增 local rule 限定驗收字串檢查範圍 | T0008～T0011 |

---

## 詳細紀錄（倒序）

### D027 2026-10-23 — 學習、playbook 與 local rule 的儲存範圍與置信度

- **背景**：使用者要求 `*evolve` 萃取 T0008～T0011，之後 `*evolve --playbook parallel-worktree-merge`（SEQ1＋P4）與 `*import rule`。`_learnings.md`、`_playbooks/`、`_local-rules.md` 原本都不存在，全局層 `~/.claude/control-tower-data/` 也不存在。
- **選項**：
  - 儲存範圍：[A] 通用，存入全局（需新建 Layer 2）；[B] 本專案；[C] 先放專案並標 `candidate: global`
  - Playbook 置信度：[A] 🟢（來源 SEQ1、P4 皆 🔵，只驗證 1 次，與 `playbook-engine.md` 晉升標準不符）；[B] 🟡 實驗中（如實反映證據）
  - P8（followups 集中）：[A] 開單建 `_backlog.md`；[B] 只記錄
- **決定**：全部存本專案，不建立全局層；P1、P5、P6、P7 標 `candidate: global`，其餘不標。GP-SMS-001 以 [B] 🟡 寫入專案層（使用者決定）。P8 [B] 只記錄、不開單、不建 `_backlog.md`。`*import rule` 新增「驗收『沒有某字串』一律限定追蹤的 Go 檔」，由 P1 而來（`git grep -n '<樣式>' -- '*.go'`）。`gp-prefix: SMS` 放在 `_tower-config.yaml` 頂層。
- **影響**：後續工單的驗收寫法受 `_local-rules.md` 約束（T0012、T0013 若寫「無某字串」類驗收須照辦）。GP-SMS-001 要等再一次並行共改驗證後才能升 🟢。Episodic E1（`--close-worker --worktree remove` exit 3）、E2 未 distill。commit：`09efd0b`（學習）、`be01ea8`（playbook）、`2b99563`（local rule）。
- **相關工單**：T0008、T0009、T0010、T0011

### D026 2026-10-23 — `SMS_CONFIG` 設定檔欄位與驗證規則

- **背景**：AD-11 只說「JSON、租戶／API key SHA-256 hex／`callback_url`／`webhook_secret`／`rate_limit_per_min`／`disabled`（API key 層級）」，沒有欄位名與巢狀結構；Story 1.8 與 2.1 的 AC 也只列概念。Worker 自己發明會讓 Epic 2 後續擴充與範例檔不一致。
- **選項**：
  - [A] 塔台定案：`{"tenants":[{"id","callback_url","webhook_secret","rate_limit_per_min","api_keys":[{"sha256","disabled"}]}]}`，`DisallowUnknownFields`，驗證規則寫進工單
  - [B] 只做 Story 1.8 最小版（預設租戶的 `callback_url`／`webhook_secret`）：仍無法驗證 API key，`/v1/*` 依舊 401，F1 沒解
  - [C] 留給 Worker 提問：多一輪往返，且規則本來就是塔台可決定的格式
- **決定**：選 A。`disabled` 放 API key 層（AD-11、FR-21）；有 `callback_url` 必須有 `webhook_secret`；`rate_limit_per_min` 省略／0 沿用預設 60（D009）；錯誤訊息不得回顯 secret／hash（AD-15）。範例檔只放假值。
- **影響**：Story 2.1 的完整租戶設定因此提前由 T0012 實作到可用程度；`epics.md` 2.1 的 AC 與 Spine AD-11 尚未回寫欄位名（回顧 action item 7，交 Winston）。
- **相關工單**：T0012

### D025 2026-10-23 — epic 1 回顧 F1／F10 開單與範圍

- **背景**：使用者要求把 `epic-1-retro-2026-10-23.md` Action items 第 1 條（F1：真實 `smsd` 對所有 `/v1/*` 回 401）與第 6 條（F10：`cmd/smsd/main_test.go`）各開一張工單，只開單、不派工。D023 followups 已列「`SMS_CONFIG` 載入」。
- **選項**：
  - 範圍 [A]：沿用現有 `tenant.Registry`，只加「以 hash 登錄 key」入口；[B] 同時引入 Spine 的 `store.TenantDirectory`：介面重構會連動 `webhook`／`ratelimit`／`httpapi`，超出 F1，且 D020 後依賴圖才剛調整
  - `SMS_CONFIG` 缺漏：[A] 必填、缺漏啟動失敗（與 `PROVIDER` 一致，AD-11 fail-fast）；[B] 缺漏時退回空 registry：會把「全部 401」這個靜默失敗留下來
  - 順序：[A] T0013 `depends_on` T0012，序列；[B] 平行：兩張都改 `cmd/smsd/main.go`，T0013 還需要 T0012 抽出的組裝函式，平行必衝突
  - F10 驗證：[A] 要求變異驗證（拿掉 `WithProvider`、對調關機順序測試必須變紅）；[B] 只寫測試：無法證明補到了回顧指出的缺口
- **決定**：全選 A。T0012 抽出可測的組裝函式並在 `cmd/smsd` 內寫「走真實組裝根」測試（`package main` 不能被 `e2e/` import，這是 `AGENTS.md`「增量附 E2E 放 `e2e/`」的有意例外，工單要求 Worker 記錄理由）；T0013 允許兩項為可測性的最小變更（`purgeLoop` 注入 tick／clock、`q.Run` 錯誤改為記 `error_class=queue`），後者是唯一行為變更。
- **影響**：未納入本次的回顧項目：F2／Q1 通知範圍、F3 Story 1.1 收單、F8 `Send` 逾時值、F9 空 `ProviderRef`、action item 7～9 皆未開單，等使用者決定。T0012 完成並合回 `main` 才能派 T0013。兩張都不隔離、主 checkout yolo（比照 T0009）。本次只開單，不派工；sprint-status 無對應 story，不動。
- **相關工單**：T0012、T0013

### D024 2026-10-23 — `sprint-status.yaml` 以工單為 SoT 補同步

- **背景**：5-10 收尾照 CT 收單規則（`work-order-system.md` § 回報處理流程 第 2 步、§ sprint-status.yaml 同步）複查 `_bmad-output/implementation-artifacts/sprint-status.yaml`：自 CH5 起點 `f00fd8e` 起沒有更新過，除 1-1 為 `review` 外全部 `backlog`；但 T0002～T0010 都已 DONE。各單回報區〈sprint-status.yaml 已更新〉皆為「不適用」（T0008 明列不在範圍，由塔台收單時補）。
- **選項**：
  - [A] 以工單為 SoT，依 story 標題一對一補成 `done`；沒有工單證據的不動
  - [B] 只補本次收單的 T0009（1-8）：其餘漂移留著，`sprint-status` 繼續失真
  - [C] 不動，留給 `bmad-sprint-planning` 修復：本次不需要使用者決定，規則已指明塔台補
- **決定**：選 A。對應：1-2 T0002、1-3 T0006、1-4 T0004、1-5 T0007、1-6／1-7 T0008、1-8 T0009、1-9 T0010、2-1／2-2 T0003、2-3 T0005 → `done`；epic-2 三個 story 皆 `done` → `done`。1-1（CH2 story 1）沒有工單、沒有 code review 紀錄，維持 `review`，epic-1 因此維持 `in-progress`。`last_updated` 改為 `10-23-2026 03:26`（取自系統，沿用檔案既有格式）。
- **影響**：1-1 是否標 `done` 交使用者 → 使用者決定維持 `review`、不標 `done`；epic retrospective 仍 `optional`。使用者指示後與塔台私有檔一起 commit。
- **相關工單**：T0002～T0010

### D023 2026-10-23 — T0009 收單 DONE；`red_` tag 驗收依意圖判定

- **背景**：T0009（主 checkout、不隔離、yolo）回報 DONE，產品 `b2166dd`、工單 `c7d2677`，無提問。〈遭遇問題〉1：驗收「`grep -rn 'go:build red_' .` 在整個 repo 沒有任何結果」字面仍有輸出，命中 (a) `_ct-workorders/` 的工單／決策日誌／塔台狀態引用這個指令的文字；(b) `_bmad-output/tmp/verify-T0008|T0010|T0011/`（`.gitignore` 排除）裡塔台先前乾淨匯出驗證留下的舊版 `e2e/*_test.go`。
- **選項**：
  - [A] 依意圖判定：條件要的是「三個紅燈 tag 都已從 Go 原始碼清掉」；`git grep -n 'go:build red_' -- '*.go'` exit `1`、`grep -rnE '^//go:build .*red_' e2e/` exit `1` 即成立
  - [B] 字面判定 → `PARTIAL`：命中的是塔台自己的檔與暫存，Worker 依工單〈不做〉不得動，Renew 也無法讓它變綠
- **決定**：選 A，判 DONE。塔台收單驗證：22 檔變更（`631bd91..c7d2677`）全部在 `affects_files`；主 checkout `go build`／`go vet` exit `0`、`gofmt -l` 空、`TestE2E_Webhook_*` 4 個 PASS、`go test ./... -race -count=1` exit `0`；`go.mod` 無 `require`、無 `t.Skip`、`internal/` 與 `e2e/` 非測試檔無 `time.Now(`／`time.Sleep(`、`internal/webhook` 無 `http.Client{`。Worker 自行取捨（只通知回呼造成的轉移、`GetByProviderRef` 只用 ref、`undelivered` reason 常數、Dispatcher 另建受限 client、`Deps.Callbacks` 選填）都在工單範圍內，照記錄接受。
- **影響**：日後工單的同類驗收改寫成 `git grep -n 'go:build red_' -- '*.go'`；`_bmad-output/tmp/verify-*` 是 gitignored 暫存，暫不刪（列入 followups）。Worker 分頁 `--close-worker T0009 --delay-ms 10000` exit `0`（使用者指定延遲；`bat_list_terminals` 已 `alive: false`）。T0009 followups（`sent`／`failed` 也通知、24h 回呼逾時、`(provider_name, provider_ref)` 索引、簽章防重放、mock 回呼無驗證、`SMS_CONFIG` 載入、`cmd/smsd/provider.go` 過時註解）待開單。
- **相關工單**：T0009

### D022 2026-10-23 — T0008 以模式 ② 衝突解決合併回 `main`

- **背景**：T0010 先合（D021）後，`--worktree-status T0008`（`source: git`）判準第 1 條（`behind 10`，`main` 多了 T0010 產品檔）與第 4 條（`conflicts ["cmd/smsd/main.go"]`）不成立；第 2、3、5 條成立（`head` = `validated_sha` = `9080685`、`outOfScopeFiles []`、`dirty []`）。
- **選項**：
  - [②] 衝突解決：衝突只有 D012 刻意留下的組裝行，git merge 解衝突即可
  - [③] 等價移植：分支未過舊、無重構，不需要
- **決定**：模式 ②，開合併工單 T0011（`base_main_sha` `1b0dabb`，整合 worktree `..\.ct-integration\sms-platform`），yolo 派出。Worker 回報 DONE：merge `abc1eba`（parent1 `1b0dabb`／parent2 `9080685`），`main.go` 兩邊都保留（`newProvider` 錯誤處理＋`queue.NewInProcess(st, queue.WithProvider(prov))`），驗收跑在 `abc1eba` 全綠、範圍外檔案無；塔台乾淨匯出獨立重驗全綠。SHA 比對 ①②③ 成立；⑤ `main` 因 T0011 工單 commit 前進到 `9ecabb1`，只動 `_ct-workorders/**` → 整合分支再 `--no-ff` 合 `main` 得 `7df6703`（產品樹同 `abc1eba`）；斷點 E（yolo、模式 ②、全綠＋無範圍外）自動 `--ff-only`；④ `main` = `7df6703`。
- **影響**：整合 worktree detach；`--close-worker T0008 --worktree remove` exit `3` → 手動 `git worktree remove` exit `0`；使用者事先同意，`git branch -d ct/T0008 ct-merge/T0008` exit `0`；T0011 分頁 `--close-worker --delay-ms 10000` exit `0`。<個人規則>〈合併模式 ②〉證據已送 CT 塔台（`<CT 塔台>`）。merge commit 訊息含 git 自動的 `# Conflicts:` 兩行，未 amend。T0009 可在主 checkout 派出（本段未派）。
- **相關工單**：T0008、T0011

### D021 2026-10-23 — T0010 以模式 ① 機械合併回 `main`

- **背景**：T0010（Renew #1）回報 DONE，`ct/T0010` 要先於 T0008 合併回 `main`（D012 合併順序）。照 `merge-workorder.md` 判準決定模式。
- **判準**（`--worktree-status T0010`，`source: live`）：① `behind 7`，`800b7b4..0c5d01b` 只有 `_ct-workorders/T0008*`、`T0010*`（放寬版成立）；② `validated_sha` = `head` = `f6f00a5`；③ `outOfScopeFiles []`、`touchesCtWorkorders false`；④ `conflicts []`；⑤ `dirty []`，主 checkout 髒檔只有 `_decision-log.md`、`_tower-state.md`，與 13 個 `changedFiles` 不重疊。ff 前重查仍成立，`main` 未前進。
- **決定**：模式 ①。整合 worktree `..\.ct-integration\sms-platform`（本匯出專案首次建立）開 `ct-merge/T0010` 自 `base_main_sha` `0c5d01b`，`--no-ff` 得 merge commit `1b0dabb`（parent1 `0c5d01b`、parent2 `f6f00a5`）；合併結果與 `head` 只差 `_ct-workorders/**`，不重跑驗收（塔台已於 `f6f00a5` 乾淨匯出重跑 `go test ./... -race -count=1` 全綠）。yolo 斷點 E 自動 `--ff-only`，`main` = `1b0dabb`。
- **影響**：整合 worktree 已 `switch --detach`；`--close-worker T0010 --worktree remove` exit `3`（分頁已以 `keep` 關閉）→ 手動 `git worktree remove`（乾淨，exit `0`）；使用者事先同意，`git branch -d ct/T0010 ct-merge/T0010` exit `0`。T0008 合併時 `cmd/smsd/main.go` 預期衝突（D012）。
- **相關工單**：T0010

### D020 2026-10-23 — 架構守門圖補 `provider/providertest`，T0010 擴大範圍

- **背景**：T0010 Worker 經 BAT 通道提問 Q-4（提問時間 2026-10-23T02:26:15+0800）：`TestArchitecture_ImportGraph` 必然擋下 AD-1 指定的 `provider/providertest`（不在 `allowedImports`；`mock_test.go`／`mitake_test.go` import 它也違規），但 `e2e/architecture_test.go` 不在 `affects_files`，工單〈不做〉也明列不改。塔台查證屬實；屬範圍變更，升級使用者。BAT 題在使用者回答前 `expired`（`not-polled`，Worker 輪詢間隔超過 120 s），Worker 走備援 `BLOCKED`。
- **選項**：
  - [A] 納入 `e2e/architecture_test.go`：新增節點 `"provider/providertest": {"provider"}`，另加「只限 `_test.go`」邊表（mock／mitake 的 `_test.go` 可 import），補測試驗證產品碼仍被擋
  - [A+] 同 A，並修 `ForbiddenCalls`（允許 `cmd/smsd` 建 `http.Client`，其他地方連 `new(http.Client)` 也擋）
  - [B] 直接加進 mock／mitake 允許清單（不分產品碼／測試碼）
  - [C] 不改架構測試，契約放 `internal/provider` 本身（產品碼 import `testing`，偏離 AD-1）
- **決定**：選 A（使用者決定，2026-10-23 於塔台分頁回覆「A」；塔台 2026-10-23T02:51:40+0800 寫入 T0010 `## 塔台補充（Renew #1）` 的 `塔台答覆 Q-4` 行，`renew_count` 1、狀態回 `PENDING`）。理由：AD-1 指定該節點，守門圖漏列，補齊不是放寬；只限 `_test.go` 讓 adapter 產品碼仍不能依賴測試工具。
- **影響**：T0010 `affects_files` 加 `e2e/architecture_test.go`，〈不做〉該條加 Renew #1 例外註記。`ForbiddenCalls` 對 `cmd/smsd` 的矛盾（AD-9 要求在 `cmd/smsd` 建 client，守門測試連 `cmd/smsd` 都禁 `http.Client{`；T0010 以 `new(http.Client)` 繞過字面檢查）本輪不處理，列 followup。
- **相關工單**：T0010

### D019 2026-10-23 — mitake 位址設定鍵改用 `PROVIDER_URL`

- **背景**：T0010 Worker 經 BAT 通道提問 Q-5（2026-10-23T02:27 前送出）：工單與 D014 寫 `MITAKE_BASE_URL`，SPINE AD-18 固定為 `PROVIDER_URL`；D017 已依 AD-18 定 `PROVIDER_CALLBACK_SECRET`。
- **選項**：
  - [A] 照工單與 D014：`MITAKE_BASE_URL`，差距記 followups
  - [B] 照 AD-18：位址改 `PROVIDER_URL`；帳密仍 `MITAKE_USERNAME`、`MITAKE_PASSWORD`（AD-18 未列，記 followups）
- **決定**：選 B（塔台代決，2026-10-23T02:27:42+0800 以 `--answer --choice 2` 經 BAT 通道答覆，exit `0`）。理由：與 D017 一致以 SPINE 為準；`MITAKE_BASE_URL` 是開單時塔台未對照 AD-18 的疏漏，修正成本只在本單的常數與文件。本條取代 D014 與 D017〈影響〉中對 `MITAKE_BASE_URL` 的寫法。
- **影響**：`cmd/smsd/provider.go`、`provider_test.go`、`.env.example`、`docs/README.md`、mitake 註解用 `PROVIDER_URL`；`MITAKE_USERNAME`／`MITAKE_PASSWORD` 不在 AD-18 清單，待回寫 spine（followup）。
- **相關工單**：T0010

### D018 2026-10-23 — 共用契約的「未授權回呼」為選填

- **背景**：T0010（yolo）Worker 經 BAT 通道提問 Q-3（提問時間 2026-10-23T02:23:55+0800）：共用契約要驗「回呼驗證失敗包 `ErrUnauthorized`」，但 `mock` 沒有回呼驗證、`mock.go` 不在 `affects_files`。
- **選項**：
  - [A] harness 的「未授權回呼」為選填：有提供才驗；`mock` 不提供，`mitake` 必須提供；共同必驗格式錯誤回錯誤、正確回呼解出非空 `ProviderRef` 與合法 `Status`
  - [B] 擴大範圍，把 `mock.go` 加入本單、替 `mock` 加共用密鑰驗證（改 mock 回呼格式，影響 T0009 `CallbackSink`）
- **決定**：選 A（塔台代決，2026-10-23T02:25:31+0800 以 `--answer --choice 1` 經 BAT 通道答覆，exit `0`，工單不寫答覆行）。理由：工單範圍內的技術選擇，不擴大範圍、不動 T0009 依賴的 `mock` 回呼格式；選填是 harness 明確宣告，不是 `t.Skip`。
- **影響**：`mock` 回呼沒有驗證是 AD-7 的已知差距，記入 T0010 回報區與 followups，留給 T0009 或後續工單。
- **相關工單**：T0010

### D017 2026-10-23 — mitake 回呼格式、驗證與狀態對應

- **背景**：T0010 Worker 經 BAT 通道提問 Q-2（提問時間 2026-10-23T02:23:55+0800），D014 已定此項須問塔台。Worker 建議 [A]（表單 POST、header 密鑰、設定鍵 `MITAKE_CALLBACK_SECRET`、`1`→`sent`、其他已知錯誤值→`failed`）。塔台比對 SPINE：AD-7「回呼只接受 `delivered`、`undelivered`，其他值回 400」、AD-18 環境變數固定含 `PROVIDER_CALLBACK_SECRET`，與 Worker 建議的兩處不一致。
- **選項**：
  - [A] Worker 建議原樣
  - [B] 同 A，密鑰放表單欄位 `secret`
  - [C] 同 A，改 `GET` query（密鑰進 URL，違背 D002 精神）
  - [D] 以 A 為基礎、依 SPINE 修正狀態對應與設定鍵名
- **決定**：選 D（塔台代決，2026-10-23T02:25:31+0800 經 BAT 通道答覆，不帶 `--choice`，exit `0`）：`POST`、`application/x-www-form-urlencoded`；`msgid`→`ProviderRef`、`statuscode`→`Status`、`donetime`（`yyyyMMddHHmmss`，`time.FixedZone("UTC+8", 8*60*60)`，不用 `LoadLocation` 以免 Windows 無 tzdata）→`ReceivedAt`，缺或壞用注入 `Now()`；header `X-Mitake-Secret` 以 `subtle.ConstantTimeCompare` 比對，缺或不符包 `ErrUnauthorized`；設定鍵 `PROVIDER_CALLBACK_SECRET`（`PROVIDER=mitake` 必填、不進日誌）；`4`→`delivered`，`5`／`6`／`8`→`undelivered`，其他值（含 `1`）回錯誤包 `ErrPermanent`。`Raw` = 表單 body。理由：SPINE 是架構權威；header 密鑰不進 URL（D002）；表單與 send 風格一致。
- **影響**：T0009 回呼路由依此格式，非 `ErrUnauthorized` 的解析錯誤對應 400 `invalid_request`、`ErrUnauthorized` 對應 401。觀察到的既有差異（不在本題處理）：T0010 工單的 `MITAKE_BASE_URL`／`MITAKE_USERNAME`／`MITAKE_PASSWORD` 與 AD-18 的 `PROVIDER_URL` 不一致，屬開單時的塔台選擇，留待設定檔載入 story（AD-11）統一。
- **相關工單**：T0010、T0009

### D016 2026-10-23 — mitake `Send` 的失敗類別對應

- **背景**：T0010 Worker 經 BAT 通道提問 Q-1（提問時間 2026-10-23T02:23:55+0800），D014 已定此項須問塔台。`provider.go` 寫「未分類錯誤視為 `ErrPermanent`」，裸 `ctx.Err()` 會讓 queue 把逾時當永久失敗。
- **選項**：
  - [A] HTTP 5xx、429、網路錯誤、明列模擬 statuscode（`t`、`u`）→ `ErrTransient`；其他 4xx、其他非 `1` statuscode、HTTP 200 無法解析 → `ErrPermanent`；ctx 取消／截止回 `ctx.Err()` 並同時包 `ErrTransient`
  - [B] 同 A，但 200 無法解析 → `ErrTransient`（接受重複發送風險）
  - [C] 同 A，但 ctx 錯誤照 mock 回裸 `ctx.Err()`
- **決定**：選 A（塔台代決，2026-10-23T02:25:31+0800 以 `--answer --choice 1` 經 BAT 通道答覆，exit `0`）。理由：與 SPINE AD-1「逾時與連線錯誤歸 `ErrTransient`、未分類歸 `ErrPermanent`」一致；200 無法解析代表供應商可能已收件，不重試避免重複簡訊；ctx 錯誤同時保留兩個 sentinel，T0008 的 queue 不必寫特例。
- **影響**：`mitake.go` 錯誤分類、`mitake_test.go`、`providertest` 暫時／永久案例、E2E 假對端的失敗回應；錯誤訊息只含類別與狀態碼，不含 `password`、號碼、內文。改對應表須開新決策取代本條。
- **相關工單**：T0010

### D015 2026-10-23 — 供應商回呼：造成轉移回 204，重複或已終態回 200

- **背景**：使用者的紅燈骨架與 `docs/openapi.yaml` 都寫 `POST /v1/providers/{name}/callbacks` 成功回 204；epics AD-7 寫「終態或重複回呼回 200」。兩處在「重複回呼」上沒有矛盾，但 openapi 只列 204。
- **選項**：
  - [A] 第一次造成轉移回 204、重複或已終態回 200（AD-7 全保留，openapi 補一列 200）
  - [B] 一律回 204（簡單，但偏離 AD-7）
  - [C] 一律回 200（骨架與 openapi 的 204 要改）
- **決定**：選 A，理由：骨架的 204 不用動，AD-7 與冪等語意（可由狀態碼區分「這次有效」與「重複」）同時成立；代價只是 openapi 補一列。
- **影響**：T0009 實作依此；`docs/openapi.yaml` 補 200；未知供應商或查無 `provider_ref` 仍回 404（openapi 既有）；簽章／驗證失敗（`ErrUnauthorized`）回 401 信封，不計限流、不經 API key。
- **相關工單**：T0009

### D014 2026-10-23 — 三竹風格 send 的表單與回應：塔台定案；錯誤碼對應與回呼格式留給 T0010 問塔台

- **背景**：T0010 要寫「三竹風格」HTTP adapter。公開文件的風格是表單 POST、純文字回應，但課程不連真實端點、全用假帳號，也不宣稱與真實服務逐欄一致；欄位名若留給 Worker 自行挑，E2E 假對端與 adapter 兩邊都是同一個 Worker 寫的，契約測試會變成自己驗自己。
- **選項**：
  - [A] 塔台現在定死 send 的欄位與回應，錯誤碼對應與回呼格式留給 Worker 提問
  - [B] 全部現在定死
  - [C] 全部留給 Worker 自行決定
- **決定**：選 A。send 請求：`POST {MITAKE_BASE_URL}/SmSendPost.asp`，`Content-Type: application/x-www-form-urlencoded`，欄位 `username`、`password`、`dstaddr`（原樣用 `OutboundMessage.To`）、`smbody`、`CharsetURL=UTF-8`、`clientid`（用 `OutboundMessage.ID`）。成功回應 `text/plain`：第一行 `[1]`，其後 `msgid=<id>`、`statuscode=1`、`AccountPoint=<n>` 各一行；`ProviderRef` = `msgid`。**這是「風格」，不是真實規格**：文件與程式註解都要寫明為模擬、帳號密碼為假。留給 Worker 向塔台提問（`tower_question`）的兩項：(1) 非 `1` 的 `statuscode` 與 HTTP 5xx／逾時，哪些對應 `ErrTransient`、哪些對應 `ErrPermanent`；(2) 回呼（`ParseCallback`）收的請求格式、欄位名，以及怎麼對應 `DeliveryReport.Status`。
- **影響**：T0010 的假對端與 adapter 照上述欄位；Q-1、Q-2 的答覆會寫進 D016（或之後的編號）。`http.Client` 只在 `cmd/smsd` 建立並注入（AD-9）。
- **相關工單**：T0010

### D013 2026-10-23 — 出站通知簽章編碼

- **背景**：T0009 要對租戶 `callback_url` 發簽章通知；簽什麼、怎麼編碼、放哪個 header 不先定，租戶端的驗證範例（`docs/onboarding.md`）就沒有依據。
- **選項**：
  - [A] HMAC-SHA256，簽原始 body bytes，小寫 hex，header `X-Signature: sha256=<hex>`；金鑰為租戶 `CallbackSecret`；不含時間戳
  - [B] 同 A，但簽 `<unix 時間戳>.<body>` 並多一個 `X-Timestamp` header（可防重放，租戶端驗證多一步）
  - [C] base64 編碼
- **決定**：選 A，理由：驗證只需「原始 body＋密鑰」，租戶端範例最短；hex 小寫無歧義、不需處理 padding；冪等已由 `(message_id, status)` 唯一約束保證「最多通知一次」，重放風險記入 followups，不在本版。比對一律 `hmac.Equal`（constant-time）。
- **影響**：`tenant.Tenant` 新增 `CallbackSecret string`（不得出現在日誌與任何回應）；body 固定 `{"id","status"}`（AD-3）；測試用假密鑰 `test-secret`，不得使用真實密鑰；`docs/onboarding.md` 補驗證範例。要加時間戳須開新決策取代本條。
- **相關工單**：T0009

### D012 2026-10-23 — `cmd/smsd/main.go` 與 `e2e/testutil` 的歸屬

- **背景**：T0008、T0010 平行（皆 `isolation: worktree`），使用者要在 5-08 練習衝突合併，刻意讓兩張都改 `main.go` 的組裝；`e2e/testutil` 則是共用夾具。
- **選項**：
  - [A] `main.go` 由 T0008 與 T0010 共改（刻意撞同一段組裝）；`testutil` 只歸 T0008，T0010 不碰，T0009 在兩張合回 `main` 後接續
  - [B] 把 `main.go` 的組裝抽成新檔讓兩張不撞（違背使用者的教學目的）
  - [C] 序列化 T0008→T0010（放棄並行示範）
- **決定**：選 A。T0008 在 `main.go` 把 `prov` 傳給 queue（改 `queue.NewInProcess` 那行並刪掉「consumed by the queue workers from Story 1.6」註解）；T0010 把 `prov := mock.New()` 改成依 `PROVIDER` 選擇（`PROVIDER` 無預設，未設定或非法 → 啟動失敗）。兩邊都**不得**為對方預先留位置，也不得在自己的分支解對方的衝突。合併順序：先 T0010、後 T0008；衝突在合併 T0008 時出現，由使用者處理（5-08）。T0009 派在兩張都合回 `main` 之後，在主 checkout 接手 `main.go` 與 `testutil`。
- **影響**：T0010 的 `affects_files` 不含 `e2e/testutil/*`；T0008 擁有 `testutil` 的改動（預設啟動 worker、`WithoutWorker()` 選項、FakeClock 等待同步）；`main.go` 是兩張的已知重疊（overlap check 預期 WARN，已由本條認可）。
- **相關工單**：T0008、T0009、T0010

### D011 2026-10-23 — CH5 紅燈骨架：每張工單一個 build tag 先 commit

- **背景**：使用者手寫的三組紅燈骨架（`queue_retry_test.go`、`webhook_test.go`、`mitake_adapter_test.go`）還沒 commit。`isolation: worktree` 的分支從 HEAD 切出，未 commit 的檔案不會出現在 worktree；但直接 commit 紅燈測試會讓 `go test ./... -race` 紅，每張工單的收單門檻都過不了。AGENTS.md 禁止 `t.Skip`。
- **選項**：
  - [A] 先 commit，各檔首行加 build tag（`red_queue`／`red_webhook`／`red_mitake`）；預設不編譯、`go test ./...` 維持綠；工單把測試改成真實斷言並**移除自己的 tag** 才算完成
  - [B] 原樣 commit，接受全套紅到三張都做完（T0008／T0010 的 `go test ./... -race` 驗收條件無法成立）
  - [C] 不 commit，由 Worker 自行重建（worktree 看不到，使用者的教學骨架白做）
  - [D] 用 `t.Skip` 暫時略過（違反 AGENTS.md Policy）
- **決定**：選 A，理由：不改測試斷言與意圖、不用 `t.Skip`；紅燈可用 `go test ./e2e -tags 'red_queue red_webhook red_mitake' -run 'Queue|Webhook|Mitake'` 隨時重現（已驗證預設綠、帶 tag 四個測試皆紅）。tag 不是永久機制：各單驗收條件含「`grep -rn 'go:build red_' e2e/` 不再出現自己的 tag」，T0009 收單時整個 repo 不得殘留任何 `red_` tag。
- **影響**：骨架的紅燈指令從 `go test ./e2e -run 'Queue|Webhook|Mitake'` 改為帶 `-tags`；build tag 檔案要和工單一起 commit，才能讓 worktree 看見。
- **相關工單**：T0008、T0009、T0010

### D010 2026-10-22 — `Retry-After` 取 `max(1, ceil(秒數))`

- **背景**：T0005（yolo）Worker 經 BAT 通道提問 Q-2（提問時間 2026-10-22T23:58:07+0800）：被限流時從「等到下一枚 token 的秒數」換成 `Retry-After` 正整數秒，用哪種取整、最小值多少。工單明定 Worker 不得自行決定。
- **選項**：
  - [A] `max(1, ceil(秒數))`，秒數 = `(1 - 目前 tokens) / 每秒補充率`（AD-12 草案）
  - [B] `floor(秒數) + 1`：剛好整數秒時多等 1 秒
- **決定**：選 A（使用者決定，4-10 錄製前指定；塔台 2026-10-22T23:58:42+0800 以 `--answer --choice 1` 經 BAT 通道答覆，exit `0`，工單不寫答覆行）。理由：ceil 保證客戶端照 header 等完一定有 token、不會再被拒；`max(1, …)` 保證 header 是正整數（`openapi.yaml` 宣告 integer）。
- **影響**：`internal/ratelimit` 的 `Retry-After` 計算與單元測試、`e2e/ratelimit_test.go` 的斷言依此實作；改取整規則須開新決策取代本條。
- **相關工單**：T0005

### D009 2026-10-22 — 限流桶：容量＝租戶 `rate_limit_per_min`，每秒補 `/60`，上限放租戶設定、預設 60

- **背景**：T0005（yolo）Worker 經 BAT 通道提問 Q-1（提問時間 2026-10-22T23:58:07+0800）：桶容量、補充速率、限額來源與未設定時的預設。`internal/tenant/tenant.go` 原本沒有限額欄位，限額來源須一併決定。
- **選項**：
  - [A] `tenant.Tenant` 加 `RateLimitPerMin`；每個 API key 一桶，容量 = 所屬租戶 `RateLimitPerMin`，補充率 `RateLimitPerMin/60` token/秒（連續補充），桶初始為滿；`<= 0`（未設定）用預設 60
  - [B] 只用全域設定（`ratelimit` 建構時給每分鐘上限，預設 60），租戶不帶欄位
- **決定**：選 A（使用者決定：容量等於租戶的每分鐘上限 `rate_limit_per_min`，每秒補 `rate_limit_per_min/60` 個，上限放在租戶設定、預設 60）；塔台 2026-10-22T23:58:42+0800 以 `--answer --choice 1` 經 BAT 通道答覆，exit `0`，工單不寫答覆行。理由：與 AD-12、`docs/README.md` 資料模型（Tenant 有每分鐘 `rate_limit`）及工單「限額由租戶帶入」一致。
- **影響**：`tenant.Tenant` 新增每分鐘上限欄位；同租戶多把 key 各一桶、各用租戶限額；`e2e/testutil` 可把測試租戶設小限額以少送請求。設定檔載入 story 上線後，租戶設定檔要帶這個欄位；分散式限流不在範圍。
- **相關工單**：T0005

### D008 2026-10-22 — T9004 以模式 ① 機械合併回 `main`

- **背景**：T9004 以 `--isolation worktree` 派出（yolo），Worker 回報 DONE，`ct/T9004` 要合併回 `main`。照 `merge-workorder.md` 判準決定模式。
- **判準**（`--worktree-status T9004`，`source: live`）：① `behind 2`，`base..mainHead` 只有 `_ct-workorders/T9004-onboarding-bat-section.md`（放寬版成立）；② `validated_sha` = `head` = `86e3d7e`；③ `outOfScopeFiles []`、`touchesCtWorkorders false`；④ `conflicts []`；⑤ `dirty []`，主 checkout 髒檔只有 `_tower-state.md`，與 `changedFiles`（`docs/dev-workflow.md`）不重疊。ff 前重查仍成立。
- **決定**：模式 ①，塔台在整合 worktree `..\.ct-integration\sms-platform`（本次首次建立）開 `ct-merge/T9004` 自 `base_main_sha` `94fecfa`，`--no-ff` 合併得 merge commit `6107732`（parent1 `94fecfa`、parent2 `86e3d7e`）；yolo 斷點 E 自動 `--ff-only`，ff 後 `main` = `6107732`。合併結果與 `head` 只差 `_ct-workorders/**`，不重跑驗收。
- **影響**：`--close-worker T9004 --worktree remove` exit `0`，Worker 分頁已結束、T9004 worktree 已移除；整合 worktree 已 `switch --detach`。分支 `ct/T9004`、`ct-merge/T9004` 保留，待使用者同意後以 `git branch -d` 刪除。
- **相關工單**：T9004

### D007 2026-10-22 — `vars` 為 JSON 物件、值只收字串

- **背景**：T0004 讓 `POST /v1/messages` 接受 `template_id`＋`vars`；spine 與 `openapi.yaml` 都沒定 `vars` 的形狀。yolo Worker 以 Q-3 經 BAT 通道問塔台（2026-10-22T23:37:23+0800 答覆，塔台代決）。
- **選項**：
  - [A] `object`，`additionalProperties: {type: string}`；數字、布林、`null`、巢狀回 400 `invalid_request`，訊息不回顯內容
  - [B] 收字串、數字、布林，伺服器端轉成字串
- **決定**：選 A，理由：沒有隱性轉換（例如 `1.0` 轉成 `"1"`），呼叫端行為可預期；日後要放寬可相容地擴充，反過來收緊則是相容性破壞。證據（T0004 回報區〈互動紀錄〉）：DTO 用 `map[string]*string`，`null` 也擋下，固定訊息 `field 'vars' must be an object of string values`。
- **影響**：`docs/openapi.yaml` 的 `SendMessageRequest.vars` 為字串值物件；要接受其他型別須開新決策取代本條。
- **相關工單**：T0004

### D006 2026-10-22 — `ErrInvalidTemplate` 回 400 `invalid_template`

- **背景**：T0004 工單寫 `ErrInvalidTemplate` → 400 `invalid_request`，但 `docs/error-codes.md:14` 與 SPEC 的 error-codes 都定為 400 `invalid_template`（錯誤碼變更屬相容性變更）。Worker 以 Q-2 問塔台；塔台查證 `docs/error-codes.md` 後代決。
- **選項**：
  - [A] 照已文件化的契約回 400 `invalid_template`
  - [B] 照工單回 400 `invalid_request`，差異交塔台
- **決定**：選 A，理由：錯誤碼是對外契約，文件與 SPEC 一致；工單寫法是塔台開單筆誤。證據：T0004 `errors_templates.go` 的 `templateError` 對應 400 `invalid_template`，`httpapi_test.go` 驗信封；塔台收單重跑 `go test ./... -race` 全綠。
- **影響**：T0007 只加 `ErrMissingVariable` → 422，保留本條的 400 對應；開單時錯誤碼以 `docs/error-codes.md` 為準。
- **相關工單**：T0004、T0007

### D005 2026-10-22 — 模板 E2E 暫以 store 讀回的 `Body` 驗證已渲染

- **背景**：T0004 驗收條件寫「用 `FakeProvider` 收到的內容驗證」已渲染，但 `queue.Run` 目前只等 ctx（送信 worker 尚未實作），`testutil` 也沒把 `FakeProvider` 接進 queue，provider 收不到訊息。Worker 以 Q-1 問塔台。
- **選項**：
  - [A] 從 `env.Store.Get(testutil.TenantA, id)` 讀 `Body` 驗證，並斷言 `FakeProvider` 沒收到未渲染內容
  - [B] `GET /v1/messages/{id}` 回應加 `body` 欄位（API 契約變更、檔案不在 `affects_files`）
  - [C] 本單補 queue 送信 worker（超出範圍）
- **決定**：選 A，理由：不擴大範圍、不改 API 契約，仍以真實 handler＋`httptest` 驗到「入佇列前已渲染」（AD-16）。證據：`TestE2E_Template_RendersVariables` PASS（塔台收單重跑）。
- **影響**：工單驗收條件「用 FakeProvider 收到的內容驗證」未達成，屬塔台決定的差異；實作 queue 送信 worker 的工單須把這個 E2E 改成以 `FakeProvider` 收到的內容驗證。
- **相關工單**：T0004

### D004 2026-10-22 — httpapi 每功能一檔，用明確 stub 預留接口

- **背景**：第四章 T0005（限流）、T0006（列表）、T0007（模板缺變數 422）要 yolo 同時派出，`affects_files` 不能重疊。現況 `handler.go`（路由表、`Deps`、`v1` 串接）、`errors.go`（`writeError` 單一 switch）、`messages.go`（發送／查單筆／DTO）會被三張同時改。
- **選項**：
  - [A] 明確 stub：`NewHandler` 預先列好 `registerXxx`，`v1()` 預留 `rateLimit` 直通位置，`writeError` 詢問固定清單 `featureErrorMappers`；各功能填自己的檔。程式裡看得到路由、中介層順序、錯誤對應；代價是新增「全新功能」仍要改一次 `handler.go`／`errors.go`（本章已知的功能由 T9003 預先列好）
  - [B] 套件層級 registry＋各檔 `init()` 自行註冊：新增功能完全不改共用檔；但順序（authenticate → ratelimit）要靠 order 數字，註冊散在 `init()`，讀程式碼看不出路由表與錯誤對應，與 AD-8「`writeError` 單一對應處」的可讀性相悖
  - [C] 不重構，三張工單序列派：不撞檔，但放棄並行，也無法示範 yolo 同時派工
- **決定**：選 A，理由：本章要的功能清單已知，預留接口成本低；明確呼叫讓路由表與中介層順序可一眼讀出，AD-8 的 `writeError` 仍是信封唯一輸出處（mapper 只回 `apiError`，不寫回應）。B 的好處只在「未知的未來功能」，不抵可讀性成本。
- **影響**：T9003 只做結構調整、行為不變；T0005 填 `ratelimit.go`／`errors_ratelimit.go`／`deps.go`，T0006 新增 `messages_list.go`，T0007 填 `errors_templates.go`／`messages_send.go`，並行三單 `affects_files` 不重疊（`cmd/smsd/main.go`、`e2e/testutil/server.go` 只有 T0004 與 T0005 會改，T0004 先做完）。日後出現 stub 清單以外的新功能，才改 `handler.go`／`errors.go` 一次。
- **相關工單**：T9003

### D003 2026-10-22 — 路由維持標準庫 `http.ServeMux`，中介層用函式包裝（A1），不引入 chi

- **背景**：D001 寫了「標準庫真的不夠用時，再開 EXP 比較」。API 草案的路由數量增加到 7 組，觸發 EXP-ROUTER-001 比較標準庫 `http.ServeMux` 與第三方 router（chi）。
- **選項**：
  - [A1] 標準庫，單一 mux，`/v1` 每條路由各自用 `v1(...)` 包中介層；串接約 17 行
  - [A2] 標準庫，`/v1/` 子 mux 掛在頂層 mux，中介層只包一次；串接約 10 行，但未帶 key 的未知路徑與錯方法回 401，而非 404／405
  - [B] chi ＋ `r.Use`／`r.Group`；預期串接約 3–5 行，**未實測**（離線，本機無 chi 原始碼）；`go.mod` 會出現 `require`
- **決定**：選 A1，理由：7 條路由（`/healthz`、`/v1/messages`、`/v1/messages/{id}`、`/v1/messages?status=`、`/v1/templates`、`/v1/templates/{id}`、`/v1/providers/{name}/callbacks`）A 方案實測全部直接表達，沒有需要變通的路由；D001 的啟動條件（標準庫真的不夠用）不成立。A2 少約 7 行，但 `/v1` 下的 401 先於 404／405，與 AD-8 的信封行為不一致，不採。B 的優勢只在省 5–12 行串接，不抵外部依賴成本。證據：EXP-ROUTER-001 實驗紀錄（commit `8f6c58e`；A 實測 `go vet` 乾淨、`go test -count=1` PASS；B 未實測，結論不依賴 B 的數字）。
- **影響**：**不推翻 D001，ARCHITECTURE-SPINE AD-17 維持**；`go.mod` 仍不得有 `require`。404／405 必須走錯誤信封（`envelopeFallbacks`，AD-8）。中介層以 `func(http.Handler) http.Handler` 串接，可用小型 `chain` helper。`/healthz` 不包驗證與限流。日後出現多層版本前綴或每群組不同中介層組合再開新 EXP；要引入 router 須新決策取代本條。T9002 把結論回寫 spine。
- **相關工單**：EXP-ROUTER-001、T9002

### D002 2026-10-22 — API key 只從 header `X-API-Key` 讀，query 帶 key 視為沒帶

- **背景**：T0003 以驗證 middleware 取代 `withTenant` 佔位。key 可以從哪裡讀，決定憑證會不會進入存取日誌、瀏覽器歷史、代理伺服器快取與 `Referer`，也決定錯誤處理的行為（帶了無效 key 與沒帶 key 是否要區分）。工單開立時已把規則寫進硬性規則，由塔台收單時記錄。
- **選項**：
  - [A] 只讀 header `X-API-Key`；query 參數（`?api_key=…`）一律忽略，等同沒帶 → 401 `unauthorized`。key 不會出現在 URL，標準庫的 `r.URL` 日誌與 `httptest` 記錄都看不到它；代價是瀏覽器網址列或簡易 `curl` 連結不能直接帶 key
  - [B] header 與 query 都收（header 優先）：手動測試與 webhook 式連結方便；但 key 會進 URL，落到存取日誌、代理與瀏覽器歷史，違反 AD-15「日誌不記 key」的精神，且兩個來源衝突時要多定一條優先規則
  - [C] header 為主，query 帶 key 時回 400 並提示改用 header：對呼叫端更友善、錯誤更明確；但多一個錯誤碼與分支，且「有人把 key 放進 URL」這件事本身已經讓 key 外洩，回應再提醒也補救不了，並與 `docs/error-codes.md`／`openapi.yaml` 已定義的 401 契約不一致
- **決定**：選 A，理由：憑證不進 URL 是最便宜的防線，且與既有 401 契約完全一致，不新增錯誤碼。證據（T0003 回報區）：〈產出摘要〉記 `auth.go` 僅 `r.Header.Get("X-API-Key")`、query 參數一律忽略；〈驗收證據〉記「`auth.go` 僅 `r.Header.Get("X-API-Key")`；e2e 子測試『key in query string counts as missing』驗證 401」，並列 `go test ./e2e -run 'TestE2E_(Auth_MissingKey401|Tenant_IsolationReturns404)' -v` 兩個測試與全部子測試 PASS；`TestE2E_Auth_MissingKey401` 另涵蓋「不回顯 key」與「被拒不寫入 store」。塔台收單時重跑 `CGO_ENABLED=1 go test ./... -race`、`go vet`、`gofmt -l .` 皆乾淨。
- **影響**：之後所有 `/v1/*` 端點（限流 story 6、模板 story 5／8…）都經同一個 `authenticate` middleware，不得另開 query 取 key 的路徑；`docs/openapi.yaml` 與對外文件只描述 header；手動驗證要用 `curl -H 'X-API-Key: …'`。要改成接受 query，須開新決策取代本條，不改寫本條。
- **相關工單**：T0003

### D001 2026-10-22 — 只用標準庫 `net/http`＋`httptest`，不引入第三方套件

- **背景**：T0001 建 sms-platform 的第一個 Go module 與 `GET /healthz`。這是之後每一章長功能的地基：HTTP 路由與測試寫法在這裡定下來，後面的工單都會沿用。Go 1.22 起標準庫 `http.ServeMux` 已支援「方法＋路徑」（`GET /healthz`）與 `{id}` 路徑參數；`net/http/httptest` 可以在本機起真實的 HTTP server 跑 E2E，不需要網路或帳號。
- **選項**：
  - [A] 只用標準庫：`net/http` 路由、`httptest` 跑 E2E、`testing` 斷言。`go.mod` 沒有 `require`，clone 下來不必下載任何東西就能 `go test`；代價是路由群組、middleware 串接要自己多寫幾行
  - [B] 第三方 router（例如 chi）＋標準庫測試：路由群組與 middleware 現成；但多一個依賴要跟版本，課程錄影後套件改版，學員畫面就會和影片不一致
  - [C] 第三方 router＋斷言套件（例如 testify）：測試寫起來較短；依賴再多一個，學員要同時認兩套 API，偏離「第一章不教 Go 語法」的目標
- **決定**：選 A，理由：骨架只有一條路由，標準庫足夠；沒有依賴，「`go.mod` 無 `require`」就能直接寫成驗收條件（`go list -m all` 只列出 `example.com/sms-platform`），離線、在 dev container 裡都能重現。標準庫真的不夠用時，再開 EXP 比較「標準庫路由 vs 第三方 router」（3-06），結論另記新的決策，不改寫本條
- **影響**：`go.mod` 不得有 `require`；`internal/httpapi` 用 `http.NewServeMux` 註冊路由；`e2e/testutil` 用 `httptest.NewServer` 包真實 handler；之後的工單要加依賴，必須在工單明示並記一條新的決策
- **相關工單**：T0001
