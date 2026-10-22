# 租戶接入

## 送出與查詢

```text
POST /v1/messages        {"to":"+886900000001","body":"hi"}   -> 202 {"id":"msg_…","status":"queued"}
GET  /v1/messages/{id}                                        -> 200 {"id","to","status","attempts",…}
```

- `to` 必須是 E.164（`^\+[1-9]\d{1,14}$`）；`body` 不得空白，最多 70 個 Unicode code point。
- 目前（Epic 1）以固定的預設租戶運作，不需要 `X-API-Key`；API key 驗證在 Epic 2 加入。
- 錯誤一律為 `{"error":{"code","message"}}`，清單見 [error-codes.md](error-codes.md)。
- 佇列容量 1000，滿載回 503 `queue_full`，請稍後重試；該次請求不會建立訊息。

## 已知限制

- **重啟後訊息不保留（FR-28）**：資料只存在記憶體。服務重啟後，尚未進入終態的訊息會遺失，查詢回 404 `not_found`；請以自己的紀錄為準，必要時重新送出。
- 終態訊息（`delivered`、`undelivered`、`failed`）保留 24 小時，之後查詢回 404。
- 設定：`LISTEN_ADDR`（預設 `:8080`）。
