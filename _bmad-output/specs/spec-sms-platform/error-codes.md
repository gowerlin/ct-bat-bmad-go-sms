# 錯誤契約

信封：`{"error":{"code":"…","message":"…"}}`。`code` 為穩定小寫底線字串；清單變更視為相容性變更。信封只在 `httpapi` 的單一 `writeError` 產生，領域套件只回 sentinel error（AD-8）。

| HTTP | code | 觸發 |
| --- | --- | --- |
| 400 | `invalid_request` | 缺必填欄位、號碼非 `^\+[1-9]\d{1,14}$`、內容超過 70 code point、非法 `status` 篩選、回呼帶 `delivered`／`undelivered` 以外的值、405 方法不符（帶 `Allow`） |
| 400 | `invalid_template` | 模板無法解析（佔位符括號不成對） |
| 401 | `unauthorized` | 缺、無效或停用 `X-API-Key`；Provider 回呼驗證失敗 |
| 404 | `not_found` | 訊息、模板、回呼對應不到；跨租戶存取（不用 403）；未知路徑；重啟或過期後的訊息；回呼路徑 `{name}` 不等於啟用的 Provider |
| 422 | `missing_template_variable` | 帶 `template_id` 缺變數；message 列出缺少的變數名 |
| 429 | `rate_limited` | 超過 API key 每分鐘上限；帶 `Retry-After`（`max(1, ceil(秒))`） |
| 500 | `internal_error` | 未知錯誤；message 為固定字串，不洩漏堆疊 |
| 503 | `queue_full` | 佇列滿載；不建立訊息（PRD FR-26 最低清單未列，由 AD-14 新增） |

規則：
- 各 FR 的 404 一律 `not_found`，401 一律 `unauthorized`。
- 驗證順序：authenticate → ratelimit → handler；無效 key 直接 401，不計限流。
- Provider 回呼端點不經 API key、不計限流。
- 測試須遍歷所有已定義錯誤路徑確認格式一致。
