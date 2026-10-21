# P02 Payment Core 與 Stripe（15 次迭代）

- **對應模組**：#04 Payment Core、#06 Connector（第一家 PSP）
- **階段目標**：一筆卡片付款能完成授權、3DS、全額或部分請款、取消，狀態由受控的狀態機決定，並保留 Stripe 的原始狀態與 reference。
- **前置**：P01。需要 Stripe 測試帳號。
- **結束時的架構**：`payment` 模組（`payment` schema：intents、attempts、captures）；`connector` 模組（`connector` schema：merchant_connector_accounts；Router、Registry、Stripe adapter）；`web/checkout` 有一個只用來完成 3DS 的最小頁面。

這個階段刻意**不處理**重送、逾時、webhook（全部在 P03）。先把「正常路徑」的狀態模型做清楚，P03 再把它放進不可靠的世界。

---

### I-02.01 PaymentIntent 模型

- **目標**：`POST /v1/payments` 建立一個 PaymentIntent（尚不呼叫 PSP），並可查詢。
- **為什麼**：Intent 代表「商戶想收這筆錢」的業務意圖，它的生命週期比任何一次對 PSP 的呼叫都長。
- **不做**：Attempt、PSP 呼叫。
- **先想清楚**：
  1. Intent 與 Order 是什麼關係？一張訂單可以有多個 Intent 嗎？
  2. Intent 需要哪些狀態？先列出來，下一次再定轉移規則。
  3. 讀 Stripe PaymentIntent 的文件：它為什麼把「建立」和「確認」分開？
- **完成條件**：測試：建立與查詢；金額與幣別使用 I-01.09 的 Money；跨商戶查詢 404。
- **常見陷阱**：把 PSP 的欄位（例如 `stripe_payment_intent_id`）直接放在 Intent 上。
- **設計模式**：Repository 與交易邊界（現在就決定交易怎麼跨 repository 傳遞，P03 的 outbox 需要和業務寫入在同一個交易）
- **Survey**：Stripe PaymentIntent lifecycle、Adyen payment lifecycle
- **產出**：`payment.payment_intents`、ADR 0006「Intent 與 Attempt 分離」

### I-02.02 PaymentAttempt 模型

- **目標**：建立 `payment.payment_attempts`，一個 Intent 可以有多個 Attempt；每個 Attempt 記錄 provider、provider reference、原始狀態。
- **為什麼**：同一筆付款可能因卡片被拒而換卡重試，或在 P12 換 PSP；每一次對外執行都要能獨立追蹤。
- **不做**：狀態轉移規則。
- **先想清楚**：
  1. 什麼情況下可以為同一個 Intent 建立新的 Attempt？什麼情況下絕對不行（例如上一個 Attempt 結果未知）？
  2. Attempt 要存哪些 PSP 資料？原始回應整包存，還是只存關鍵欄位？
  3. 同一時間最多只能有一個「進行中」的 Attempt 嗎？怎麼用約束保證？
- **完成條件**：測試：partial unique index 保證同一 Intent 最多一個進行中的 Attempt。
- **常見陷阱**：把 Attempt 的失敗直接寫成 Intent 失敗，失去重試的可能。
- **Survey**：payment attempt model、Postgres partial unique index
- **產出**：migration、Attempt repository

### I-02.03 狀態機與合法轉移表

- **目標**：Intent 與 Attempt 的狀態轉移由一張轉移表決定，非法轉移回錯誤。
- **為什麼**：webhook 亂序、重複請求、併發操作都會試圖把狀態推到錯的地方；狀態機是第一道防線。
- **不做**：持久化的併發控制（下一次）。
- **先想清楚**：
  1. 列出 Intent 的狀態：requires_payment_method、requires_confirmation、requires_action、processing、requires_capture、succeeded、canceled，還要加什麼？
  2. 哪些狀態是終態？終態之後還能有什麼事件（例如退款）？退款要改 Intent 狀態嗎？
  3. 轉移表要寫成程式碼、資料表還是設定檔？
- **完成條件**：表驅動測試涵蓋每一個「狀態 × 事件」組合，合法的轉移成功，其餘全部拒絕。
- **常見陷阱**：用一堆 if/else 判斷狀態，漏掉組合。
- **設計模式**：表驅動狀態機
- **Survey**：finite state machine Go、payment status state machine
- **產出**：`internal/payment/internal/statemachine`、`docs/payment-lifecycle.md` 的轉移表

### I-02.04 樂觀鎖與並發更新

- **目標**：Intent 有 `version` 欄位；兩個併發更新只有一個成功，另一個收到衝突錯誤。
- **為什麼**：商戶同時送「請款」和「取消」時，只能有一個生效。
- **不做**：冪等（P03）。
- **先想清楚**：
  1. 樂觀鎖（version）與悲觀鎖（`SELECT ... FOR UPDATE`）的取捨？在這裡為什麼選樂觀鎖？
  2. 衝突時回什麼給商戶？商戶應該怎麼處理？
  3. 版本號要不要露出在 API？
- **完成條件**：測試：50 個 goroutine 同時嘗試 capture 與 cancel，最終只有一個成功，狀態合法。
- **常見陷阱**：先讀狀態、在程式裡判斷、再更新，中間沒有任何保護。
- **設計模式**：樂觀鎖
- **Survey**：optimistic locking Postgres、compare-and-swap update
- **產出**：repository 的條件更新

### I-02.05 Connector 介面與正規化結果

- **目標**：定義 Connector 介面（Authorize、Capture、Void、Refund、GetStatus），以及回傳的正規化結果：Succeeded、Failed、Pending、RequiresAction、Unknown。
- **為什麼**：Payment Core 不能知道任何一家 PSP 的細節，否則 P12 加第二家時要改核心。
- **不做**：Stripe 實作（下一次）。
- **先想清楚**：
  1. 為什麼要有 `Unknown` 這個結果？它和 `Failed` 的處理方式差在哪？
  2. 不同 PSP 的能力不同（例如不支援部分請款），介面要怎麼表達「不支援」？
  3. 介面要不要接收冪等 key？
- **完成條件**：介面與型別定義完成；一個 in-memory fake connector 讓 Payment Core 的測試能跑。
- **常見陷阱**：介面回傳 PSP 的原始錯誤，讓核心開始解析字串。
- **設計模式**：Ports & Adapters、Anti-corruption layer、Fake 與 Mock 的區分（見 [`03-design-patterns.md`](../03-design-patterns.md) 第 2 節）
- **Survey**：adapter pattern、anti-corruption layer、Hyperswitch ConnectorIntegration
- **產出**：`internal/connector/api.go`、ADR 0007「Connector 結果的五種狀態」

### I-02.05a 多 PSP 的介面分層與能力介面

- **目標**：把 Connector 分成「每家都必須支援的核心操作」與「選擇性能力」（部分請款、多次請款、增量授權、payout 等），並拿一家非 Stripe 的 PSP 文件做一次紙上映射，找出介面裡藏著的 Stripe 假設。
- **為什麼**：只看 Stripe 設計出來的介面，到 P12 才會發現處處是 Stripe 的形狀。在寫任何 adapter 之前，花一次迭代對照第二家的文件，比之後改核心便宜很多。
- **不做**：第二家的程式實作（P12）、路由（下一次與 P12）。
- **先想清楚**：
  1. Go 裡表達「選擇性能力」有幾種做法：大介面加 `ErrNotSupported`、小介面加型別斷言、能力宣告加事前檢查？各自對呼叫端和測試的影響？
  2. 對照 Adyen 或 PayPal：建立付款是一步還是兩步？3DS 與 redirect 怎麼表達？有幾種 reference？哪些概念在 Stripe 裡沒有？
  3. 「下一步動作」（3DS、redirect、顯示 QR code）要用什麼通用型別表達，Payment Core 才不用知道是哪一家？
  4. 每個 adapter 內部要不要固定分成：組 request、送出、解析 response、映射狀態與錯誤、驗 webhook？
- **完成條件**：`docs/connector-design.md` 有一張對照表（我們的操作 × Stripe × 第二家），列出不一致之處與處理方式；介面依結論調整後，fake connector 的測試仍然通過。
- **常見陷阱**：為了「通用」把介面做成 `Do(map[string]any)`，等於沒有介面；或反過來，把只有 Stripe 有的參數放進核心介面。
- **設計模式**：Capability interface（介面分離）、Adapter（見 [`03-design-patterns.md`](../03-design-patterns.md) 第 2 節）
- **Survey**：Go interface segregation、optional interfaces type assertion、Adyen Checkout API、PayPal Orders API、Hyperswitch ConnectorIntegration
- **產出**：`docs/connector-design.md`、更新 ADR 0007

### I-02.05b 渠道註冊表、商戶渠道帳號與固定路由

- **目標**：建立 provider registry（provider 名稱 → Connector 實作）與 `connector.merchant_connector_accounts`（商戶啟用哪家 PSP、test 或 live、憑證的 secret 參照）；Payment Core 透過 Router 介面取得要用的渠道帳號，目前的 Router 實作只回傳商戶唯一啟用的那一個。
- **為什麼**：真實平台上，同一家 PSP 會有多組憑證（不同商戶、test 與 live）。而且「選哪一家」（路由）和「怎麼呼叫那一家」（adapter）是兩件事。先把這兩條縫留好，P12 只要替換 Router 的實作。
- **不做**：多家之間的選擇規則、健康度、failover（P12）。
- **先想清楚**：
  1. 平台用自己的 PSP 帳號替所有商戶收款，和每個商戶連自己的 PSP 帳號，在資金流、對帳、憑證保管上差在哪？本專案選哪一種，寫進筆記。
  2. PSP 憑證存在哪裡？資料表只存 secret 的參照時，啟動與輪替怎麼運作？
  3. Attempt 要記錄用了哪一個渠道帳號。為什麼只記 provider 名稱不夠（想想退款與對帳）？
  4. Registry 在 `cmd/` 啟動時組裝（composition root），還是用 `init()` 自動註冊？哪一種比較好測試？
- **完成條件**：測試：商戶 A 用 fake connector 甲、商戶 B 用 fake connector 乙，兩者都走完授權流程；Payment Core 程式碼裡沒有任何 provider 名稱的判斷；Attempt 記錄渠道帳號 ID。
- **常見陷阱**：在 Payment Core 寫 `switch provider`；把 PSP secret key 以明文存進資料表。
- **設計模式**：Registry、Strategy（Router）、Composition root（見 [`03-design-patterns.md`](../03-design-patterns.md) 第 2 節）
- **Survey**：Hyperswitch merchant_connector_account、Stripe Connect vs direct integration、strategy pattern Go
- **產出**：`connector.merchant_connector_accounts`、Router 介面與固定路由實作

### I-02.06 Stripe adapter：建立 PaymentIntent

- **目標**：Stripe adapter 以 `capture_method=manual` 建立 Stripe PaymentIntent，保存 Stripe ID 與原始狀態。
- **為什麼**：manual capture 讓你能分開觀察授權與請款，這是 W2 的核心。
- **不做**：確認付款（下一次）。
- **先想清楚**：
  1. 我們的 Intent 與 Stripe 的 PaymentIntent 是一對一嗎？還是我們的 Attempt 對應 Stripe 的 PaymentIntent？
  2. Stripe 的 metadata 要放什麼才能在對帳時反查（例如我們的 attempt id）？
  3. API key 與 Stripe 帳號設定怎麼注入？
- **完成條件**：sandbox 整合測試（手動或夜間執行）建立成功，資料庫有 Stripe ID；單元測試用錄好的回應。
- **常見陷阱**：把我們的 Intent ID 當成 Stripe 的冪等 key 卻在重試新 Attempt 時重用。
- **Survey**：Stripe manual capture、stripe-go、Stripe metadata
- **產出**：`internal/connector/stripe`

### I-02.07 授權：用 Stripe 測試支付方式確認付款

- **目標**：`POST /v1/payments/{id}/confirm` 帶入 PSP 的支付方式 token（測試時使用 Stripe 的測試 PaymentMethod，例如 `pm_card_visa`），完成授權，Intent 進入 requires_capture。
- **為什麼**：伺服器只處理 token，不碰卡號；這是 PCI 範圍控制的起點。
- **不做**：3DS（下一次）、前端收卡（P11）。
- **先想清楚**：
  1. confirm 時才建立 Attempt，還是 create 時就建立？
  2. 授權被拒時，Attempt 和 Intent 分別變成什麼狀態？商戶可以換卡重試嗎？
  3. 回應給商戶的內容要包含哪些 PSP 資訊？
- **完成條件**：sandbox 測試：成功卡授權成功；被拒卡（`pm_card_chargeDeclined`）Attempt 失敗、Intent 回到可重試狀態。
- **常見陷阱**：把拒絕當成系統錯誤回 500。
- **Survey**：Stripe test cards、Stripe confirm PaymentIntent、decline codes
- **產出**：confirm API、Attempt 建立流程

### I-02.08 3DS：requires_action 與最小結帳頁

- **目標**：3DS 測試卡讓 Intent 進入 requires_action；用一個最小 Next.js 頁面以 Stripe.js 完成驗證，之後 Intent 進入 requires_capture。
- **為什麼**：3DS 讓付款流程變成「伺服器→持卡人→伺服器」的非同步流程，這是很多狀態錯誤的來源。
- **不做**：完整結帳頁、樣式、Payment Link（P11）。
- **先想清楚**：
  1. requires_action 時要回給商戶什麼（client secret、next action）？client secret 的權限有多大？
  2. 持卡人完成 3DS 後，你的系統怎麼知道結果：前端回報、主動查詢、還是 webhook？哪一個可以信任？
  3. 持卡人關掉頁面、不完成驗證，Intent 會停在哪裡？
- **完成條件**：用 3DS 測試卡在頁面上完成驗證；伺服器端用查詢確認結果，不以前端回報為準。
- **常見陷阱**：前端一回報成功就把付款標為成功。
- **Survey**：3D Secure 2、SCA、Stripe handleNextAction、frictionless vs challenge
- **產出**：`web/checkout` 最小頁面、`docs/payment-lifecycle.md` 補 3DS 流程

### I-02.09 全額請款

- **目標**：`POST /v1/payments/{id}/capture` 對已授權的付款請款，成功後 Intent 進入 succeeded，並建立 Capture 紀錄。
- **為什麼**：授權只是保留額度，請款才是真正要求付款；兩者在帳務上是不同事件。
- **不做**：部分請款（下一次）。
- **先想清楚**：
  1. Capture 要不要是獨立的實體？為什麼不只改 Intent 狀態？
  2. 沒有授權就請款、重複請款，分別怎麼被擋下？
  3. 請款失敗時 Intent 回到哪個狀態？
- **完成條件**：sandbox 測試成功；未授權請款被狀態機拒絕；`payment.captures` 有 Stripe charge 或 capture reference。
- **常見陷阱**：把 succeeded 等同於「錢已到帳」。
- **Survey**：authorization vs capture vs settlement、Stripe capture PaymentIntent
- **產出**：capture API、`payment.captures`

### I-02.10 部分請款

- **目標**：請款金額小於授權金額；剩餘額度依 PSP 規則自動釋放或保留。
- **為什麼**：電商常在出貨時才知道實際金額（缺貨、重量計價）。
- **不做**：多次請款（若 PSP 不支援就在筆記中說明）。
- **先想清楚**：
  1. Stripe 的部分請款之後，剩下的授權額度會怎樣？
  2. 請款金額大於授權金額怎麼處理？
  3. `amount_capturable` 與 `amount_received` 在帳務上的意義？
- **完成條件**：測試：授權 100、請款 60，Capture 記錄 60；請款 120 被拒絕。
- **常見陷阱**：用我們自己的欄位計算可請款金額，與 PSP 不一致。
- **Survey**：Stripe partial capture、multicapture、incremental authorization
- **產出**：capture 金額驗證

### I-02.11 取消授權與授權過期

- **目標**：`POST /v1/payments/{id}/void` 取消尚未請款的授權；過期未請款的授權由排程標記為過期。
- **為什麼**：授權會佔用持卡人額度；不請款就應該取消，過期後就不能再請款。
- **不做**：退款（P04）。
- **先想清楚**：
  1. Void 與 Refund 的差別是什麼？對持卡人帳單的影響呢？
  2. 卡片授權大約多久過期？你要信任 PSP 告訴你的時間，還是自己算？
  3. 排程怎麼做：worker 定期掃描？要注意什麼才不會重複處理？
- **完成條件**：測試：取消後 Intent 為 canceled；過期的授權無法請款；排程重跑不會重複處理。
- **常見陷阱**：已請款之後還允許 Void。
- **設計模式**：單一執行者排程、Clock 注入
- **Survey**：void vs refund、authorization expiry card networks、Stripe cancel PaymentIntent
- **產出**：void API、過期掃描 job

### I-02.12 狀態映射與拒絕原因正規化

- **目標**：把 Stripe 的狀態與拒絕碼映射成我們的正規化狀態與錯誤分類，同時保留原始值。
- **為什麼**：商戶需要穩定的錯誤分類；你除錯時需要原始資訊；P12 的第二家 PSP 也會映射到同一套。
- **不做**：重試策略（P12）。
- **先想清楚**：
  1. 拒絕原因要分成哪幾類（資金不足、卡片問題、疑似詐騙、需驗證、系統錯誤）？
  2. 哪些拒絕原因可以告訴商戶，哪些不應該（例如 fraud 相關）？
  3. 遇到未知的 PSP 狀態時要怎麼辦？
- **完成條件**：映射表有單元測試；未知狀態被歸為 Unknown 並產生告警日誌，不會被當作成功或失敗。
- **常見陷阱**：遇到沒見過的狀態時預設成 failed。
- **設計模式**：Anti-corruption layer（映射表放在 adapter 內，核心只看到正規化結果）
- **Survey**：Stripe decline codes、soft decline vs hard decline、Hyperswitch GSM
- **產出**：映射表、`docs/connector-mapping.md`

### I-02.13 Adapter 契約測試與 conformance suite（含階段檢核）

- **目標**：用錄下來的 Stripe 回應做 adapter 的契約測試，CI 不需要連 Stripe；並把 P02 的生命週期測試整理成一套所有 Connector 實作共用的 conformance suite（現在跑 fake 與 Stripe，P03 加 Mock PSP，P12 加第二家）。
- **為什麼**：外部 API 測試慢、會受網路影響、有配額；但 adapter 的解析錯誤會直接變成資金錯誤。共用的 conformance suite 則是「介面真的通用」的證據。
- **不做**：Mock PSP 的故障注入（P03）。
- **先想清楚**：
  1. 錄製回應時，如何去除 API key、client secret 等敏感值？
  2. 什麼時候要重新錄製？怎麼知道 PSP 的回應格式變了？
  3. 契約測試與 sandbox 測試各自證明什麼？
- **完成條件**：`make test` 在無網路下跑過所有 Stripe adapter 測試；fixtures 不含任何 secret；同一套 conformance suite 在 fake 與 Stripe（錄製回應）上都通過，不支援的能力被明確標記。
- **常見陷阱**：把真實 secret 錄進 fixture 後 commit。
- **設計模式**：錄製回應的契約測試、Conformance suite
- **Survey**：go-vcr、httptest server、contract testing
- **產出**：`internal/connector/stripe/testdata`、`docs/notes/p02-review.md`

## 階段檢核

- **面試題**：
  1. Authorization、Capture、Settlement 的差別？
  2. 為什麼 PaymentIntent 與 PaymentAttempt 要分開？
  3. Void 和 Refund 差在哪？
  4. 3DS 完成後，你怎麼確認結果？為什麼不信任前端？
  5. 遇到沒見過的 PSP 狀態時，你的系統怎麼做？
- **故障場景**：併發 capture 與 void；未知 PSP 狀態。
- **tag**：`p02-payment-core-done`
