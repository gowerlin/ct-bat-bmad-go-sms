---
schema_version: 1
schema_kind: workorder
id: T9004
title: 新增 docs/dev-workflow.md「第四章：用 BAT 派工」小節（worktree 隔離派工示範）
type: docs
status: DONE
repo: sms-platform
project: sms-platform
created_at: "2026-10-22T23:07:32+0800"
started_at: "2026-10-22T23:50:23+0800"
updated_at: "2026-10-22T23:51:27+0800"
completed_at: "2026-10-22T23:51:27+0800"
sizing: S
affects_files:
  - docs/dev-workflow.md
interaction:
  mode_hint: yolo
  interactive: false
  intervention_type: fire-and-forget
renew_count: 0
workdir: main repo
isolation: worktree
worker_question_timeout: 30
---
# 工單 T9004-onboarding-bat-section

> 檔名沿用 `onboarding-bat-section`；內容已改為新增 `docs/dev-workflow.md`（`docs/onboarding.md` 是給租戶看的接入文件，不放開發流程）。

## 元資料
- **工單編號**：T9004（練習區段，給 4-06 示範 `--isolation worktree` 派工）
- **任務名稱**：新增 `docs/dev-workflow.md`，內含「第四章：用 BAT 派工」小節
- **狀態**：DONE
- **類型**：docs
- **建立時間**：2026-10-22 23:07 (UTC+8)
- **intervention_type**：fire-and-forget
- **isolation**：worktree（工單檔本身一律在主 checkout 修改，所以 `affects_files` 只列 `docs/dev-workflow.md`）
- **affects_files**：`docs/dev-workflow.md`（新增檔）

## 任務指令

### 前置條件
- 讀 `AGENTS.md`；`docs/dev-workflow.md` 目前不存在，由本單新增（`docs/` 內其他文件只讀，用來對齊語氣）

### 輸入上下文
新增檔案 `docs/dev-workflow.md`（開發流程筆記，**只放這一小節**，不要加其他章節、前言或目錄），小節標題 `## 第四章：用 BAT 派工`，**整節含標題五行以內**，只寫三件事、各一句：
1. **派工**：塔台以 `bat-terminal.mjs` 開 Worker 分頁並執行 `/ct-exec T####`
2. **回收**：Worker 回報後，塔台收單、確認後關閉該 Worker 分頁
3. **worktree**：工單標 `isolation: worktree` 時，Worker 在獨立的 git worktree 與分支 `ct/T####` 工作，驗收後再合併回 `main`

不要寫任何權杖、埠號、終端 ID 或個人路徑。

### 驗收條件
- [ ] 只新增 `docs/dev-workflow.md`，內容只有上述小節，≤ 5 行（含標題）；沒有修改任何既有檔案
- [ ] `git diff --name-only <base>..HEAD` 只出現 `docs/dev-workflow.md`
- [ ] 小節內沒有權杖、埠號、終端 ID；回報區的隔離分支證據（`branch`／`base_sha`／`validated_sha`／`head_sha`）填好
- [ ] `go test ./... -race` 全綠、`go vet ./...` 與 `gofmt -l .` 沒有輸出（純文件，結果預期與開工前相同）
- [ ] 回報區填好、`status` 與 body 鏡像一致；產品 commit 訊息含 `T9004`

### 不做
- 不改 `docs/dev-workflow.md` 以外的任何檔案（尤其不碰 `docs/onboarding.md`）；不改 `_ct-workorders/` 底下的塔台私有檔，也不 stage worktree 內的 `_ct-workorders/`
- 不 push；不改 `_bmad/`、`.claude/`；不新增 `go.mod` 的 `require`

## Sub-session 執行指示

> 由塔台以 yolo 模式、`--isolation worktree` 在 BAT 派工（`/ct-exec T9004`）。
> 時間值一律用系統指令取得（`date "+%Y-%m-%dT%H:%M:%S%z"`），不要憑印象填。驗證指令成功跑完、看到輸出之後，才在回報區寫「通過」。暫存腳本寫在 `_bmad-output/tmp/`。

---

## 回報區

> 以下由 sub-session 填寫

### 完成狀態
DONE

### 產出摘要
- 新增 `docs/dev-workflow.md`（worktree 分支 `ct/T9004`，commit `86e3d7e`）：只有 `## 第四章：用 BAT 派工` 一個小節，含標題共 5 行（標題、空行、三條各一句：派工／回收／worktree），LF 結尾。
- 沒有修改任何既有檔案；worktree 內的 `_ct-workorders/` 沒有 stage。
- 著陸檢查：**PASS**
  - C-0：工單 `repo` = `sms-platform`，`basename "$CT_HOME"` = `sms-platform` → PASS
  - C-1：工單位於 `$CT_HOME/_ct-workorders/` → PASS
  - C-3：`docs/dev-workflow.md` 不存在（本單新增），最近的祖先 `docs/` 存在 → present
  - C-2：工單沒有 `branch` 欄位，不比對；目前分支是 `ct/T9004`
  - `CT_HOME`＝`<demo>/sms-platform`，`CT_WORKTREE_BRANCH`＝`ct/T9004`，`BAT_WORKSPACE_ID`＝`<workspace>`（只是證據）
- 驗收（在 `validated_sha` 上執行）：
  - `git diff --name-only d7fd9a2..HEAD` → 只有 `docs/dev-workflow.md` ✅
  - 敏感字串掃描（`grep -nE '[0-9]{3,}|token|port|:[0-9]+|磁碟路徑|/Users/|16 位以上 hex'`）→ 沒有命中（exit 1）✅
  - `go test ./... -race -count=1`（`CGO_ENABLED=1`，MinGW gcc）→ exit 0，所有有測試的套件都 `ok`（e2e、httpapi、provider/mock、queue、store、template、tenant）✅
  - `go vet ./...` → 沒有輸出，exit 0 ✅
  - `gofmt -l .` → 沒有輸出，exit 0 ✅
  - `go build ./...` → exit 0 ✅
- Commit：產品 `86e3d7e`（`ct/T9004`，worktree）；工單：開工 `47c701b`，收單 commit 在主 checkout `main`（`git commit --only` 只帶本工單檔）。沒有 push。

### 互動紀錄
無（yolo，沒有向塔台提問）

### Renew 歷程
無

### 遭遇問題
- 第一次跑 `go test ./... -race` 時 PATH 沒有 gcc，出現 `cgo: C compiler "gcc" not found`。比照 T0004 回報，把 `C:\ProgramData\mingw64\mingw64\bin` 加到 PATH 之前，再設 `CGO_ENABLED=1` 重跑，結果全綠。這是環境問題，跟本單變更無關（T0004 已建議在 AGENTS.md 寫明 MinGW 路徑）。

### sprint-status.yaml 已更新
不適用

### 隔離分支證據
- branch：`ct/T9004`
- base_sha：`d7fd9a2d60ceb4bb1599e50a6de678b7963b7300`
- validated_sha：`86e3d7ea8321cb59f216e8aca550053a77adb3c3`
- head_sha：`86e3d7ea8321cb59f216e8aca550053a77adb3c3`
- validated_sha ≠ head_sha 的說明：不適用（兩者相同）
- 收單時 `git -C "$CT_WORKTREE" status --porcelain` 是空的

### 回報時間
2026-10-22T23:51:06+0800
