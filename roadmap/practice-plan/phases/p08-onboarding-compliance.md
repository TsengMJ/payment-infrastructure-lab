# P08 商戶入駐與合規（10 次迭代）

- **對應模組**：#01 Merchant Management 的 KYB/KYC 部分、#18 的合規部分
- **階段目標**：商戶從申請到可以收款、可以出款，經過可稽核的審核流程；身分驗證、制裁篩查、銀行帳戶驗證都透過第三方完成，平台只做流程、狀態、決策與資料保存邊界。
- **前置**：P03（inbox）、P07（`payouts_enabled`）。
- **結束時的架構**：`onboarding` 模組（`onboarding` schema：applications、persons、verifications、screenings、reviews）；三種 vendor adapter（KYC/KYB、制裁篩查、銀行帳戶驗證），各有 Mock 與 sandbox 實作。

**範圍提醒**：本專案只練技術流程，不宣稱符合任何法域的 KYC/AML 規範。不要用真人身分或真實證件測試，一律使用 vendor 提供的測試資料。

本階段的大方向：

- KYC（驗證個人）與 KYB（驗證公司與其受益人）是不同的流程。
- Vendor 的結果是建議，最終決策由平台的規則與人工審核做出，並留下理由。
- 個資最小化：能只存 reference 就不存原始資料；需要保存的有保存期限。
- 失敗與補件是常態，使用者體驗與狀態設計同樣重要。

---

### I-08.01 入駐狀態機

- **目標**：定義入駐申請的狀態：draft、submitted、in_review、needs_info、approved、rejected，以及合法轉移。
- **為什麼**：入駐是長時間、多參與者（商戶、vendor、審核人員）的流程；沒有清楚的狀態機，很快就會出現「到底誰在等誰」。
- **不做**：vendor 整合。
- **先想清楚**：
  1. approved 之後還能回到 in_review 嗎（例如資料變更、定期複查）？
  2. 部分通過（可以收款但不能出款）要用狀態表示，還是用能力（capabilities）表示？
  3. 誰可以觸發哪些轉移（商戶、系統、審核人員）？
- **完成條件**：表驅動測試涵蓋所有轉移與觸發者權限。
- **常見陷阱**：用一個 `is_verified` 布林值取代整個流程。
- **Survey**：Stripe Connect account requirements、capabilities、onboarding state machine
- **產出**：migration、ADR 0017「入駐狀態與能力分離」

### I-08.02 入駐資料模型與個資邊界

- **目標**：建立公司資料、負責人、最終受益人（UBO）的資料模型；逐欄決定「平台保存」或「只存 vendor reference」。
- **為什麼**：KYC 資料是最敏感的個資之一，外洩的後果比交易資料更嚴重。
- **不做**：加密實作的細節（P16 的 KMS）；這裡先用欄位層級加密的簡單版本。
- **先想清楚**：
  1. 哪些欄位平台一定要有（例如公司名稱、國家），哪些只需要 vendor 的結果？
  2. 證件影像要存嗎？（答案應該是否，由 vendor 保存）
  3. UBO 的持股門檻（例如 25%）是誰定的？在模型中怎麼表示？
- **完成條件**：`docs/onboarding-data.md` 有逐欄的保存決策表；資料庫中不存在證件號碼明文。
- **常見陷阱**：把 vendor 回傳的整包 JSON 存進資料庫，裡面包含完整個資。
- **Survey**：UBO beneficial ownership、data minimization GDPR、field level encryption
- **產出**：migration、資料保存決策表

### I-08.03 KYC/KYB adapter 與 Mock vendor

- **目標**：定義驗證 adapter 介面（建立驗證、查詢結果），回傳 Approved、Rejected、NeedsInfo、Pending、Unknown；Mock vendor 可控回這五種結果與逾時。
- **為什麼**：本機與 CI 不能依賴外部 vendor；Mock 也讓你能測試罕見的結果。
- **不做**：真實 vendor。
- **先想清楚**：
  1. 驗證通常是非同步的，介面要怎麼表達「已送出、等待結果」？
  2. vendor 的拒絕原因要怎麼正規化？哪些可以讓商戶知道？
  3. 這和 P02 的 Connector 介面有什麼相似之處？能共用嗎？
- **完成條件**：Mock vendor 的五種結果與逾時都有測試。
- **常見陷阱**：vendor 逾時時預設放行。
- **設計模式**：Ports & Adapters、Fail closed（和 Connector 的分層是同一個形狀，可以對照 [`03-design-patterns.md`](../03-design-patterns.md) 第 2 節）
- **Survey**：KYC API design、identity verification webhook
- **產出**：`internal/onboarding/vendor` 介面與 Mock

### I-08.04 接一家 sandbox vendor

- **目標**：用一家提供免費 sandbox 的 vendor 建立驗證 session，保存 vendor reference，讓測試使用者完成流程。
- **為什麼**：真實 vendor 的流程（託管頁面、重新導向、文件上傳）會讓你看到 Mock 看不到的細節。
- **不做**：同時接多家。
- **先想清楚**：
  1. 選哪家？比較 Stripe Identity、Persona、Sumsub、Onfido 的 sandbox 申請門檻與地區限制。
  2. 驗證頁面是 vendor 託管，還是嵌入 SDK？對個資範圍的影響？
  3. vendor 的 API key 與 webhook secret 怎麼管理？
- **完成條件**：sandbox 手動測試完成一次驗證；資料庫只有 vendor reference 與結果。
- **常見陷阱**：讓個資經過自己的後端轉送給 vendor。
- **Survey**：Stripe Identity verification sessions、Persona inquiries、Sumsub applicants
- **產出**：sandbox adapter、ADR 0018「KYC vendor 選擇」

### I-08.05 Vendor webhook 驅動狀態

- **目標**：接收 vendor 的 webhook，驗簽、寫入 inbox、去重，驅動入駐狀態。
- **為什麼**：驗證結果幾乎都是非同步回來的；這裡直接重用 P03 的元件，也驗證那些元件是否真的通用。
- **不做**：人工審核。
- **先想清楚**：
  1. webhook 只帶 reference 時，要不要回頭向 vendor 查詢完整結果？
  2. vendor 結果與目前狀態衝突時（例如已被人工拒絕），以誰為準？
- **完成條件**：測試：重複、亂序的 vendor 事件不會讓狀態倒退；簽章錯誤被拒絕。
- **常見陷阱**：為了 vendor 重寫一套 webhook 處理，而不是重用 inbox。
- **Survey**：webhook verification、inbox reuse
- **產出**：vendor webhook handler

### I-08.06 制裁與 PEP 篩查

- **目標**：入駐時對公司與相關人員做制裁名單與 PEP 篩查；疑似命中轉人工；出款前對受款人再篩一次。
- **為什麼**：與制裁對象交易可能違法；名單比對會有大量誤判（同名同姓），所以一定要有人工判斷。
- **不做**：持續監控（I-08.10）。
- **先想清楚**：
  1. 模糊比對的門檻怎麼設？門檻太低與太高各自的代價？
  2. 篩查結果要保存什麼才能在稽核時說明「為什麼放行」？
  3. 出款前篩查失敗時，payout 停在哪個狀態？hold 怎麼辦？
- **完成條件**：測試：Mock 命中時申請進入 in_review；出款前命中時 payout 停在 pending_approval 且 hold 保留。
- **常見陷阱**：命中就自動拒絕，沒有人工判斷與紀錄。
- **Survey**：OpenSanctions API、ComplyAdvantage、PEP screening、false positive
- **產出**：screening adapter、payout 流程中的篩查步驟

### I-08.07 銀行帳戶驗證

- **目標**：商戶的出款帳戶需經驗證（第三方帳戶驗證或 Mock 微額存款），結果寫入 P07 的 bank_accounts.verified。
- **為什麼**：未驗證的帳戶可能填錯或屬於他人；出款到錯的帳戶很難追回。
- **不做**：多國銀行格式。
- **先想清楚**：
  1. 微額存款驗證與即時帳戶驗證（例如 Plaid）的差別、時間、成本？
  2. 帳戶持有人姓名要不要和 KYB 的公司名稱比對？
  3. 驗證失敗幾次要鎖定？
- **完成條件**：測試：未驗證帳戶無法用於出款；Mock 微額驗證流程成功與失敗都能走完。
- **常見陷阱**：只驗證帳號格式，沒驗證帳戶真的存在且屬於商戶。
- **Survey**：micro-deposit verification、Plaid Auth、Stripe Financial Connections、account name matching
- **產出**：verification 流程

### I-08.08 能力閘門

- **目標**：未審核通過的商戶可以建立帳號與使用 test 模式，但不能用 live 模式收款，也不能出款；把 P07 的 `payouts_enabled` 改由入駐結果與銀行帳戶驗證決定。
- **為什麼**：把合規結果真正接到資金流程上；否則審核只是表單。
- **不做**：細緻的能力分級。
- **先想清楚**：
  1. 能力檢查放在哪裡：API middleware、各模組，還是兩者？
  2. 已經在收款的商戶被暫停時，進行中的付款與出款怎麼處理？
  3. 從手動旗標遷移到由規則決定時，舊資料怎麼處理？
- **完成條件**：測試：未通過商戶的 live 付款與出款被拒；通過後可以使用；被暫停後出款立即停止。
- **常見陷阱**：只在前端隱藏功能，API 仍可呼叫。
- **Survey**：capability gating、account restrictions Stripe Connect
- **產出**：capability 檢查、移除手動 `payouts_enabled` 的 migration

### I-08.09 人工審核紀錄

- **目標**：審核人員的每個決定（核准、駁回、要求補件）都記錄審核人、時間、理由與當時看到的證據快照。
- **為什麼**：稽核時要能回答「為什麼這個商戶被核准」；之後 P15 的審核頁會使用這些資料。
- **不做**：UI（P15）。
- **先想清楚**：
  1. 審核紀錄要不要不可修改？
  2. 「當時看到的證據」要存多少才夠，又不違反個資最小化？
  3. 審核人員可以審核自己建立的商戶嗎？
- **完成條件**：測試：每個人工轉移都產生審核紀錄；紀錄不能被修改或刪除。
- **常見陷阱**：只記錄最後狀態，不記錄決策過程。
- **Survey**：audit log design、append-only
- **產出**：reviews 表與 API

### I-08.10 持續監控與資料保存期限（含階段檢核）

- **目標**：定期重新篩查已核准的商戶；重要資料變更（負責人、出款帳戶）觸發重新審核；個資依保存期限刪除或匿名化。
- **為什麼**：入駐不是一次性的；制裁名單會更新，商戶資料會變；個資也不能永久保存。
- **不做**：交易監控（P10 的風控）。
- **先想清楚**：
  1. 重新篩查的頻率？名單更新時要不要立即全量重跑？
  2. 哪些資料變更要觸發重新審核？審核期間商戶能不能繼續收款？
  3. 帳務紀錄需要長期保存，KYC 個資卻要刪除，兩者怎麼切開？
- **完成條件**：測試：排程重新篩查不重複；變更負責人觸發 in_review；過期個資被匿名化，但帳務紀錄完整。
- **常見陷阱**：刪除個資時連帶刪掉帳務需要的關聯。
- **Survey**：ongoing due diligence、data retention policy、right to erasure vs record keeping
- **產出**：排程 job、`docs/security-and-compliance.md` 的入駐章節、`docs/notes/p08-review.md`

## 階段檢核

- **面試題**：
  1. KYC 與 KYB 的差別？為什麼支付平台要做 KYB？
  2. 你的系統保存了哪些個資？為什麼不存證件影像？
  3. KYC vendor 掛掉了，商戶入駐會怎樣？
  4. 制裁篩查誤判時怎麼處理？
  5. 合規審核的結果怎麼真正影響收款與出款？
- **故障場景**：vendor 逾時（fail closed）；vendor webhook 重複與亂序；出款前篩查命中；已核准商戶被暫停時的進行中出款。
- **tag**：`p08-onboarding-done`
