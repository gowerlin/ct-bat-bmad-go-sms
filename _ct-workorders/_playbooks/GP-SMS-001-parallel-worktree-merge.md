# Playbook: 並行隔離單共改同一檔 → 模式 ② 合併

> 驗證次數：1（最近：T0011）
> 置信度：🟡 實驗中（來源 pattern 僅 🔵，未達 🟢 晉升門檻；見下方「晉升條件」）
> 首次記錄：2026-10-23
> 範圍：project
> 來源 Pattern：SEQ1 + P4（from _learnings.md）
> Skill 候選：否

## 適用場景
- 兩張以上隔離單（`isolation: worktree`）的 `affects_files` 有完全相同的路徑，且你決定接受衝突，不用 `depends_on` 序列化。
- 衝突是機械式的：兩邊意圖互補，改的是同一段的不同面向（例：一邊管「用哪個 provider」，一邊管「queue 拿到 provider」）。
- 衝突檔數量少（樣本中只有 `cmd/smsd/main.go` 一個）。

## 不適用場景
- 兩邊改的是同一行的相互矛盾語意。這種要序列化，或先開決策。
- 衝突檔多，或牽涉測試斷言取捨。模式 ② 的合併工單不得手動放寬任何測試。
- 兩張單彼此有資料或介面依賴。這種用 `depends_on`。

## 序列定義

### 工單 1：隔離單 A（預估 ~20 min，L 級）
- 命名：`T{N}-<feature-a>`，`isolation: worktree`
- Session 建議：🆕 新 Session（BAT 隔離派工）
- 包含：功能實作加 code review；〈合併注意〉段寫明衝突檔與自己要改的行

### 工單 2：隔離單 B（預估 ~30 min，L 級，與工單 1 並行）
- 命名：`T{N+1}-<feature-b>`，`isolation: worktree`
- Session 建議：🆕 新 Session
- 包含：同上；〈合併注意〉指向工單 1 的衝突點

### 工單 3：合併工單（預估 ~3～5 min）
- 命名：`T{N+2}-merge-<A>`，`kind: merge`，不隔離
- 派發時機：先收單者以模式 ① 合回 `main` 之後，後收單者的 `--worktree-status` 判準不成立（`conflicts` 非空）
- 包含：整合 worktree 自最新 `main` 開 `ct-merge/<id>`，`--no-ff` 合併後者，解衝突，跑完整驗收

## 品質關卡
- 第二張單派發時 `affects_files` overlap 回 🚨 BLOCK → 需使用者回「確認接受衝突」才派（D012 的做法）
- 兩張隔離單各自收單 → 乾淨匯出重跑 build／vet／gofmt／`go test ./... -race -count=1`
- 先收單者 `--worktree-status` 五條判準全過 → 模式 ① 機械合併
- 後收單者判準 1（behind）或 4（conflicts）不成立 → 開合併工單，不手動改 `ct/` 分支
- 合併工單收單 → 驗 SHA：parent1／parent2 與預期一致、範圍外檔案為空、塔台乾淨匯出獨立重驗全綠
- 全部成立 → 斷點 E 自動 `--ff-only` 到 `main`

## 工單模板
- 兩張隔離單的〈合併注意〉：
  `衝突檔：<path>。本單改：<行／面向>。另一張（T####）改：<行／面向>。預期解法：兩邊都保留。`
- 合併工單的回報要求：衝突檔逐一說明（各邊意圖、解法、理由）、parent1／parent2、驗收跑在哪個 SHA、範圍外檔案。

## 注意事項
- 先合哪一張：樣本中是先收單者先合；後合者的測試檔與 `ct/` head 要與合併結果一致，合併工單不得改任何測試。
- 合併完成清理時，`--close-worker <id> --worktree remove` 在樣本中兩次回 exit `3`，改手動 `git worktree remove` exit `0`；原因未明（`_learnings.md` Episodic E1）。
- `git commit --no-edit` 會把 `# Conflicts:` 兩行帶進 merge commit 內文。不影響產品樹，不 amend。
- 同一檔被兩邊改時，Worker 的〈遭遇問題〉會先提醒預期衝突。T0010 已寫在 Followups，這點可直接給合併工單參考。

## 驗證歷史
| 執行 | 專案 | 工單 | 結果 | 時間 |
|------|------|------|------|------|
| 1 | sms-platform | T0008、T0010、T0011 | 成功；僅 `cmd/smsd/main.go` 衝突 | 02:22～03:01，約 40 min |

## 晉升條件
- 🟡→🟢：再在另一次並行共改中驗證（同專案或跨專案皆可），最好能補上「衝突檔 ≥ 2 或有語意取捨」的變體。
- ⭐：3+ 驗證加使用者認可。

## Skill 晉升紀錄
（無）
