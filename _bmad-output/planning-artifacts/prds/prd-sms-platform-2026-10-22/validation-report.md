# Validation Report — 內部 SMS 閘道（sms-platform）PRD

- **PRD:** `_bmad-output/planning-artifacts/prds/prd-sms-platform-2026-10-22/prd.md`
- **Rubric:** `.claude/skills/bmad-prd/assets/prd-validation-checklist.md`
- **Run at:** 2026-10-22T17:10:30+0800
- **Grade:** Fair

## Overall verdict
這份 PRD 的驗收條件寫得紮實（幾乎每個 FR 都有可測的狀態碼、次數與錯誤碼），範圍取捨與限制（重啟丟佇列、不得承載驗證碼、不做 failover）誠實地寫在願景、非目標與 FR-28，假設也完整索引回扣。
風險在策略層：v1 只有 `mock` 與假對端、佇列重啟即丟，既不能承載真實流量，也沒有解決觸發本案的「驗證碼斷線 2 小時」事故；成功指標量的是「準備度」與「測試全綠」，不是論點本身是否成立；而最大的決策風險（自建對比現成方案，§8 Q2）仍未解，文件卻標為 `status: final`。補上幾個行為缺口（冪等、Webhook 順序／逾時、FR-22 與 `mock` 的矛盾）與少量術語整理後，可交付下游。

## Dimension verdicts
- Decision-readiness — adequate
- Substance over theater — adequate
- Strategic coherence — adequate
- Done-ness clarity — strong
- Scope honesty — strong
- Downstream usability — adequate
- Shape fit — strong

## Findings by severity

### Critical (0)

### High (2)

**[Decision-readiness]** — 最大決策風險未解，文件卻標為 final（frontmatter status、§8 Q2、Q1）
Q2 若選現成方案，FR 範圍整個重來；Q1「負責人待指定」使「向五個租戶說明 v1 沒有解決原始事故」沒有人負責，卻是「第一個租戶換接前」的前置。
Fix: 將 Q2 結論列為進入 final 或架構開工的明確閘門（加 `[NOTE FOR PM]`：Q2 未結前，架構與 Story 拆分僅屬暫定），並為 Q1 指定負責人與期限，或把 status 改為 draft／conditional。

**[Strategic coherence]** — v1 無法驗證自己的論點，且不處理觸發事故（§1、§5、SM-4、§8 Q1）
v1 沒有真實 Provider、重啟丟失全部訊息紀錄，SM-2 限定「僅限程序存活期間」；驗證碼事故明說「不會因 v1 而消失」。PRD 誠實，但沒說清楚 v1 要證明什麼、證據是什麼。
Fix: 在 §6 用一句話寫出 v1 的真正目的（驗證介面、重試與回呼語意、租戶隔離，為接入真實 Provider 鋪路），並把 §6.2 的啟動條件（Q5）轉成可決定的里程碑，否則「永久的臨時方案」就是現實。

### Medium (6)

**[Substance over theater]** — 容量與時限類 NFR 全數延後或缺席（FR-8、FR-12、FR-16、§8 Q8）
重試間隔、Webhook 逾時、通知重試排程、限流視窗、預設上限、峰值吞吐都沒有數字；約 30 萬則／月（平均約 0.1 則／秒）說明容量並非難點，但 PRD 沒有據此給 v1 容量上界。
Fix: 至少為 Webhook 逾時、通知重試總時間、佇列上限量級寫出 `[ASSUMPTION]` 數值；明說哪些延後到架構階段是刻意的。

**[Strategic coherence]** — 成功指標多量測「準備度」或「測試通過」，不是論點本身（SM-1、SM-3、SM-5）
SM-1 的基準是 §8 Q6 的差異清單，而該清單目前沒有；SM-3、SM-5 本質是測試全綠，與 FR-20、FR-22 重複；SM-2 沒有數字目標。
Fix: 讓 Q6 成為 SM-1 的前置；SM-3、SM-5 降為完成門檻；為 SM-2 補可觀察量（例如「事故排查只需查閘道」的演練結果）。

**[Done-ness clarity]** — FR-22 要求以 `mock` 跑重試與簽章全路徑，但 `mock` 的失敗行為未定義（FR-22、FR-8、FR-24、詞彙表「假對端」）
FR-22 要求 FR-8 的跨模組測試經 `mock` 走完整路徑，但 FR-8 驗收依賴「假對端連續回暫時失敗」，詞彙表又說 `mock` 不經網路、與假對端不同。工程師無法判斷重試 E2E 該用哪個。
Fix: 擇一：規定 `mock` 可設定回應腳本（成功／暫時失敗／永久失敗），或將 FR-22 路徑改為「`mock` 或假對端」。

**[Done-ness clarity]** — FR-28 對「終態訊息」的命運沒有交代（FR-28、§5、SM-2）
本文說重啟丟失佇列與記憶體內的訊息紀錄，驗收只測「尚未進入終態」者回 404；終態訊息同在記憶體中，理應也是 404，且 FR-27 的失敗原因會一併消失。
Fix: 驗收改為「重啟前建立的所有 `id`（含終態）回 404」，或明寫終態紀錄保留行為並與 Q8 對齊。

**[Done-ness clarity]** — Webhook 通知的順序、逾時與重啟行為未定（FR-11、FR-12、UJ-1）
UJ-1 寫「先後收到 `sent` 與 `delivered`」，但 `sent` 通知若因非 2xx 在重試，`delivered` 可能先到；FR-10 又允許對 `sending` 直接回呼 `delivered`。通知重試在重啟後同樣丟失，PRD 沒說。
Fix: FR-11／12 加一條：通知帶單調序號或時間，或明定「不保證順序，租戶以 `id` 加 `status` 處理」；寫明通知重試受 FR-28 限制。

**[Scope honesty]** — 重送與冪等性被默默漏掉（FR-1、FR-28、§5）
FR-28 要求租戶自行處理「已收到 202 但訊息遺失」，實務就是重送；但 `POST /v1/messages` 無冪等鍵，Provider 已送出時重送會造成重複簡訊。既不在需求，也不在非目標。
Fix: 將「不提供冪等／去重」列入 §5 非目標並在租戶接入文件警示，或新增冪等鍵 FR。

### Low (6)

**[Strategic coherence]** — 模板（4.5）與限流（4.6）與論點的連結未說明（§1、4.5、4.6）
願景只談重試與回呼，模板與限流看起來像「有人想要的功能」。
Fix: 在 4.5、4.6 各加一句其與論點或事故的關係，或標示為支援性功能。

**[Done-ness clarity]** — 模板渲染後長度與 FR-1 的 70 字上限互動未定（FR-1、FR-14）
FR-14 在入佇列前渲染，但沒說渲染後超長怎麼辦。
Fix: FR-14 加一條渲染後超長回 400（`invalid_request` 或 `invalid_template`）。

**[Scope honesty]** — 開放項目數量與 final 狀態並存（§8、§9）
Q1、Q3、Q6、Q8 都沒有負責人或期限，只有 Q2 有。
Fix: 每題標註「阻擋什麼、由誰、何時」，其餘標為「不阻擋」。

**[Downstream usability]** — FR-29（法遵：發送紀錄不外送）被放進 4.9「錯誤格式」（§4.9、§0）
主題不符，Story 拆分易被誤歸到錯誤處理 Epic；FR-29 的 JTBD 與 §5 的 CPaaS 排除原因分散三處。
Fix: 移到獨立小節「合規與資料邊界」，或併入 4.2。

**[Downstream usability]** — 跨功能 NFR 沒有編號（§4.9）
「500 `internal_error` 不洩漏堆疊」「日誌不記錄內容與 API key」無 ID，無法被 Story 或測試引用，也不在 FR-22 範圍。
Fix: 編為 NFR-1、NFR-2 並給驗收。

**[Downstream usability]** — 術語漂移（§3、§8、addendum）
詞彙表聲明不用「供應商」，addendum 仍用；「狀態」與「Status」混用；「發送紀錄」「接入」「換接」「遷移」並用；「接入檢核表」「租戶接入文件」未定義；§6.2 稱 Open Questions 而章節為「待決問題」。
Fix: 統一為詞彙表用語並補「接入檢核表」定義。

## Mechanical notes
- ID 連續性：FR-1 至 FR-29 連續且唯一；FR-27、28、29 插入 4.1、4.3、4.9，§0 已說明。SM-1 至 SM-5、SM-C1、SM-C2 連續；UJ 僅 UJ-1。
- 交叉參照：對 §8 Q 編號的參照皆可解析；FR-22 僅列 FR-1、7、8、10、11、12，未含 FR-27、28；FR-22 說「4.1 至 4.7」與 §6.1「八項功能」計法不同。
- 假設索引往返：§9 的 13 條與行內 `[ASSUMPTION]` 逐一對上，無遺漏或多餘。
- UJ 主角：UJ-1 有名有姓（小林）並內嵌背景，合格。
- 必要章節俱全；缺獨立 NFR 章節（內容分散在 FR 與 §4.9）。

## Reviewer files
- `review-rubric.md`
