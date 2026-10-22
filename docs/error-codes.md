# 錯誤碼

所有 4xx／5xx 回應的 body 一律為信封：

```json
{"error":{"code":"invalid_request","message":"field 'to' is required"}}
```

`code` 是穩定的小寫底線字串。**清單的新增、移除或語意變更都視為相容性變更。** 信封只由 `internal/httpapi` 的單一 `writeError` 產生。

| HTTP | code | 觸發 | 本版已實作 |
|---|---|---|---|
| 400 | `invalid_request` | 缺必填欄位、號碼非 `^\+[1-9]\d{1,14}$`、內容超過 70 個 Unicode code point、JSON 無效、非法 `status` 篩選、回呼帶 `delivered`／`undelivered` 以外的值或供應商無法解析、405 方法不符（帶 `Allow`） | 是 |
| 400 | `invalid_template` | 模板無法解析 | 是 |
| 401 | `unauthorized` | 缺、無效或停用 `X-API-Key`；Provider 回呼驗證失敗 | 是 |
| 404 | `not_found` | 訊息、模板、回呼對應不到（含尚未寫入 `sent` 的訊息）；回呼路徑 `{name}` 不是啟用的供應商；跨租戶存取（不用 403）；未知路徑；重啟或過期（終態滿 24 小時）後的訊息 | 是 |
| 422 | `missing_template_variable` | 帶 `template_id` 缺變數 | 是 |
| 429 | `rate_limited` | 超過 API key 每分鐘上限，帶 `Retry-After` | 是 |
| 500 | `internal_error` | 未知錯誤；message 固定為 `internal error`，不洩漏內部資訊 | 是 |
| 503 | `queue_full` | 佇列滿載（容量 1000）；不建立訊息 | 是 |

`queue_full` 也用於關機中（`queue.ErrClosed`），message 為 `service shutting down`。

規則：405 以 `invalid_request` 回覆並帶 `Allow` header；錯誤訊息不回顯收件號碼與簡訊內容。
