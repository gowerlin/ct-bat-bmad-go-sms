# Local Rules

## 驗收「沒有某字串」一律限定追蹤的 Go 檔
- 規則：工單驗收要證明程式碼裡沒有某個字串時，寫 `git grep -n '<樣式>' -- '*.go'`，不要寫 `grep -rn '<樣式>' .`。
- 由來：T0009 的 `grep -rn 'go:build red_' .` 命中塔台自己的文件與 gitignored 暫存，靠 D023 依意圖判定（_learnings.md P1）。

## 收單檢查：改到 API 行為的工單要同步文件
- 規則：工單改到 API 行為（路由、請求／回應欄位、狀態碼、錯誤碼、標頭）時，同一張工單要同步 `docs/openapi.yaml` 與 `docs/error-codes.md`（含「本版已實作」欄），並把這兩個檔列進 `affects_files`；塔台收單時看 diff 有沒有這兩個檔，沒有就不算收完，回 Worker 補（Renew）。
- 由來：CH6 結業道館（T0243）發現 `docs/error-codes.md` 的「本版已實作」欄沒有隨實作工單回寫，`openapi.yaml` 也停在 `0.0.1-skeleton`，到 CH6 才一次補齊（課程製作的塔台決策 D192）。
