# P11 Checkout 與支付方式（11 次迭代）

- **對應模組**：#02 Checkout & Payment SDK、#07 Payment Method & Tokenization
- **階段目標**：完整的 hosted checkout 與 Payment Link；卡資料只經過 PSP 的託管欄位；支援電子錢包與一種非同步的銀行轉帳支付方式，並理解它們在帳務與對帳上的差異。
- **前置**：P02（最小 3DS 頁面）、P10（風控決策）。
- **結束時的架構**：`checkout` 模組（`checkout` schema：sessions、payment_links、method_configs）；`web/checkout` 成為完整的 Next.js 應用；payment 支援非卡的生命週期。

**Repo 決策點**：如果你想把結帳做成商戶可安裝的 npm 套件（像 Stripe.js），在這個階段結束時評估是否拆出 `payment-checkout-sdk` repo（見 `02-architecture-evolution.md` 第 4 節）。只做 hosted page 就不拆。

---

### I-11.01 Checkout session

- **目標**：`POST /v1/checkout/sessions` 建立結帳 session（綁定訂單、金額、成功與取消網址、到期時間），回傳結帳頁網址。
- **為什麼**：session 把「商戶後端決定的金額」安全地帶到前端頁面，前端不能改。
- **不做**：前端頁面（下一次）。
- **先想清楚**：
  1. session 與 PaymentIntent 的關係：建立 session 時就建 Intent，還是付款時才建？
  2. session 到期後，已經授權的付款怎麼辦？
  3. session ID 出現在網址上，能被猜測或重用嗎？
- **完成條件**：測試：過期 session 無法付款；同一 session 只能完成一筆付款。
- **常見陷阱**：結帳頁從 query string 讀取金額。
- **Survey**：Stripe Checkout Sessions、hosted payment page
- **產出**：session API

### I-11.02 Hosted checkout 頁面

- **目標**：Next.js 結帳頁使用 Stripe Payment Element 收集卡資料，完成授權與 3DS。
- **為什麼**：卡號只在 PSP 的 iframe 裡，平台的頁面與後端都碰不到，這是降低 PCI 範圍的主要手段。
- **不做**：樣式客製化。
- **先想清楚**：
  1. iframe 欄位與完全由 PSP 託管的頁面，PCI 自評問卷（SAQ A vs SAQ A-EP）的差別？
  2. 頁面需要哪些安全 header（CSP、frame-ancestors）？
- **完成條件**：E2E 測試：用測試卡在頁面完成付款；瀏覽器的 network log 中，卡號只送往 Stripe 網域。
- **常見陷阱**：自己做輸入欄位再把卡號送到 Stripe。
- **Survey**：Stripe Payment Element、PCI SAQ A、Content Security Policy
- **產出**：`web/checkout`

### I-11.03 回跳與結果頁

- **目標**：付款完成後回跳商戶網址；結果頁以伺服器查詢的狀態為準，處理重新整理、返回鍵、重複點擊。
- **為什麼**：「前端說成功」不等於成功；ProjectRoadmap W1 要求避免只靠前端 redirect 判斷。
- **不做**：商戶端的訂單履行。
- **先想清楚**：
  1. 持卡人關掉頁面，付款仍在 processing 時，商戶怎麼得知結果？
  2. 重複點擊付款按鈕會發生什麼？I-03 的冪等能擋住嗎？
- **完成條件**：E2E 測試：重新整理結果頁不會重複付款；processing 中的付款最終透過 webhook 讓商戶得知。
- **常見陷阱**：用 redirect 網址上的參數判斷付款成功。
- **Survey**：Stripe return_url、payment confirmation polling
- **產出**：結果頁

### I-11.04 Payment Link

- **目標**：商戶可以建立 Payment Link（一次性或可重複使用），付款者打開連結即進入結帳。
- **為什麼**：不寫程式的商戶也能收款；但可重複使用的連結會帶來不同的冪等與庫存問題。
- **不做**：自訂表單欄位。
- **先想清楚**：
  1. 可重複使用的連結，每次打開都建立新 session 嗎？
  2. 連結要不要有到期與使用次數上限？停用後已打開的頁面怎麼辦？
- **完成條件**：測試：一次性連結付款後失效；可重複連結每次產生新的付款。
- **常見陷阱**：可重複連結共用同一個 Intent。
- **Survey**：Stripe Payment Links、Hyperswitch payment_link
- **產出**：payment_links

### I-11.05 支付方式設定

- **目標**：商戶可設定啟用的支付方式；結帳頁依幣別、國家、金額篩選出可用的方式。
- **為什麼**：不同支付方式支援的幣別、國家、金額範圍不同；顯示不能用的方式只會造成失敗。
- **不做**：路由（P12）。
- **先想清楚**：
  1. 可用性規則由誰決定：平台、商戶、還是 PSP 能力？
  2. 規則要放在後端還是前端？
- **完成條件**：測試：JPY 訂單不會顯示不支援 JPY 的方式。
- **常見陷阱**：前端寫死支付方式清單。
- **Survey**：payment method availability、Stripe automatic payment methods
- **產出**：method_configs

### I-11.06 Token reference

- **目標**：平台只保存 PSP 的 payment method ID、卡片品牌、末四碼、到期月年、指紋。
- **為什麼**：P13 的訂閱與已存卡需要能重複使用支付方式，但平台不應持有卡號。
- **不做**：已存卡的扣款（P13）。
- **先想清楚**：
  1. PSP token 是否能跨 PSP 使用？這對 P12 的多渠道路由有什麼限制？
  2. Network token 是什麼？和 PSP token 有什麼不同？
- **完成條件**：資料表與 API 回應只包含允許的欄位。
- **常見陷阱**：以為有了 token 就能在任何 PSP 扣款。
- **Survey**：PSP token vs network token、vault portability
- **產出**：payment_method_refs 表、ADR 0022「Token 可攜性與 PSP 綁定」

### I-11.07 PCI 邊界測試

- **目標**：自動化測試掃描日誌、資料庫、trace、錯誤回應，確保不出現卡號樣式（Luhn 檢查）或 CVV。
- **為什麼**：PCI 範圍控制要有證據，而不是只靠「我們沒有收卡號」的說法。
- **不做**：正式 PCI 稽核。
- **先想清楚**：
  1. 卡號可能從哪些地方意外進來（商戶誤傳、錯誤訊息、metadata）？
  2. 偵測到時要怎麼處理：拒絕請求、遮罩、還是告警？
- **完成條件**：測試：在 metadata 送入測試卡號時，API 拒絕並告警，任何儲存位置都找不到。
- **常見陷阱**：只檢查欄位名稱，不檢查內容。
- **Survey**：PAN detection Luhn、PCI DSS scope reduction
- **產出**：掃描工具、`tests/security/pan_leak_test.go`

### I-11.08 電子錢包

- **目標**：透過 Stripe 支援 Apple Pay 與 Google Pay（測試模式）；完成網域驗證。
- **為什麼**：錢包付款的資料流不同（裝置 token、密碼學資料），但對平台而言仍應只是 PSP 的 payment method。
- **不做**：自己解密錢包 token。
- **先想清楚**：
  1. 錢包付款還需要 3DS 嗎？
  2. 網域驗證需要什麼？本機怎麼測？
- **完成條件**：至少一種錢包在測試模式完成付款（受裝置限制時，記錄限制並以 Stripe 測試工具驗證）。
- **常見陷阱**：以為錢包付款不會被拒或不會被爭議。
- **Survey**：Apple Pay on the web、Google Pay、Stripe wallets
- **產出**：錢包支援、測試紀錄

### I-11.09 銀行轉帳與虛擬帳號

- **目標**：支援一種非同步付款方式（例如 Stripe 的銀行轉帳或 Mock 虛擬帳號）：付款者收到轉帳資訊，付款要幾天後才確認，並有到期時間。
- **為什麼**：非卡支付的生命週期完全不同：沒有授權與請款，只有「等待入帳」。
- **不做**：多國銀行轉帳。
- **先想清楚**：
  1. Intent 的狀態機要怎麼擴充？還是用不同的狀態機？
  2. 到期前沒入帳怎麼處理？到期後才入帳呢？
  3. 商戶什麼時候可以出貨？
- **完成條件**：Mock 測試：建立、等待、入帳成功、到期取消四條路徑。
- **常見陷阱**：把卡片的狀態機硬套到銀行轉帳。
- **Survey**：bank transfer payment method、virtual account、Stripe customer_balance
- **產出**：非同步付款流程、狀態機擴充

### I-11.10 多付、少付、逾期入帳

- **目標**：銀行轉帳入帳金額與應付不符，或在到期後才入帳時，依規則處理（部分付款、退回、轉為餘額）。
- **為什麼**：付款者會打錯金額、分兩次付、晚付；這些在卡片付款裡不會發生。
- **不做**：自動退回到付款者帳戶（用 Mock 表示）。
- **先想清楚**：
  1. 多付的錢屬於誰？要記在哪個科目？
  2. 少付時訂單算完成嗎？
- **完成條件**：測試：多付、少付、逾期三種情況的狀態與分錄都正確且平衡。
- **常見陷阱**：多付的錢沒有入帳，直接消失在對帳差異裡。
- **Survey**：overpayment underpayment bank transfer、unapplied cash
- **產出**：處理規則、ADR 0023「轉帳金額不符的處理」

### I-11.11 非卡方式在帳務與對帳上的差異（含階段檢核）

- **目標**：補上非卡付款的過帳規則與對帳規則（沒有授權、結算時間不同、手續費結構不同）。
- **為什麼**：每多一種支付方式，ledger 與對帳就多一套規則；這次讓你體會「支付方式多樣性」的成本。
- **不做**：更多支付方式。
- **先想清楚**：
  1. 非卡付款的退款怎麼做？可以退回原帳戶嗎？
  2. 對帳時用什麼 reference 對上？
- **完成條件**：P06 的對帳測試加入銀行轉帳資料後仍全部通過。
- **常見陷阱**：對帳規則只考慮卡片的 reference 格式。
- **Survey**：alternative payment methods reconciliation
- **產出**：規則擴充、`docs/notes/p11-review.md`

## 階段檢核

- **面試題**：
  1. 你的系統看得到卡號嗎？你怎麼證明？
  2. 為什麼不能用前端的 redirect 判斷付款成功？
  3. 卡片付款和銀行轉帳的生命週期差在哪？
  4. 付款者轉帳多付了錢，你怎麼處理？
  5. PSP token 可以拿到另一家 PSP 用嗎？
- **故障場景**：重複點擊付款；卡號出現在 metadata；轉帳逾期入帳。
- **tag**：`p11-checkout-done`
