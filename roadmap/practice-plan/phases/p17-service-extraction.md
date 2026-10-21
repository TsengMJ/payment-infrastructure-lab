# P17 抽出服務（14 次迭代）

- **對應**：架構文件第 2 節（8 個邏輯服務）與第 3 節（5 個部署服務）、第 9 節的 ADR 摘要
- **階段目標**：依 a → e 的順序抽出 webhook-delivery、connector-service、ledger-service、settlement-service、payout-service，最後形成架構文件第 3 節的 5 個部署服務（payment-platform 加上 connector、ledger、settlement、payout），並證明資金不變量在網路邊界下仍然成立。
- **前置**：P16（每模組 DB role、trace、告警、故障演練）。
- **結束時的架構**：同一個 repo、`go.work` 多 module；`contracts/` 放 proto 或 OpenAPI；每個服務在 `services/<name>/` 下有自己的 `cmd/`、Dockerfile、DB role 或資料庫；其餘模組留在 `payment-platform`。

先讀 [`02-architecture-evolution.md`](../02-architecture-evolution.md) 第 3 節。每次抽出都照那裡的固定步驟：盤點 → 前置條件 → 行程內 client → 契約 → 網路實作 → 拆部署 → 資料搬遷 → 故障測試 → ADR。

**關於數量**：webhook-delivery 在架構文件第 3 節屬於 payment-platform 的「獨立背景 Worker」，不算進 5 個部署服務。本階段仍把它第一個抽成獨立部署，因為它風險最低，適合先練一次完整的抽出步驟。

---

### I-17.01 抽出前盤點

- **目標**：產生模組依賴圖、資料所有權表、同步呼叫清單、事件清單。
- **為什麼**：抽出服務的成本主要來自你沒發現的依賴。
- **不做**：任何程式改動。
- **先想清楚**：
  1. 有沒有模組之間直接 JOIN 的查詢？P15 的 read model 之外還有嗎？
  2. 哪些同步呼叫在抽出後會變成網路呼叫，且在交易的關鍵路徑上？
  3. 依據盤點結果，`02-architecture-evolution.md` 的抽出順序還合理嗎？
- **完成條件**：`docs/service-extraction/inventory.md`；依賴圖由工具產生（例如 go-callvis 或 import 圖）。
- **常見陷阱**：只看 import，沒看資料庫層的隱性依賴。
- **設計模式**：Strangler fig
- **Survey**：dependency graph Go、strangler fig pattern
- **產出**：盤點文件

### I-17.02 go.work 多 module 與 contracts

- **目標**：同一個 repo 改成多個 Go module（`services/` 下每個未來服務一個，加上共用的 platform 與 contracts）；建立 `contracts/` 與 breaking change 檢查。
- **為什麼**：module 邊界讓依賴更明確；契約檢查讓你不會意外破壞其他服務。
- **不做**：拆部署。
- **先想清楚**：
  1. 用 gRPC（proto + buf）還是 HTTP（OpenAPI）？取捨是什麼？
  2. platform 共用套件會不會變成所有服務的耦合點？
  3. CI 怎麼只跑受影響的 module？
- **完成條件**：`go work sync` 後全部測試通過；修改 proto 移除欄位時，breaking 檢查失敗。
- **常見陷阱**：把業務型別放進共用 module，所有服務綁在一起。
- **Survey**：go workspaces、buf breaking、OpenAPI diff
- **產出**：`go.work`、`contracts/`、ADR 0027「服務間協定」

### I-17.03 零停機 migration

- **目標**：用 expand / contract 方式完成一次欄位改名或資料搬遷，過程中新舊版本的程式可以同時運作。
- **為什麼**：抽出服務、搬資料時，新舊版本一定會並存；停機遷移在付款系統裡通常不可接受。
- **不做**：真正的服務抽出。
- **先想清楚**：
  1. expand、migrate、contract 三步各自要部署幾次？
  2. 回填大量資料時怎麼避免鎖表？
- **完成條件**：模擬測試：在持續寫入的狀態下完成一次欄位改名，期間沒有錯誤。
- **常見陷阱**：同一次部署同時改 schema 與程式。
- **設計模式**：Expand / contract
- **Survey**：expand and contract migration、online schema change Postgres
- **產出**：遷移手冊 `docs/runbooks/zero-downtime-migration.md`

### I-17.04 抽出 a：webhook-delivery

- **目標**：webhook delivery consumer 變成獨立部署的 worker，只消費 RabbitMQ、只寫 `webhook` schema 的投遞紀錄。
- **為什麼**：它只消費事件、不擁有資金資料，是風險最低的第一次練習。
- **不做**：endpoint 管理 API（仍在 payment-platform）。
- **先想清楚**：
  1. endpoint 設定（URL、secret）由 payment-platform 擁有，delivery worker 要怎麼讀？
  2. 獨立擴容時，同一個 endpoint 的投遞會不會同時太多？
- **完成條件**：獨立容器運行；I-09 的故障場景全部重跑通過。
- **常見陷阱**：delivery worker 直接讀 payment-platform 的其他表。
- **Survey**：competing consumers、consumer scaling RabbitMQ
- **產出**：`services/webhook-delivery`、ADR 0028

### I-17.05 抽出 b1：connector 行程內 client

- **目標**：payment 與 refund 改用 `ConnectorClient` 介面呼叫 connector；實作暫時仍在行程內；provider webhook 的接收端點歸 connector 所有。
- **為什麼**：先把呼叫方式換好、測試全部通過，下一步換成網路呼叫時才只改一個地方。
- **不做**：網路呼叫。
- **先想清楚**：
  1. provider webhook 由 connector 接收、正規化後，要怎麼通知 payment：事件還是呼叫？
  2. 路由（routing）在 connector 裡，那 payment 還需要知道哪些 PSP 資訊？
- **完成條件**：所有既有測試通過；payment 不再 import connector 的任何非公開型別。
- **常見陷阱**：client 介面直接暴露 PSP 型別。
- **Survey**：client interface pattern、anti-corruption layer
- **產出**：client 介面

### I-17.06 抽出 b2：connector-service

- **目標**：connector 成為獨立服務（gRPC 或 HTTP）；payment 呼叫時傳遞 attempt 級冪等 key；網路逾時被歸為 UNKNOWN。
- **為什麼**：現在 payment 與 PSP 之間多了一跳網路，UNKNOWN 的來源從一個變兩個。
- **不做**：獨立資料庫。
- **先想清楚**：
  1. payment → connector 逾時，但 connector → PSP 成功了，recovery 要問誰？
  2. connector-service 自己的重試會不會和 payment 的重試疊加？
  3. connector-service 需要自己的 inbox 與 outbox 嗎？
- **完成條件**：故障場景：在 payment 與 connector 之間注入延遲與斷線，沒有重複扣款，recovery 能收斂。
- **常見陷阱**：兩層都重試，請求數量相乘。
- **設計模式**：Retry budget 與 deadline 傳遞、Idempotency key（跨網路邊界）
- **Survey**：gRPC deadlines、retry budgets、idempotency across services
- **產出**：`services/connector`、`docs/evidence/connector-network-split.md`

### I-17.07 抽出 c1：ledger API 契約

- **目標**：定義 ledger 的服務契約：Post（冪等）、Reverse、Hold、Release、CaptureHold、GetBalance；明確哪些是同步、哪些由事件觸發。
- **為什麼**：ledger 是唯一的帳務寫入者，它的契約決定其他服務如何正確地動錢。
- **不做**：部署。
- **先想清楚**：
  1. 為什麼 Hold 必須是同步 API，而 capture 過帳可以是事件？
  2. 冪等 key 由呼叫方提供還是 ledger 產生？
  3. 錯誤要怎麼分類（餘額不足、重複、不平衡、暫時性錯誤）？
- **完成條件**：契約文件與契約測試；行程內實作通過契約測試。
- **常見陷阱**：提供通用的「寫一筆 posting」API，讓呼叫方自己組分錄，破壞過帳規則的集中性。
- **Survey**：Modern Treasury ledger API、TigerBeetle、ledger service design
- **產出**：ledger proto 或 OpenAPI、ADR 0029

### I-17.08 抽出 c2：ledger-service

- **目標**：ledger 成為獨立服務，使用獨立的資料庫（或至少獨立的連線與角色）；只透過 API 與事件互動。
- **為什麼**：帳務不變量與寫入權限需要最強的隔離。
- **不做**：多區域。
- **先想清楚**：
  1. 資料搬到新資料庫時，怎麼確保搬遷期間沒有遺漏或重複過帳？
  2. ledger-service 掛掉時，付款還能繼續嗎？過帳會不會遺失？
- **完成條件**：搬遷後 ledger 平衡檢查、property test、對帳全部通過；ledger-service 停止期間付款繼續，恢復後過帳補齊。
- **常見陷阱**：搬遷時停止寫入卻沒有停止 outbox 消費。
- **Survey**：database per service、data migration verification
- **產出**：`services/ledger`

### I-17.09 抽出 d：settlement-service

- **目標**：結算、對帳、手續費成為獨立服務；匯入 worker 與對帳 job 可獨立擴容；只透過 ledger API 讀取與調整。
- **為什麼**：批次 job 的資源使用模式與交易 API 完全不同。
- **不做**：資料倉儲。
- **先想清楚**：
  1. 對帳需要大量讀取 ledger 資料，用 API 會不會太慢？要不要改用事件建立自己的副本？
  2. 手續費規則屬於 settlement，但 capture 時就要算平台費，這個依賴怎麼處理？
- **完成條件**：P06 的所有對帳測試在新服務上通過；100,000 筆對帳時間記錄在報告中。
- **常見陷阱**：settlement-service 直接連 ledger 的資料庫讀資料。
- **Survey**：batch processing service、event-carried state transfer
- **產出**：`services/settlement`

### I-17.10 抽出 e：payout-service

- **目標**：payout 成為獨立服務；同步呼叫 ledger-service 的 Hold；出款權限、銀行憑證只存在這個服務。
- **為什麼**：出款的風險與權限邊界從一開始就應該獨立（架構文件第 9 節）。
- **不做**：多家銀行。
- **先想清楚**：
  1. payout → ledger 的 Hold 呼叫逾時了，hold 到底建立了沒有？怎麼查？
  2. payout-service 和 ledger-service 都掛過之後，hold 與 payout 狀態怎麼對齊？
- **完成條件**：故障場景：併發 payout 70 + 70 在跨服務下仍只有一筆成功；Hold 呼叫逾時不會造成重複 hold 或漏 hold。
- **常見陷阱**：payout-service 快取餘額來判斷能否出款。
- **Survey**：synchronous reservation across services
- **產出**：`services/payout`、`docs/evidence/cross-service-payout.md`

### I-17.11 跨服務 trace 與契約測試

- **目標**：一筆付款從 API 到 PSP、ledger、webhook 的 trace 在 Grafana 中完整串起；每對服務之間有契約測試在 CI 執行。
- **為什麼**：拆成多個服務後，沒有 trace 的除錯幾乎不可能。
- **不做**：服務網格。
- **先想清楚**：
  1. trace context 怎麼穿過 RabbitMQ 訊息？
  2. 契約測試用 consumer-driven（Pact）還是 schema 檢查就夠？
- **完成條件**：一筆付款的 trace 包含所有服務的 span；改壞一個契約時 CI 失敗。
- **常見陷阱**：非同步事件沒有傳遞 trace context，trace 在 broker 處斷掉。
- **Survey**：OTel context propagation messaging、Pact contract testing
- **產出**：trace 傳遞、契約測試

### I-17.12 跨服務故障演練

- **目標**：重跑 P16 的故障演練，加上服務間的網路分割、單一服務當機、服務版本不一致。
- **為什麼**：抽出服務的代價在故障時最明顯，這次要親眼看見。
- **不做**：生產環境演練。
- **先想清楚**：
  1. 哪些故障在 monolith 時不存在、現在才出現？
  2. 哪些不變量仍然由單一服務保證，哪些現在要靠對帳兜底？
- **完成條件**：演練報告比較 monolith 與多服務在相同故障下的表現；資金不變量全部成立。
- **常見陷阱**：只演練 happy path 的部署。
- **Survey**：network partition testing、toxiproxy
- **產出**：`docs/evidence/game-day-services.md`

### I-17.13 部署與 payment-infra repo（選做）

- **目標**：Docker Compose 支援多服務；選做：用 Helm 部署到本機 Kubernetes（kind 或 k3d），並把部署程式碼拆到 `payment-infra` repo。
- **為什麼**：體驗部署設定與應用程式分開管理的理由（權限不同、發版節奏不同）。
- **不做**：雲端部署。
- **先想清楚**：
  1. 拆 repo 之後，應用程式版本與部署設定版本怎麼對應？
  2. 為什麼服務本身不拆 repo（見 `02-architecture-evolution.md` 第 4.3 節）？
- **完成條件**：`make demo-up` 啟動所有服務；選做部分有 README。
- **常見陷阱**：為了體驗多 repo 而把每個服務拆出去。
- **Survey**：Helm charts、kind、GitOps
- **產出**：compose 設定、選做的 `payment-infra` repo

### I-17.14 Edge gateway 與總檢核（edge gateway 為選做）

- **目標**：選做：在前面加一層 Envoy 或 Nginx，處理 TLS、全域限流、WAF 規則。必做：依 `02-architecture-evolution.md` 第 3.6 節評估是否拆到 8 個服務，寫成 ADR；完成整個計劃的總檢核。
- **為什麼**：架構文件區分了 Edge API Gateway 與業務 API；總檢核讓你確認 18 個模組都真正落地。
- **不做**：真的拆到 8 個服務（除非評估結果明確需要）。
- **先想清楚**：
  1. edge 層的限流與 P01 的商戶限流各自防什麼？
  2. 目前哪一個模組最有理由下一個被拆出去？理由是技術還是組織？
- **完成條件**：ADR 0030「是否拆到 8 個服務」；`docs/final-review.md` 對照 `00-overview.md` 的 18 模組表，逐一列出實作位置與證據。
- **常見陷阱**：為了數字好看而拆服務。
- **Survey**：API gateway vs service mesh、Envoy rate limit、ModSecurity
- **產出**：最終檢核文件、ADR 0030

## 階段檢核

- **面試題**：
  1. 你為什麼先做 Modular Monolith？什麼時候、依什麼順序拆？
  2. 拆成服務後，PSP 逾時的處理變得更難在哪裡？
  3. 出款預留為什麼必須是同步呼叫？
  4. 跨服務一致性你怎麼保證？為什麼不用分散式交易？
  5. 什麼情況下你會把服務合回去？
- **故障場景**：connector 網路分割；ledger-service 停止；跨服務併發出款；服務版本不一致。
- **tag**：`p17-extraction-done`
