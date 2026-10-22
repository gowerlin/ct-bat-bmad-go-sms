# 詞彙表

全 spec 與下游產物逐字使用，不引入同義詞。

- **租戶（Tenant）**：一個接入團隊；資料互相不可見。
- **API key**：租戶憑證，`X-API-Key` header；屬一個租戶。
- **訊息（Message）**：一次發送請求；全域唯一 `id`（前綴 `msg_`），屬一個租戶。
- **狀態（Status）**：`queued`、`sending`、`sent`、`delivered`、`undelivered`、`failed`。終態：`delivered`、`undelivered`、`failed`。
- **Provider**：實際送簡訊的國內第三方；經 Provider 介面接入。v1 有 `mock`、`mitake`。
- **Adapter**：實作 Provider 介面的模組。
- **佇列（Queue）**：暫存待送訊息，介面抽象；v1 為 in-process。
- **模板（Template）**：含 `{{變數}}` 的內容，屬一個租戶（`id` 前綴 `tpl_`）。
- **Webhook**：閘道 → 租戶的 Status 變化通知。
- **Provider 回呼（Provider Callback）**：Provider → 閘道的送達結果；路徑 `POST /v1/providers/{name}/callbacks`。與 Webhook 是兩件事。
- **維運者（Operator）**：閘道團隊中以設定檔管理租戶、API key、Webhook URL、簽章密鑰、限流上限與 `PROVIDER` 的人；租戶不能自助。
- **假對端（Fake Peer）**：模擬 Provider、經網路的測試替身，`mitake` 只連它；`mock` 不經網路。
- **簽章（Signature）**：以 HMAC 計算、讓租戶確認通知來自閘道的驗證值。
- **Outbox**：`store` 內待送 Webhook 通知的列（`pending→delivering→done|abandoned`）。
