# P12 多渠道編排（9 次迭代）

- **對應模組**：#05 Payment Orchestration & Routing、#06 Connector（第二家 PSP）
- **階段目標**：接入第二家 PSP；依能力、規則、健康度選擇渠道；只有在能證明原交易未被 PSP 接受時才 failover。
- **前置**：P03（UNKNOWN 與 recovery）、P11（token 可攜性的限制）。
- **結束時的架構**：`connector` 模組新增 routing 子套件與第二家 adapter（`connector` schema 在 P02 的 merchant_connector_accounts 之外，新增 capabilities、route_decisions、health）。I-02.05b 的固定路由在 I-12.04 被規則路由取代，Payment Core 不需要改動。

**為什麼這個階段刻意延後**：ProjectRoadmap 把多 PSP 放在 W6。這裡延後到 P12，是因為單純多接一家 PSP 學到的是 API 整合；編排真正難的部分（UNKNOWN 時不 failover、能力差異、token 綁定）要等可靠性、帳務、對帳都完成後，才看得出代價。這個階段也是閱讀 Hyperswitch 路由程式碼最好的時機。

延後的是「接第二家」和「路由」，不是抽象本身：Connector 的分層、Router 介面、商戶渠道帳號、decorator 鏈在 I-02.05a、I-02.05b、I-03.06a 就已經就位，這個階段應該只新增實作，不改 Payment Core。

---

### I-12.01 第二家 PSP：授權

- **目標**：接入 Adyen test 或 PayPal sandbox，實作同一個 Connector 介面的授權。
- **為什麼**：第二家 PSP 會暴露你的 Connector 介面裡藏著多少 Stripe 的假設。
- **不做**：請款、退款（下一次）。
- **先想清楚**：
  1. 兩家的授權流程有什麼差異（同步、非同步、3DS 的處理方式）？
  2. 你的介面有哪些地方要改？改動應該只在 adapter 裡嗎？
- **完成條件**：sandbox 測試成功；Payment Core 沒有任何改動（若有，寫進筆記並說明原因）。
- **常見陷阱**：為了第二家 PSP 在核心加 `if provider == ...`。
- **Survey**：Adyen Checkout API、PayPal Orders API、Hyperswitch connectors
- **產出**：第二家 adapter

### I-12.02 第二家 PSP：請款、退款、webhook、狀態映射

- **目標**：完成第二家的 capture、refund、webhook 驗簽與狀態映射，並加入錄製回應的契約測試。
- **為什麼**：狀態映射要對兩家一致，同時保留原始狀態。
- **不做**：路由。
- **先想清楚**：
  1. 兩家對「部分退款」「請款逾期」的表達方式差在哪？
  2. webhook 驗簽的機制有什麼不同？
- **完成條件**：I-02.13 的 conformance suite 在第二家 adapter 上全部通過，或明確標記不支援的項目；回頭對照 I-02.05a 的紙上映射，記錄哪些預測對了、哪些沒想到。
- **常見陷阱**：把不支援的功能默默跳過，而不是回明確錯誤。
- **Survey**：Adyen webhooks HMAC、PayPal webhook verification
- **產出**：完整 adapter、契約測試

### I-12.03 Capability matrix

- **目標**：把每家 PSP 支援的支付方式、幣別、3DS、部分請款、部分退款、webhook、payout 寫成資料。
- **為什麼**：路由前要先知道「哪幾家能做這筆交易」。
- **不做**：路由決策。
- **先想清楚**：
  1. 能力是靜態設定，還是由商戶的 PSP 帳號設定決定？
  2. 交易進行中發現能力不支援（例如部分請款），要怎麼處理？
- **完成條件**：`docs/provider-capabilities.md` 與資料表一致；查詢「USD 卡片付款 + 部分請款」回傳正確的 PSP 清單。
- **常見陷阱**：能力只寫在文件裡，程式不使用。
- **Survey**：Hyperswitch connector feature matrix
- **產出**：capabilities 表

### I-12.04 路由規則與決策紀錄

- **目標**：依商戶、幣別、支付方式、金額選擇 PSP；每次決策記錄輸入、候選、選中與理由。
- **為什麼**：路由決策會直接影響成功率與成本；出問題時要能解釋「為什麼這筆走了這家」。
- **不做**：健康度與成功率。
- **先想清楚**：
  1. 規則的表達方式能否重用 P10 的規則引擎？
  2. 已存卡（綁定在某家 PSP 的 token）怎麼影響路由？
  3. 沒有任何 PSP 符合時回什麼？
- **完成條件**：表驅動測試；決策紀錄可以查詢；這是 I-02.05b Router 介面的新實作，Payment Core 沒有改動。
- **常見陷阱**：路由到一家 PSP，但卡片 token 屬於另一家。
- **Survey**：Hyperswitch routing algorithms、priority vs volume split routing
- **產出**：router、route_decisions

### I-12.05 Provider 健康度

- **目標**：依每家 PSP 的錯誤率與延遲計算健康度，提供給路由使用。
- **為什麼**：PSP 會局部故障；路由要能避開正在出問題的渠道。
- **不做**：circuit breaker（下一次）。
- **先想清楚**：
  1. 「拒絕」算不算錯誤？（持卡人資金不足不是 PSP 的問題）
  2. 健康度的時間窗口？樣本太少時怎麼判斷？
- **完成條件**：Mock PSP 注入錯誤率上升時，健康度下降；正常拒絕不影響健康度。
- **常見陷阱**：把 hard decline 算成 PSP 不健康。
- **Survey**：provider health scoring、success rate routing
- **產出**：health 元件、metrics

### I-12.06 Circuit breaker

- **目標**：PSP 錯誤率超過門檻時斷路，一段時間後半開試探。
- **為什麼**：持續打一個已經故障的 PSP，只會累積更多 UNKNOWN。
- **不做**：跨實例共享斷路狀態（筆記討論）。
- **先想清楚**：
  1. 斷路狀態要放在記憶體還是 Redis？多個實例看到的狀態不同時會怎樣？
  2. 斷路時，新的付款路由到哪裡？
- **完成條件**：測試：注入故障後斷路，新請求不再送往該 PSP；恢復後半開成功、關閉斷路。
- **常見陷阱**：斷路器把 UNKNOWN 的交易也當成可以改送另一家的理由。
- **設計模式**：Circuit breaker，做成 I-03.06a decorator 鏈上的新一層，不寫進 adapter
- **Survey**：circuit breaker pattern、sony/gobreaker
- **產出**：breaker

### I-12.07 Soft decline 與 hard decline

- **目標**：把拒絕原因分為可重試（soft）與不可重試（hard）；只有 soft decline 可以考慮用新的 attempt 重試。
- **為什麼**：對 hard decline 重試（例如卡片被盜）會被卡組織罰款，也可能觸發風控。
- **不做**：Revenue recovery 等級的重試策略。
- **先想清楚**：
  1. 哪些拒絕碼是 soft？可以換一家 PSP 重試嗎？
  2. 卡組織對重試次數有什麼限制？
- **完成條件**：測試：hard decline 不重試；soft decline 依規則最多重試一次。
- **常見陷阱**：所有拒絕都重試。
- **Survey**：soft decline vs hard decline、Visa retry rules、Hyperswitch GSM
- **產出**：decline 分類擴充

### I-12.08 安全 failover

- **目標**：只有在原 attempt 被確認「未被 PSP 接受」（例如連線前就失敗、PSP 明確回拒且為 soft decline）時才 failover 到第二家；UNKNOWN 一律不 failover。
- **為什麼**：這是 ProjectRoadmap 架構約束第 3 條，也是重複扣款最危險的來源。
- **不做**：自動化人工流程。
- **先想清楚**：
  1. 「連線前就失敗」怎麼確定？DNS 失敗、TLS 失敗、送出後逾時分別屬於哪一類？
  2. 第一家的 attempt 在 UNKNOWN，商戶等太久怎麼辦？
- **完成條件**：故障場景「PSP A 逾時但已扣款」：系統不向 PSP B 扣款；recovery 收斂後付款成功且只有一筆扣款。
- **常見陷阱**：把「HTTP 逾時」歸類為「未送出」。
- **設計模式**：安全 failover、逾時即未知
- **Survey**：safe failover payments、unknown outcome
- **產出**：failover 規則、`docs/evidence/no-blind-failover.md`

### I-12.09 成本與成功率路由（含階段檢核）

- **目標**：在符合能力的候選中，依成本與歷史成功率選擇；每次決策可稽核。之後閱讀 Hyperswitch 的動態路由程式碼並寫比較筆記。
- **為什麼**：這是編排平台的核心商業價值；同時讓你知道生產系統比你的版本多做了什麼。
- **不做**：機器學習。
- **先想清楚**：
  1. exploration 與 exploitation 是什麼？為什麼純粹選最高成功率會有問題？
  2. 成本與成功率衝突時怎麼取捨？
- **完成條件**：模擬測試：成功率較高的 PSP 獲得較多流量，但仍保留少量探索；比較筆記完成。
- **常見陷阱**：用太少的樣本決定路由。
- **Survey**：multi-armed bandit routing、Hyperswitch dynamic routing、decision-engine
- **產出**：路由策略、`docs/provider-adapter.md`、`docs/notes/p12-review.md`

## 閱讀作業（建議在 I-12.04 之後）

在 Hyperswitch 原始碼中閱讀：

- `crates/router/src/core/payments/routing`：路由流程
- `crates/euclid`：規則語言
- `crates/router/src/core/revenue_recovery`：重試策略
- 任一個 `crates/hyperswitch_connectors/src/connectors/<psp>`：adapter 如何把一家 PSP 收成統一介面

寫一份筆記：你的設計與它的設計各自的取捨。

## 階段檢核

- **面試題**：
  1. 什麼時候可以安全地 failover 到另一家 PSP？
  2. 路由決策有哪些輸入？出問題時怎麼解釋？
  3. Soft decline 和 hard decline 的差別？
  4. 為什麼加第二家 PSP 不等於做了編排？
- **故障場景**（ProjectRoadmap 場景 12）：PSP A 逾時但已扣款；PSP A 故障觸發斷路；能力不符時的路由。
- **tag**：`p12-orchestration-done`
