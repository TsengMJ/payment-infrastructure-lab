# 0004 test / live 資料隔離與 API key 輪替

- 狀態：Accepted
- 日期：2026-10-22
- 相關迭代：I-01.04

## 背景

商戶要能用 test key 試接，而不碰到 live 資料；key 外洩時要能在不停機的情況下換掉。ADR 0003 曾要求行程的 `PAYMENT_MODE` 與 token 前綴一致，那會讓同一支 API 無法同時接受兩種 key，跨 mode 讀取也到不了 404。

這次還沒有真實資金操作，也沒有 provider。

## 選項

1. **行程只跑一種 mode。** test 部署拒絕 live key。隔離強，但跨 mode 的結果是 401，而且輪替與讀取都要兩套行程。
2. **客戶端自帶 mode。** 實作短。呼叫端可以宣稱自己在 test，實際讀到 live。
3. **同一資料庫，mode 欄位，mode 只來自驗證成功的 API key。** 商戶主檔共用。交易查詢必須帶 Merchant ID 與 Mode。少一列、跨商戶、跨 mode 都是 not found。

## 決定

採用選項 3。這取代 ADR 0003 裡「行程模式必須與 token 一致」那一句。key 的格式與雜湊不變。

- 請求的 mode 只來自驗證成功的 key。不讀 query、header 或 body。
- `PAYMENT_MODE` 仍是行程的資金執行姿勢。它不參與驗證，也不過濾查詢。test 姿勢的行程以後不得呼叫 live provider。本階段沒有 provider，也不做任何資金操作。
- 之後的 live 資金移動必須使用獨立的 provider credentials，並與 test 執行隔開。
- 交易資料用同一資料庫加 `mode`。`internal/platform/scope.Scope` 是查詢的必填參數，零值不能用。`GET /v1/resources/{id}` 是這條規則的第一個讀取；它不是訂單。
- 同一 mode 可以同時有多把未撤銷 key，不設上限。新 key 在建立交易提交後立刻可驗證。輪替是：建立新 key、呼叫端改用新 key、再撤銷舊 key。
- 撤銷寫 `revoked_at`，不可逆。key 的 mode 建立後不可改。這兩件事由資料庫 trigger 執行。Atlas Community 不能在 HCL 宣告 function 與 trigger，所以 SQL 寫在 migration 裡，並在 `atlas.hcl` 排除這兩個物件，避免下次 diff 把它們丟掉。
- 撤銷、未知 key、雜湊不符、前綴與資料列 mode 不符，都是 401 `invalid_api_key`。不說明是哪一種。
- 撤銷交易提交後，下一筆開始的驗證失敗。已經進入 handler 的請求不取消。驗證每次讀資料庫，不加 cache。
- 建立 key、撤銷 key、變更 live access 只接受管理端憑證 `MANAGEMENT_TOKEN`。這是 P15 操作者登入之前的單一管理身份，actor 記成 `management`。商戶 API key 不能做這些事。
- Live key 只發給 `live_access = true` 的 active 商戶。關掉資格只擋新的 live key，不撤銷、也不拒絕既有 live key。
- 上述寫入與 `merchant.admin_events` 在同一個資料庫交易。稽核表拒絕 UPDATE 與 DELETE。稽核不放 secret、token 或雜湊。

## 後果

- 好處：test 流量讀不到 live 列。換 key 時兩把可以並存，撤銷後下一筆請求就失效。資料庫外洩仍然沒有可用的 secret。
- 代價：同一支 API 行程同時認 test 與 live key，所以模式隔離靠查詢條件，不靠行程邊界。管理端目前是一把共享 token，不是個別操作者。`payment.resources` 只用來鎖住隔離規則，I-01.10 的訂單要沿用同一個 Scope。
- 何時要重新評估：接上 live provider 時，test 姿勢的行程必須拒絕載入 live credentials。若驗證的主鍵讀取成為瓶頸，快取的失效必須與撤銷同一時間生效。P15 要用個別操作者取代共享管理 token，並把 actor 改成那個人。
