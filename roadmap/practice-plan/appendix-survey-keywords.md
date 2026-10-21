# 附錄：Survey 關鍵字、官方文件入口、第三方 sandbox 注意事項

這份附錄給「先想清楚」階段查資料用。關鍵字是搜尋起點，不是標準答案；連結以官方文件入口為主，實際內容與條款會變，以當下官方說明為準。

查資料的順序建議：

1. 先讀官方文件（PSP、資料庫、broker 的行為以官方為準）。
2. 再讀工程部落格或開源實作，看別人怎麼取捨。
3. 最後才問 AI，並請它指出來源。

---

## 1. 共通文件入口

| 主題 | 入口 |
|---|---|
| Go | [go.dev/doc](https://go.dev/doc/)、[Effective Go](https://go.dev/doc/effective_go) |
| PostgreSQL | [官方文件](https://www.postgresql.org/docs/current/)（交易隔離、鎖、分區、備份這幾章最常用） |
| Redis | [redis.io/docs](https://redis.io/docs/) |
| RabbitMQ | [rabbitmq.com/docs](https://www.rabbitmq.com/docs)（publisher confirms、consumer acknowledgements、DLX） |
| OpenTelemetry | [Go 文件](https://opentelemetry.io/docs/languages/go/) |
| Prometheus / Grafana | [prometheus.io/docs](https://prometheus.io/docs/)、[grafana.com/docs](https://grafana.com/docs/) |
| k6 | [grafana.com/docs/k6](https://grafana.com/docs/k6/latest/) |
| Next.js | [nextjs.org/docs](https://nextjs.org/docs) |
| Stripe | [docs.stripe.com](https://docs.stripe.com/)、[測試卡與測試情境](https://docs.stripe.com/testing) |
| Adyen | [docs.adyen.com](https://docs.adyen.com/)、[Payments lifecycle](https://docs.adyen.com/account/payments-lifecycle/) |
| 帳務模型 | [Modern Treasury Ledgers](https://docs.moderntreasury.com/ledgers/docs/overview)、[TigerBeetle 文件](https://docs.tigerbeetle.com/) |
| 分散式模式 | [microservices.io patterns](https://microservices.io/patterns/)（outbox、saga、database per service） |
| PCI | [PCI Security Standards Council](https://www.pcisecuritystandards.org/) |
| API 安全 | [OWASP API Security Top 10](https://owasp.org/API-Security/) |
| 參考實作 | [Hyperswitch](https://github.com/juspay/hyperswitch)（P12 閱讀作業） |

書籍（選讀，遇到瓶頸時再看）：

- *Designing Data-Intensive Applications*：交易、一致性、事件流。P03、P05、P17 最有幫助。
- *Accounting for Developers*（Modern Treasury 的系列文章）：P05 之前讀。
- *Release It!*（Michael Nygard）：timeout、circuit breaker、bulkhead 的原始出處。P03、P12 最有幫助。
- *Enterprise Integration Patterns*（Hohpe、Woolf）：idempotent receiver、dead letter channel、process manager。P03、P09 查閱用。

系統設計模式的整理與對應迭代見 [`03-design-patterns.md`](./03-design-patterns.md)。

---

## 2. 各階段 Survey 關鍵字

### P00 基礎建設

- Go project layout、Go internal packages、golangci-lint depguard、go-arch-lint
- Docker Compose healthcheck、PostgreSQL schema per module
- golang-migrate、goose、atlas
- 12-factor config、structured logging slog、log redaction
- testcontainers-go、table-driven tests
- OpenTelemetry Go SDK、OTLP collector、Prometheus Go client
- GitHub Actions Go、gosec、govulncheck、gitleaks

### P01 商戶與 Gateway API

- API key design prefix、hashing API keys、Stripe API keys test mode
- multi-tenant data isolation、row level security Postgres
- correlation ID propagation、W3C trace context
- RFC 9457 problem details、Stripe error codes
- OpenAPI 3.1、API versioning Stripe、cursor pagination
- ISO 4217 minor units、money type integer
- Redis rate limiting token bucket、sliding window、fail open vs fail closed

### P02 Payment Core 與 Stripe

- Stripe PaymentIntent lifecycle、PaymentIntent vs Charge
- payment attempt model、Hyperswitch payment_attempt
- finite state machine transition table、optimistic locking version column
- anti-corruption layer、canonical payment status
- Go interface segregation、optional interfaces、Hyperswitch merchant_connector_account、registry pattern、strategy pattern Go
- Stripe manual capture、partial capture、authorization expiry
- 3D Secure 2 flow、requires_action、Stripe Elements
- decline codes normalization、soft vs hard decline
- consumer-driven contract testing、golden files

### P03 可靠性

- idempotency key design、Stripe idempotent requests、IETF Idempotency-Key header draft
- request fingerprint hash、unique constraint ON CONFLICT
- PSP idempotency、unknown outcome payments、timeout is not failure
- fault injection mock server、toxiproxy
- decorator pattern Go、http.RoundTripper middleware、golang.org/x/time/rate
- recovery worker、reconcile by query
- Stripe webhook signature verification、replay window
- inbox pattern、webhook out-of-order events
- transactional outbox、FOR UPDATE SKIP LOCKED、at-least-once delivery

### P04 退款與取消

- void vs refund、Stripe refunds API
- partial refund cumulative limit、concurrent refund race
- refund pending status、refund failure handling
- event sourcing light、state convergence

### P05 Ledger

- double-entry bookkeeping for developers、chart of accounts
- normal balance debit credit、assets liabilities equity
- immutable ledger reversal entries、append-only journal
- idempotent posting source_event_id
- pending vs available balance、balance snapshot
- ledger holds reservations、SELECT FOR UPDATE vs SERIALIZABLE
- hot account problem、sharded counters、pgbench custom scripts
- property-based testing Go rapid、gopter
- Modern Treasury ledger、TigerBeetle double-entry

### P06 結算、對帳、手續費

- settlement vs clearing、Stripe balance transactions、Stripe payout reconciliation report
- settlement report parsing、file ingestion idempotency
- reconciliation matching rules、tolerance window、business day calendar
- reconciliation break types、exception queue
- maker-checker adjustment
- interchange plus pricing、blended pricing、platform fee
- bank statement formats CSV、MT940、camt.053
- three-way reconciliation

### P07 Payout

- payout state machine、Stripe payouts、Stripe Connect transfers
- bank account verification、account number tokenization
- double spend payout、atomic reservation
- unknown transfer outcome、payout returns
- maker-checker approval、four-eyes principle
- payout batching、cutoff time
- negative balance、rolling reserve

### M1 作品集整理

- architecture decision records、C4 model
- demo script、portfolio README payment system
- STAR interview method

### P08 商戶入駐與合規

- KYB vs KYC、ultimate beneficial owner、CDD vs EDD
- onboarding state machine、capability gating
- Stripe Connect onboarding、Stripe Identity、Persona、Sumsub、Onfido、Middesk
- sanctions screening、PEP screening、fuzzy name matching、false positive handling
- OpenSanctions、OFAC SDN list
- bank account verification micro-deposits、Plaid Auth
- PII minimization、field-level encryption、data retention policy
- ongoing monitoring、periodic review

### P09 商戶出站 Webhook

- Standard Webhooks spec、webhook signing HMAC
- event catalog、event versioning、thin vs fat events
- RabbitMQ publisher confirms、quorum queues、dead letter exchange
- exponential backoff with jitter、retry budget
- webhook replay、delivery log
- Mailpit

### P10 Risk

- fraud rule engine、velocity checks Redis
- blocklist hashing、data minimization
- risk decision allow review deny、manual review queue
- 3DS exemptions、SCA PSD2、risk-based authentication
- shadow mode rules、rule versioning
- Stripe Radar、Sift

### P11 Checkout 與支付方式

- hosted checkout vs embedded、Stripe Checkout、Stripe Payment Element
- PCI DSS SAQ A vs SAQ A-EP、iframe hosted fields
- return URL handling、redirect payment methods
- payment links
- network tokenization、PSP token vs vault token
- digital wallets Apple Pay Google Pay web
- virtual account bank transfer、overpayment underpayment handling

### P12 多渠道編排

- payment orchestration、Hyperswitch routing、Hyperswitch connector integration
- capability matrix、payment method support by PSP
- smart routing success rate、cost based routing
- provider health score、circuit breaker sony/gobreaker
- soft decline retry、cascading retries risks
- safe failover unknown outcome
- Adyen test account、PayPal sandbox

### P13 Billing

- subscription state machine、Stripe Billing subscriptions
- customer initiated vs merchant initiated transactions、card on file mandate
- invoicing、proration
- smart retries、dunning
- test clocks Stripe、fake clock Go
- deferred revenue（概念即可）

### P14 Dispute 與 FX

- chargeback lifecycle、Stripe disputes、dispute evidence
- reason codes Visa Mastercard
- dispute funds withdrawal ledger
- FX rate source ECB reference rates、rate lock quote expiry
- multi-currency ledger、FX clearing account、FX gain loss
- FX spread revenue、cross-currency refund policy

### P15 報表與營運

- merchant dashboard auth、session vs JWT、OIDC
- RBAC multi-tenant
- transaction timeline、audit trail
- maker-checker、back office tools
- tamper-evident audit log hash chain
- CQRS read model、materialized view refresh

### P16 安全與可靠性收尾

- STRIDE threat modeling、OWASP threat modeling
- envelope encryption、HashiCorp Vault transit、SOPS、LocalStack KMS
- secret scanning、PII detection logs
- Postgres roles least privilege、GRANT per schema
- SLO burn rate alerts、RED metrics、USE metrics
- Postgres declarative partitioning、data archival
- pg_basebackup、WAL archiving、PITR、RPO RTO
- chaos engineering game day
- k6 scenarios、open vs closed model load testing、coordinated omission

### P17 抽出服務

- strangler fig pattern、modular monolith to microservices
- go workspaces、buf breaking、OpenAPI diff
- expand and contract migration、online schema change
- gRPC deadlines、retry budgets、idempotency across services
- database per service、event-carried state transfer
- OTel context propagation messaging、Pact contract testing
- toxiproxy network partition
- Helm、kind、k3d、GitOps
- API gateway vs service mesh、Envoy rate limit

---

## 3. 第三方 sandbox 注意事項

以下是申請與使用時常見的限制方向。各家條款、可用地區與費用會變，申請前以官方說明為準，並把選擇理由寫進 ADR（例如 I-08.04 的 ADR 0018）。

| 服務 | 用在 | 注意事項 |
|---|---|---|
| Stripe（test mode） | P02–P04、P07、P11、P13、P14 | 先確認你所在地區能否註冊帳戶；test mode 不需啟用正式帳戶。測試卡可以模擬拒絕、3DS、dispute。webhook 用 Stripe CLI 轉發到本機。test mode 也有 API 速率限制，不要拿來壓測 |
| Stripe Connect | P07、P08 | 練 connected account 與 onboarding 流程時才需要；平台帳戶所在地區有限制 |
| Stripe Identity | P08 | 在 test mode 可用測試結果模擬通過與失敗；不要上傳真人證件 |
| Adyen test account | P12 | 需要申請，可能需要公司資訊或審核時間；申請不到就改用 PayPal sandbox 或自製第二家 Mock PSP，並在 ADR 記錄 |
| PayPal sandbox | P12 | 開發者帳號即可建立 sandbox 帳戶；API 模型和 Stripe 差異大，正好練狀態映射 |
| Persona、Sumsub、Onfido | P08 | 多數需要申請 sandbox 或試用；拿不到就只做 Mock vendor，介面與 webhook 流程照樣練 |
| Middesk | P08 | 主要服務美國公司驗證，可用性看你的申請狀況；拿不到時用 Mock |
| OpenSanctions | P08 | 資料可下載，自建篩查；注意授權條款對商業使用的限制，個人學習一般可用 |
| ComplyAdvantage | P08 | 商業服務，通常需要洽詢；本計劃預設用 OpenSanctions 加自己的比對 |
| Plaid（Auth） | P08 | Sandbox 可用測試帳號；Plaid 支援的國家有限，不支援時用自製的 Mock 微額驗證 |
| Sift | P10 | 商業服務；本計劃預設只消費 Stripe Radar 結果或用 Mock 分數 |
| ECB 參考匯率 | P14 | 免費、每個工作日更新一次；只是參考匯率，不是可交易匯率，這點本身就是設計題 |
| Open Exchange Rates | P14 | 免費方案有請求數與基準幣別限制；做好快取 |
| HashiCorp Vault（dev mode） | P16 | dev mode 資料不持久，只能用在本機；不要把 root token 寫進 repo |
| LocalStack | P16 | 本機模擬 KMS；部分功能需要付費版，選用前確認 KMS 在免費版的支援程度 |
| Mailpit | P09 | 本機收信，不會真的寄出 |

共同規則：

- 所有 sandbox 金鑰放 secret 管理（P00 起是 `.env` 不進版控，P16 起是 Vault 或 SOPS）。
- 自動化測試只跑 Mock；sandbox 測試標記為手動或夜間執行，並限制次數。
- sandbox 的行為不等於正式環境，報告中要寫明哪些結論只在 sandbox 驗證過。

---

## 4. 與架構文件的交叉檢查

### 4.1 18 個模組

每個模組對應的迭代見 [`00-overview.md`](./00-overview.md) 第 5 節。撰寫完成時檢查過：

- 每個模組至少有一個「主要迭代」範圍，且所有被引用的迭代編號都存在於 `phases/` 中。
- 全部 19 份階段文件共 194 次迭代，沒有重複編號。

### 4.2 架構文件第 8 節的必測故障場景

| # | 場景 | 對應迭代 |
|---|---|---|
| 1 | 同一 Idempotency Key 100 次並發 | I-03.03 |
| 2 | Provider 已完成但 Connector 逾時 | I-03.06、I-03.07；跨服務版本 I-17.06 |
| 3 | Provider webhook 重複、亂序、晚到 | I-03.09、I-03.10、I-05.07 |
| 4 | DB 已提交但 broker 故障，outbox 補送 | I-03.11、I-03.12、I-09.04 |
| 5 | 部分退款與結算報表同時到達 | I-04.03、I-06.08、I-06.09 |
| 6 | 報表缺單、手續費不符、跨批次結算 | I-06.05–06.07、I-06.10、I-06.14 |
| 7 | 兩筆並行 payout 搶同一筆可用餘額 | I-05.11、I-07.03；跨服務版本 I-17.10 |
| 8 | Payout 送出後斷線，不可重送 | I-07.05；跨服務版本 I-17.10 |

I-M1.02 整理 ProjectRoadmap 的 12 個場景時，要確認這 8 個都包含在內並可重跑。P16 的 I-16.08 在組合故障下再演練一次，P17 的 I-17.12 在跨服務邊界下再演練一次。

### 4.3 架構文件其他章節

| 章節 | 對應 |
|---|---|
| 第 2–3 節：8 個邏輯服務、5 個部署服務 | [`02-architecture-evolution.md`](./02-architecture-evolution.md) 第 3 節、P17 |
| 第 4 節：完整生命週期與三種「成功」 | I-05.09（pending / available）、I-06.01（結算語意）、I-07.01（出款語意）、I-15.03（交易時間線） |
| 第 5 節：核心服務邊界 | P02（Payment Core）、P02 與 P12（Connector）、P05（Ledger）、P06（Settlement）、P07（Payout） |
| 第 6 節：可靠性設計 | `00-overview.md` 第 5 節的可靠性對照表 |
| 第 6 節：最小資料模型 | P01–P09 各自的 schema 迭代；欄位名稱以你自己的 ADR 為準 |
| 第 7 節：安全、合規、營運 | P08、P15、P16，以及 I-00.06、I-01.11、I-11.07 |
| 第 9 節：架構決策摘要 | 各階段 ADR；服務數量的重新評估在 I-17.14 的 ADR 0030 |
