# Local Rules

## 驗收「沒有某字串」一律限定追蹤的 Go 檔
- 規則：工單驗收要證明程式碼裡沒有某個字串時，寫 `git grep -n '<樣式>' -- '*.go'`，不要寫 `grep -rn '<樣式>' .`。
- 由來：T0009 的 `grep -rn 'go:build red_' .` 命中塔台自己的文件與 gitignored 暫存，靠 D023 依意圖判定（_learnings.md P1）。
