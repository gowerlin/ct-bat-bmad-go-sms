# sms-platform 多代理工作流（講師版，6-06 結業道館參考解）

> 適用：Control Tower v5.12.2 · BAT 0.7.1-pre.6 · BMad-Method v6.12.4 · Claude Code。每區塊四行：規則／參考／實例／例外。
> 原料：1-03 對照表、3-07 品質閘門、3-08 Playbook 草稿、4-07 派工前五要件、5-08 合併三模式、6-03 FIELDGUIDE。

## 1. 工單模板片段
- 規則：frontmatter 必有 `repo`、`affects_files`、`worker_question_timeout`；驗收條件寫成 `TestE2E_*` 測試名，寫不出測試名就不開單。
- 參考：`cr/work-order-template.md`（frontmatter 是 SoT、body 是鏡像）；1-03、3-02。
- 實例：T0012 由回顧 F1 開單，驗收寫「走真實組裝根」的 202／401／404／429 四個情境，`affects_files` 列 12 個路徑（D025）。
- 例外：調查型工單（`type: research`）沒有測試名可寫，改寫「交付結論＋建議工單」，不派 yolo。

## 2. auto-session 對照表
- 規則：`fire-and-forget` 且 `affects_files` 不重疊 → yolo＋`--no-interactive`；要使用者選方案 → ask；範圍未定 → 先開 research 單。
- 參考：`cr/auto-session.md`（四階 off／on／ask／yolo、派工旗標）、`cr/yolo-mode.md`（斷點 A～E）；4-03、4-07。
- 實例：T0009 主 checkout yolo 一次 DONE、無提問；T0010 yolo 提問 Q-4 逾時 `expired` → BLOCKED → Renew #1（D020）。
- 例外：會改範圍或對外契約的提問，塔台不代答，一律升級給使用者（T0010 Q-4）。

## 3. worktree 準則
- 規則：會改共用檔、和別張並行或要跑很久 → `--isolation worktree`；合併依 `--worktree-status` 五條判準，不成立就開 `kind: merge` 合併工單。
- 參考：`cr/merge-workorder.md`（模式 ①／②／③、SHA 比對五步）；4-06、5-08；Playbook GP-SMS-001。
- 實例：T0008、T0010 並行隔離，T0010 模式 ① 合併 `1b0dabb`；T0008 衝突在 `cmd/smsd/main.go`，走 T0011 模式 ②（D021、D022）。
- 例外：兩張單有介面依賴時不並行，改 `depends_on` 序列（T0013 等 T0012，D025）。

## 4. 收單門檻
- 規則：`go test ./... -race -count=1` 全綠、`go vet ./...` 與 `gofmt -l .` 無輸出、diff 只含 `affects_files`；塔台乾淨匯出後重跑一次。
- 參考：`cr/work-order-template.md`（驗收條件、回報區）；3-07 品質閘門、3-08 Playbook 草稿、5-01。
- 實例：T0009 收單時 `grep -rn 'go:build red_' .` 命中塔台文件，依意圖判定（D023），之後寫成本地規則「限定追蹤的 Go 檔」。
- 例外：主機沒有 cgo 時在 dev container 跑 `-race`，回報區寫明替代環境；不得拿掉 `-race` 當作通過。

## 5. 決策日誌規範
- 規則：凡是選項之間的取捨都記 D###（背景／選項／決定／影響／相關工單），連號不跳號；推翻舊決定要開新決策，不改舊的。
- 參考：`cr/decision-log-system.md`；3-05。
- 實例：D003 依 EXP-ROUTER-001 維持標準庫路由，明寫「不推翻 D001」；D026 由塔台定案 `SMS_CONFIG` 欄位名（AD-11 只給概念）。
- 例外：Worker 不寫決策日誌；Worker 的取捨寫回報區，塔台收單時決定要不要記成決策。

## 6. FIELDGUIDE 一頁
- 規則：repo 根目錄 `FIELDGUIDE.md`，每章 L1 設計哲學／L2 決策樹／L3 專案快照；新套件或新錯誤碼收單後跑一次 `/ct-fieldguide audit`。
- 參考：`ct-fieldguide/references/fieldguide-format.md`（三層格式、L3 路徑必須真實）；`cr/fieldguide-commands.md`；6-03。
- 實例：`/ct-fieldguide init` 產出套件邊界、測試規約、錯誤碼三章；隨後 audit 找到 4 處行號偏差與 3 條 `[FG-CANDIDATE]`，沒有規則被程式碼反駁。
- 例外：本專案沒有 UI，不產 DESIGN.md；API 回應格式寫在 FIELDGUIDE 第 3 章與 `docs/error-codes.md`。
