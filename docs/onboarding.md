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

## 接收狀態通知（Webhook）

供應商回報訊息 `delivered` 或 `undelivered` 時，平台會 `POST` 到你的 `callback_url`（由維運者設定；沒設定就不通知）：

```text
POST <callback_url>
Content-Type: application/json
X-Signature: sha256=<小寫 hex>

{"id":"msg_…","status":"delivered"}
```

- body 固定只有 `id`、`status` 兩個欄位；需要其他資料請用 `GET /v1/messages/{id}` 查。
- `X-Signature` 是用你的 callback secret 對**原始 body bytes** 算的 HMAC-SHA256，小寫 hex，前綴 `sha256=`；不含時間戳。
- 同一則訊息的同一個狀態最多通知一次；同一則訊息的通知依產生順序送出。
- 回 2xx 即視為收到。非 2xx 或連線失敗會在 1 秒、2 秒、4 秒後重試，共 4 次；用盡就放棄（只記在平台日誌），訊息狀態不受影響。

驗證範例（Go，只用標準庫；`test-secret` 是假密鑰，換成維運者給你的值，不要寫死在程式碼裡）：

```go
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
)

var callbackSecret = []byte("test-secret") // 假密鑰；實際從環境變數或密鑰管理讀取

func verify(body []byte, header string) bool {
	got, ok := strings.CutPrefix(header, "sha256=")
	if !ok {
		return false
	}
	gotMAC, err := hex.DecodeString(got)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, callbackSecret)
	mac.Write(body)
	return hmac.Equal(gotMAC, mac.Sum(nil)) // constant-time，不要用 == 比字串
}

func webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	if err != nil || !verify(body, r.Header.Get("X-Signature")) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	// 先驗簽章、再解析 JSON；用 id + status 去重，處理完回 2xx。
	w.WriteHeader(http.StatusNoContent)
}

func main() {
	http.HandleFunc("POST /sms-status", webhook)
	_ = http.ListenAndServe("127.0.0.1:9000", nil)
}
```

注意：一定要對收到的**原始** bytes 驗證，不要先 `json.Unmarshal` 再重新編碼後才算 HMAC。

## 已知限制

- **重啟後訊息不保留（FR-28）**：資料只存在記憶體。服務重啟後，尚未進入終態的訊息會遺失，查詢回 404 `not_found`；請以自己的紀錄為準，必要時重新送出。
- 終態訊息（`delivered`、`undelivered`、`failed`）保留 24 小時，之後查詢回 404。
- 設定：`LISTEN_ADDR`（預設 `:8080`）。
