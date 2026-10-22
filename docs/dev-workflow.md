## 第四章：用 BAT 派工

- **派工**：塔台以 `bat-terminal.mjs` 開 Worker 分頁，並在分頁中執行 `/ct-exec T####`。
- **回收**：Worker 回報後，塔台收單並確認結果，再關閉該 Worker 分頁。
- **worktree**：工單標 `isolation: worktree` 時，Worker 在獨立的 git worktree 與分支 `ct/T####` 工作，驗收後再合併回 `main`。
