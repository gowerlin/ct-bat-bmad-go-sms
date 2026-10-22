---
schema_version: 1
schema_kind: workorder
id: T9002
title: 把 D003 路由結論回寫 ARCHITECTURE-SPINE 路由段
type: doc
status: DONE
repo: sms-platform
project: sms-platform
created_at: "2026-10-22T19:14:02+08:00"
updated_at: "2026-10-22T19:17:00+08:00"
sizing: S
affects_files:
  - _bmad-output/planning-artifacts/architecture/architecture-sms-platform-2026-10-22/ARCHITECTURE-SPINE.md
  - _ct-workorders/T9002-spine-routing-writeback.md
interaction:
  mode_hint: off
  interactive: false
  intervention_type: fire-and-forget
started_at: "2026-10-22T19:16:00+08:00"
completed_at: "2026-10-22T19:17:00+08:00"
renew_count: 0
workdir: main repo
---
# 工單 T9002-spine-routing-writeback

## 元資料
- **工單編號**：T9002（課程把 T0004 留給第四章的模板工單；練習與額外的單從 T9002 起）
- **任務名稱**：把 D003 路由結論回寫 `ARCHITECTURE-SPINE.md` 的路由段
- **狀態**：DONE
- **類型**：doc
- **建立時間**：2026-10-22 19:14 (UTC+8)
- **來源**：EXP-ROUTER-001（CONCLUDED）、D003
- **affects_files**：
  - `_bmad-output/planning-artifacts/architecture/architecture-sms-platform-2026-10-22/ARCHITECTURE-SPINE.md`
  - `_ct-workorders/T9002-spine-routing-writeback.md`

## 任務指令

### 前置條件
- 讀 `AGENTS.md`、`_ct-workorders/_decision-log.md`（D001、D003）、`_ct-workorders/EXP-ROUTER-001-stdlib-mux-vs-chi.md`（〈結論〉與〈實驗紀錄〉）
- 讀 spine 的 AD-8（錯誤信封，含 `ServeMux` 404／405 攔截）與 AD-17（標準庫優先）

### 輸入上下文
spine 沒有獨立的「路由」章節，路由相關規則散在 AD-8 與 AD-17。D003 的結論：路由維持標準庫 `http.ServeMux`，中介層用函式包裝＋`chain` helper（A1：單一 mux、`/v1` 各路由各自包 `v1(...)`），不引入 chi，不推翻 D001／AD-17。

要回寫的內容（以 AD-17 為主，AD-8 只在必要處加交叉引用）：
1. AD-17 `Rule` 補路由寫法：使用 Go 1.22+ 的「方法＋路徑」與 `{id}`（`r.PathValue`）；中介層以 `func(http.Handler) http.Handler` 串接，可有小型 `chain` helper；`/v1` 路由各自包驗證與限流（A1），`/healthz` 不包；不使用子 mux 掛 `/v1/`（A2），原因：未帶 key 的未知路徑會回 401 而非 404／405，與 AD-8 不一致
2. 標示來源：`D003`、`EXP-ROUTER-001`；註明方案 B（chi）**未實測**，只是紙上對照，結論不依賴 B 的數字
3. 保留既有「`go.mod` 不得出現 `require`、要加依賴須先修訂本 spine」，不得弱化
4. 若 AD-17 的 `Prevents` 或其他段落與新增文字重複，只刪重複，不改其他 AD 的意思

### 驗收條件
- [ ] 只改 `affects_files` 列出的檔案；`git diff --name-only <base>..HEAD` 核對
- [ ] AD-17 含 A1 寫法、不採 A2 的原因、D003／EXP-ROUTER-001 來源、B 未實測的註記
- [ ] 沒有改 AD 編號、沒有新增 AD、沒有動 `[ADOPTED]`／`[ASSUMPTION]` 標記語意
- [ ] 沒有在 spine 寫沒有實驗支撐的數字（只引用 EXP 紀錄的 A1 約 17 行、A2 約 10 行）
- [ ] 回報區填好、`status` 與 body 鏡像一致；commit 訊息含 `T9002`

### 不做
- 不改程式碼、`go.mod`、測試、`docs/openapi.yaml`
- 不改 `_ct-workorders/` 底下的塔台私有檔（`_tower-state.md`、`_decision-log.md`…）與 EXP 單
- 不 push；不改 `_bmad/`、`.claude/`

## Sub-session 執行指示

> 在 `sms-platform/` 執行 `/ct-exec T9002`（目前未派發，由塔台之後決定何時派）。
> 時間值一律用系統指令取得。

---

## 回報區

> 以下由 Worker 填寫，塔台不在這裡編輯

### 完成狀態
DONE（完成時間 2026-10-22T19:17:00+08:00）

### 產出摘要
- 只改 `ARCHITECTURE-SPINE.md` 的 AD-17：Rule 補路由寫法（方法＋路徑、`{id}`／`r.PathValue`、`func(http.Handler) http.Handler`＋`chain`、A1 `/v1` 各路由包 `v1(...)`、`/healthz` 不包、不採 A2 的原因並交叉引用 AD-8）；新增 `Source` 行（D003、EXP-ROUTER-001、A1 約 17 行／A2 約 10 行、B 未實測且結論不依賴 B）。
- 保留「`go.mod` 不得出現 `require`、要加依賴須先修訂本 spine」原文。`Prevents` 與新增文字無重複，未刪除任何內容；AD-8 未改。

### 驗收證據
- Landing：C-0 PASS（`repo`=sms-platform，basename(REPO_ROOT)=sms-platform）；C-1 PASS；C-3 present；未在 worktree（`CT_HOME` 未設）；BAT_* 環境變數未見。
- `git diff --name-only 0b32a9f..HEAD`（product commit 後）只有 spine 一個檔案；工單檔於收單 commit 加入。
- AD 編號、`[ADOPTED]`／`[ASSUMPTION]` 標記未動；只引用 EXP 紀錄的 17／10 行，無新數字。
- Commit：product `006d2da`（訊息含 T9002）。不需 `make test`（純文件）。未 push。

### 遭遇問題
無。
