# P10 Risk（8 次迭代）

- **對應模組**：#08 Fraud & Risk Engine
- **階段目標**：付款前有一個同步的風控決策點，回傳 Allow、Review、Deny；規則可資料化、可版本化、可稽核；能接入外部風險分數作為輸入。
- **前置**：P02（付款流程）、P09（決策事件可通知商戶）。
- **結束時的架構**：`risk` 模組（`risk` schema：rules、rule_versions、decisions、lists）；Redis 用於 velocity counter；payment 在 confirm 前同步呼叫 risk。

---

### I-10.01 決策點與介面

- **目標**：payment 在 confirm 時同步呼叫 `risk.Evaluate`，拿到 Allow / Review / Deny 與理由；目前規則固定回 Allow。
- **為什麼**：先把決策點放對位置，規則可以之後慢慢加。
- **不做**：任何規則。
- **先想清楚**：
  1. 風控逾時或出錯時，付款要放行還是拒絕？大額與小額的答案一樣嗎？
  2. 決策要存什麼才能事後解釋（輸入特徵、規則版本、分數）？
  3. Review 的付款狀態是什麼？授權要先做還是等審核？
- **完成條件**：測試：Deny 時不呼叫 PSP；風控逾時時的行為符合 ADR。
- **常見陷阱**：風控邏輯散落在 payment 模組裡。
- **Survey**：risk decisioning、fail open vs fail closed
- **產出**：risk 介面、ADR 0021「風控逾時策略」

### I-10.02 最小規則引擎

- **目標**：規則以資料表示（條件 + 動作 + 優先順序），依序評估。
- **為什麼**：風控規則常常需要快速調整，不能每次都改程式部署。
- **不做**：自訂 DSL 解析器（先用結構化條件）。
- **先想清楚**：
  1. 多條規則同時命中時，結果怎麼合併（最嚴格者優先）？
  2. 規則可以用哪些特徵（金額、幣別、國家、商戶類別、卡片 BIN）？
  3. 參考 Hyperswitch 的 euclid：它的規則表達方式有什麼優缺點？
- **完成條件**：表驅動測試涵蓋規則合併邏輯。
- **常見陷阱**：讓規則可以執行任意程式碼。
- **設計模式**：規則引擎 / Specification
- **Survey**：rule engine design、Hyperswitch euclid、Stripe Radar rules
- **產出**：規則資料模型與評估器

### I-10.03 Velocity counter

- **目標**：用 Redis 計算滑動窗口內的次數與金額（例如同一張卡 10 分鐘內 5 次失敗）。
- **為什麼**：卡片測試攻擊（card testing）的特徵就是短時間大量小額嘗試。
- **不做**：長期統計。
- **先想清楚**：
  1. 用什麼 key：卡片指紋、email、IP、裝置？
  2. 滑動窗口在 Redis 怎麼實作（sorted set、多個固定窗口）？
  3. Redis 資料遺失時，風控會變寬鬆還是嚴格？
- **完成條件**：測試：超過門檻後的請求被 Deny；窗口過後恢復。
- **常見陷阱**：用 merchant_id 當唯一維度，抓不到跨商戶的攻擊。
- **Survey**：card testing attack、velocity checks、Redis sliding window
- **產出**：velocity 元件

### I-10.04 黑名單與資料最小化

- **目標**：支援卡片指紋、email、IP 黑名單；只存雜湊或 PSP 提供的指紋，不存卡號。
- **為什麼**：已知詐騙來源要能快速封鎖；但黑名單本身也是敏感資料。
- **不做**：共享黑名單。
- **先想清楚**：
  1. 卡片指紋從哪裡來（PSP 提供的 fingerprint）？
  2. 黑名單要不要到期？誰可以新增與移除？
- **完成條件**：測試：黑名單命中時 Deny；資料庫沒有明文 email。
- **常見陷阱**：把 PAN 前 6 後 4 加上到期日當指紋（可能被還原）。
- **Survey**：Stripe card fingerprint、hashed blocklist、Hyperswitch blocklist
- **產出**：lists 表

### I-10.05 Allow / Review / Deny 與審核佇列

- **目標**：Review 的付款進入審核佇列；審核人員核准後繼續請款，拒絕則取消授權。
- **為什麼**：不是每個可疑交易都該直接拒絕；人工審核是在損失與轉換率之間取捨。
- **不做**：UI（P15）。
- **先想清楚**：
  1. Review 時先授權再等審核，還是先審核再授權？授權會過期的影響？
  2. 審核逾時沒人處理怎麼辦？
- **完成條件**：測試：Review 的付款不會被自動請款；核准後請款；拒絕後 void。
- **常見陷阱**：Review 的付款被商戶的自動請款流程直接請款。
- **Survey**：manual review fraud、Stripe Radar review
- **產出**：審核佇列 API

### I-10.06 3DS 觸發策略

- **目標**：依風險決策要求 3DS（例如高金額或中風險時強制 3DS）。
- **為什麼**：3DS 可以把詐騙責任轉移給發卡行，但會降低轉換率；風控決定何時值得。
- **不做**：自建 3DS server。
- **先想清楚**：
  1. 什麼是 liability shift？frictionless 3DS 也有嗎？
  2. SCA 豁免（低金額、低風險）由誰決定？
- **完成條件**：測試：中風險付款送往 Stripe 時帶上強制 3DS 參數。
- **常見陷阱**：所有付款都強制 3DS。
- **Survey**：3DS liability shift、SCA exemptions、Stripe request_three_d_secure
- **產出**：risk 輸出加入 3DS 指示

### I-10.07 規則版本、shadow mode 與稽核

- **目標**：規則每次變更產生新版本；新規則可以先以 shadow mode 運行（只記錄、不影響決策）；所有變更有稽核紀錄。
- **為什麼**：一條錯誤的規則可能在幾分鐘內擋掉所有正常付款。
- **不做**：A/B 測試。
- **先想清楚**：
  1. 決策紀錄要記錄當時的規則版本，才能事後解釋嗎？
  2. shadow mode 的結果要怎麼比較？
- **完成條件**：測試：shadow 規則命中時決策不變，但有紀錄；回溯查詢某筆決策用的規則版本。
- **常見陷阱**：直接修改正在運行的規則，無法回溯。
- **設計模式**：Shadow mode、Effective-dated records
- **Survey**：shadow mode rules、rule versioning
- **產出**：rule_versions、稽核紀錄

### I-10.08 外部風險分數（含階段檢核）

- **目標**：把外部風險分數（例如 Stripe Radar 回傳的 risk level，或 Sift sandbox 的分數）當作規則的輸入特徵。
- **為什麼**：外部模型看得到你看不到的跨平台資料；但最終決策仍要由你自己的規則做出並能解釋。
- **不做**：自建機器學習模型。
- **先想清楚**：
  1. 外部分數拿不到（逾時）時，規則怎麼評估？
  2. 外部分數是在授權前還是授權後才有？這影響它能用在哪一步。
- **完成條件**：測試：外部分數高時規則回 Review；外部服務逾時時依 ADR 處理。
- **常見陷阱**：外部分數直接等於決策。
- **Survey**：Stripe Radar risk_level、Sift score API
- **產出**：外部分數 adapter、`docs/notes/p10-review.md`

## 階段檢核

- **面試題**：
  1. 風控服務掛了，付款要繼續嗎？
  2. 卡片測試攻擊長什麼樣？你怎麼擋？
  3. 什麼時候要求 3DS？liability shift 是什麼？
  4. 怎麼安全地上線一條新規則？
- **故障場景**：風控逾時；velocity 攻擊；錯誤規則在 shadow mode 下被發現。
- **tag**：`p10-risk-done`
