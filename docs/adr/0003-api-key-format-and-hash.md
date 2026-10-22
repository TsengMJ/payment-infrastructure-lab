# 0003 API key 格式與雜湊

- 狀態：Accepted
- 日期：2026-10-22
- 相關迭代：I-01.02、I-01.03

## 背景

商戶程式要用密鑰呼叫 `/v1`。資料庫外洩時，攻擊者不能直接拿儲存的值去呼叫 API。驗證又必須在每次請求時查出是哪一把 key，所以雜湊要能當查詢條件。

這次不做輪替、撤銷、權限範圍，也不把驗證結果放進 Redis。

## 選項

1. **整把 key 做 SHA-256，用雜湊當查詢鍵。** 實作短。代價是不能先用一個公開 id 找出那一列，再對 secret 做 constant-time 比較；以後要顯示「是哪一把 key」也只能靠雜湊。
2. **bcrypt 或 argon2。** 適合人想出來的密碼。API key 有 256 bit 熵，慢雜湊擋不住線上猜測，salt 又讓索引查詢做不到，每次請求得掃表。
3. **`sk_<mode>_<key_id>_<secret>`。** `key_id` 與 secret 各自用 CSPRNG 產生。資料庫存 `key_id`、secret 的 SHA-256 與最後 4 碼。用 `key_id` 找出一列，再用 constant-time 比較雜湊。

## 決定

採用選項 3。

- Token 固定 74 字元：`sk_test_` 或 `sk_live_`（各 8 字元）、22 字元 `key_id`、一個 `_`、43 字元 secret。`key_id` 是 16 byte，secret 是 32 byte，都是 base64.RawURLEncoding。兩段都可能含 `_`，解析用固定長度，不用 `strings.Split`。
- `secret_hash` 是 secret 原始 32 byte 的 SHA-256。比對用 `subtle.ConstantTimeCompare`。找不到列時仍對一段同長度的 digest 做一次比較。
- 明文只在建立時回傳一次，不進資料表，也不進日誌欄位或錯誤字串。
- 行程的 `PAYMENT_MODE`、token 前綴、資料列 `mode` 必須一致。前綴與行程模式不同就拒絕，不查資料庫。雜湊比完才檢查資料列的模式。
- 只有 `active` 商戶可以建立 key。`suspended` 與 `closed` 不能。
- 驗證失敗、商戶不存在、資料庫故障是三種錯誤。後者不能被說成 invalid key。
- `name` 讓商戶分辨多把 key。不唯一。
- Redis 這次只建立連線。驗證不讀 Redis，Redis 故障不改變驗證結果，`/readyz` 也不看 Redis。

## 後果

- 好處：資料庫外洩後沒有可用的 secret。查詢是主鍵讀取，不是掃表。test 與 live 不會互相通過驗證。
- 代價：`key_id` 出現在 token 裡，本身不是密鑰，但可以在日誌裡識別是哪一把 key。SHA-256 沒有 pepper；pepper 要等有金鑰管理再評估，否則遺失 pepper 會讓每一把 key 同時失效。
- 何時要重新評估：I-01.04 做撤銷時，若壓測顯示驗證查詢成為瓶頸，再考慮快取。快取的失效必須跟撤銷同一時間生效。
