# P01 商戶與 Gateway API（12 次迭代）

- **對應模組**：#01 Merchant Management（基本部分）、#03 Payment Gateway API
- **階段目標**：商戶可以用 API key 呼叫 `/v1`，建立訂單；API 有一致的驗證、隔離、錯誤格式、版本、分頁、限流與金額規則。
- **前置**：P00。
- **結束時的架構**：`merchant` 模組（`merchant` schema）；`internal/platform` 新增 auth、request_id、money、ratelimit；compose 新增 Redis。

這個階段還不碰付款。先把「誰在呼叫、能動哪些資料、金額怎麼表示」做對，後面所有模組都會依賴這些規則。

---

### I-01.01 Merchant 模型

- **目標**：建立 `merchant.merchants` 表與建立、查詢商戶的內部服務（尚不對外開放）。
- **為什麼**：每一筆付款、帳務、出款都必須能追溯到一個商戶。
- **不做**：KYB 審核（P08）、費率（P06）、後台登入（P15）。
- **先想清楚**：
  1. 商戶 ID 用 UUID、ULID 還是帶前綴的字串（例如 `mer_...`）？對外露出時有什麼差別？
  2. 商戶狀態有哪些（active、suspended、closed）？狀態會影響哪些操作？
  3. 商戶的預設幣別、允許的支付方式要放哪裡？
- **完成條件**：整合測試：建立商戶、依 ID 查詢、狀態欄位有 CHECK 約束。
- **常見陷阱**：用自增整數 ID 對外露出，讓別人可以猜到其他商戶。
- **Survey**：prefixed IDs Stripe、ULID、multi-tenant SaaS data model
- **產出**：migration、`internal/merchant`

### I-01.02 API key 產生與雜湊儲存

- **目標**：為商戶產生 API key，只存雜湊；原始 key 只在建立時顯示一次。
- **為什麼**：資料庫外洩時，攻擊者不能直接拿 key 呼叫付款 API。
- **不做**：驗證 middleware（下一次）、輪替（I-01.04）。
- **先想清楚**：
  1. key 格式：前綴（`sk_test_`）＋隨機部分？前綴的用途是什麼？
  2. 用 SHA-256 還是 bcrypt / argon2？API key 和密碼的威脅模型有什麼不同？
  3. 怎麼在不存原始值的情況下，讓商戶辨認是哪一把 key（例如存最後 4 碼）？
- **完成條件**：測試證明資料庫裡找不到原始 key；同一把 key 可以被驗證。
- **常見陷阱**：用 `math/rand` 產生 key；用一般字串比較驗證（時間差攻擊）。
- **Survey**：API key design、crypto/rand、constant time compare
- **產出**：`merchant.api_keys` 表、ADR 0003「API key 格式與雜湊」

### I-01.03 驗證 middleware 與商戶 context

- **目標**：`/v1/*` 的每個請求都必須帶有效 key；middleware 把商戶 ID 放進 request context。
- **為什麼**：之後所有業務邏輯都從 context 取得「目前是哪個商戶」，不能信任請求 body 裡的 merchant_id。
- **不做**：權限角色（P15）。
- **先想清楚**：
  1. key 放在 `Authorization: Bearer` 還是自訂 header？
  2. key 無效、被撤銷、商戶被停用，分別回什麼狀態碼與錯誤碼？
  3. 每次請求都查資料庫驗證 key，會不會太慢？要不要快取？快取帶來什麼風險？
- **完成條件**：測試：無 key 401、錯 key 401、停用商戶 403、有效 key 能在 handler 裡取得商戶 ID。
- **常見陷阱**：錯誤訊息透露「這把 key 存在但被撤銷」，等於幫攻擊者確認 key。
- **設計模式**：Middleware chain、Cache-aside 與失效（撤銷延遲是安全取捨）
- **Survey**：HTTP authentication middleware Go、bearer token
- **產出**：`internal/platform/auth`

### I-01.04 test / live 模式與 key 輪替

- **目標**：商戶有 test 與 live 兩種 key；同一模式可同時有兩把有效 key 以便輪替，並可撤銷。
- **為什麼**：測試流量不能碰到真實資料；key 外洩時要能不停機換掉。
- **不做**：自動到期提醒。
- **先想清楚**：
  1. test 與 live 的資料要怎麼隔離：同一張表加 `mode` 欄位，還是更強的隔離？
  2. 輪替流程：建立新 key → 商戶切換 → 撤銷舊 key。中間的時間窗怎麼處理？
  3. 用 test key 存取 live 建立的資源應該回什麼？
- **完成條件**：測試：test key 看不到 live 資源；撤銷後的 key 立即失效；兩把有效 key 都能用。
- **常見陷阱**：模式只存在 key 上，資料表沒有模式欄位，查詢時忘記過濾。
- **Survey**：Stripe test mode vs live mode、API key rotation
- **產出**：migration、撤銷 API

### I-01.05 商戶範圍隔離

- **目標**：所有資料查詢都強制帶 `merchant_id`；跨商戶存取一律回 404。
- **為什麼**：多租戶付款平台最嚴重的事故之一，就是商戶 A 看到或操作了商戶 B 的付款。
- **不做**：Postgres Row Level Security（可在筆記裡比較，不必實作）。
- **先想清楚**：
  1. 怎麼讓「忘記帶 merchant_id」變成編譯錯誤或測試失敗，而不是靠記憶？
  2. 跨商戶存取回 403 還是 404？為什麼？
  3. Row Level Security 能幫你什麼？代價是什麼？
- **完成條件**：一個通用的跨租戶測試：用商戶 B 的 key 存取商戶 A 的每一個資源，全部 404。這個測試之後每加一個資源就擴充一次。
- **常見陷阱**：列表 API 有過濾，單筆查詢 API 卻忘了。
- **Survey**：multi-tenant isolation、Postgres row level security、IDOR
- **產出**：repository 介面規則、跨租戶測試輔助函式

### I-01.06 request_id 與關聯 ID

- **目標**：每個請求都有 request_id（接受外部傳入或自行產生），出現在回應 header、日誌、trace。
- **為什麼**：商戶客服回報問題時，只要給你 request_id 就能查出完整路徑。
- **不做**：跨服務傳遞（P17）。
- **先想清楚**：
  1. request_id 與 trace_id 有什麼不同？兩者都要嗎？
  2. 可以信任商戶傳進來的 request_id 嗎？長度與格式要不要限制？
  3. 之後非同步處理（worker）時，要怎麼保留這個關聯？
- **完成條件**：測試：回應 header 有 `Request-Id`；日誌與 trace 裡有相同值。
- **常見陷阱**：只在 handler 開始時記錄，錯誤路徑的日誌卻沒有帶。
- **Survey**：correlation ID、W3C trace context
- **產出**：`internal/platform/requestid`

### I-01.07 錯誤格式與錯誤碼目錄

- **目標**：所有錯誤回應使用同一格式，並有一份錯誤碼目錄文件。
- **為什麼**：商戶要能用程式判斷「可以重試」或「不要重試」，付款 API 的錯誤是給程式讀的。
- **不做**：PSP 拒絕原因的對應（P02）。
- **先想清楚**：
  1. 欄位要有哪些：type、code、message、param、request_id？
  2. 哪些錯誤可以讓商戶重試？要不要在回應裡明說？
  3. 內部錯誤要露出多少資訊？
- **完成條件**：驗證錯誤、認證錯誤、找不到資源、內部錯誤四種情況都有測試，格式一致；`docs/api/errors.md` 列出所有錯誤碼。
- **常見陷阱**：把 Go 的 error 字串直接回給商戶，洩漏 SQL 或內部路徑。
- **Survey**：Stripe API errors、RFC 9457 problem details
- **產出**：`internal/platform/apierror`、錯誤碼文件

### I-01.08 OpenAPI 規格、版本與分頁

- **目標**：有 OpenAPI 規格檔；路徑以 `/v1` 為版本；列表 API 用 cursor 分頁。
- **為什麼**：商戶整合靠 API 契約；契約一旦公開就很難改。
- **不做**：SDK 產生。
- **先想清楚**：
  1. 規格先行（先寫 OpenAPI 再產生程式）還是程式先行？
  2. 什麼樣的改動算 breaking change？新增欄位算嗎？新增 enum 值呢？
  3. 為什麼 cursor 分頁比 offset 分頁適合交易資料？
- **完成條件**：規格檔通過 lint；商戶列表 API 的分頁測試：新增資料不會造成重複或遺漏。
- **常見陷阱**：用 offset 分頁，資料一直新增時翻頁會重複。
- **Survey**：OpenAPI 3.1、oapi-codegen、cursor pagination、API versioning Stripe
- **產出**：`api/openapi.yaml`、ADR 0004「API 版本策略」

### I-01.09 Money 型別：minor unit 與幣別小數位

- **目標**：建立 Money 型別，金額以整數 minor unit 儲存，並依 ISO 4217 處理小數位。
- **為什麼**：浮點數會讓錢算錯；不同幣別的最小單位不同，JPY 沒有小數，KWD 有 3 位。
- **不做**：匯率換算（P14）。
- **先想清楚**：
  1. 為什麼不用 float64？不用 decimal 型別又有什麼好處和限制？
  2. 不同幣別的 Money 能不能相加？型別上怎麼擋？
  3. API 接收金額時，商戶傳的是 minor unit 還是十進位字串？
- **完成條件**：測試：USD 1000 表示 $10.00；JPY 1000 表示 ¥1000；不同幣別相加回錯誤；負數與溢位有明確行為。
- **常見陷阱**：所有幣別都假設兩位小數。
- **設計模式**：Value object
- **Survey**：ISO 4217 minor unit、Stripe zero-decimal currencies、money pattern
- **產出**：`internal/platform/money`

### I-01.10 Order 與後端定價

- **目標**：商戶可以 `POST /v1/orders` 建立訂單；金額由後端依品項計算，而不是信任前端。
- **為什麼**：前端可以被竄改；應付金額必須是後端算出來的。
- **不做**：付款（P02）。
- **先想清楚**：
  1. 在付款平台裡，Order 是平台的概念還是商戶的概念？為什麼 ProjectRoadmap 仍要你建它？
  2. 訂單金額建立後可以修改嗎？已經有付款時呢？
  3. 訂單需要哪些狀態？
- **完成條件**：測試：請求帶入假的總額會被忽略或拒絕；訂單金額等於後端計算結果。
- **常見陷阱**：讓訂單在已有付款嘗試後還能改金額。
- **Survey**：server-side price calculation、order vs payment intent
- **產出**：`payment.orders` 表（或獨立的 order 模組，寫 ADR 說明歸屬）

### I-01.11 每個商戶的 rate limit

- **目標**：用 Redis 對每個商戶限流，超過時回 429 與 `Retry-After`。
- **為什麼**：一個商戶的程式錯誤（無限重試）不應拖垮整個平台，也不應把 PSP 的配額用光。
- **不做**：edge gateway 層的限流（P17 選做）。
- **先想清楚**：
  1. fixed window、sliding window、token bucket、GCRA 的差別？
  2. Redis 掛掉時要放行還是拒絕？付款 API 和查詢 API 的答案一樣嗎？
  3. 限流的 key 用商戶、API key 還是 IP？
- **完成條件**：測試：超過限額時回 429；Redis 停掉時的行為符合你在 ADR 裡的決定。
- **常見陷阱**：用 INCR 加 EXPIRE 但兩步不是原子的。
- **設計模式**：Rate limiting（入站）、fail open 與 fail closed 的取捨
- **Survey**：GCRA rate limiting、redis rate limit Lua、Stripe rate limits
- **產出**：`internal/platform/ratelimit`、compose 加 Redis、ADR 0005「限流在 Redis 故障時的行為」

### I-01.12 端到端 API 測試（含階段檢核）

- **目標**：一個端到端測試：建立商戶與 key → 建立訂單 → 查詢訂單 → 跨商戶查詢失敗。
- **為什麼**：把這個階段的所有規則串起來驗證一次，並作為之後的回歸測試。
- **不做**：新功能。
- **先想清楚**：
  1. 端到端測試要透過真的 HTTP server 還是直接呼叫 handler？
  2. 測試資料怎麼建立：API 還是直接寫資料庫？
- **完成條件**：`make test-e2e` 通過，並在 CI 執行。
- **常見陷阱**：端到端測試依賴執行順序。
- **Survey**：Go httptest、API end-to-end test
- **產出**：`tests/e2e/`、`docs/notes/p01-review.md`

## 階段檢核

- **面試題**：
  1. 你的 API key 怎麼儲存？資料庫外洩時會發生什麼？
  2. 怎麼保證商戶 A 不能存取商戶 B 的付款？
  3. 為什麼金額要用整數 minor unit？JPY 呢？
  4. Redis 掛掉時，你的限流怎麼處理？為什麼？
  5. 為什麼不能信任前端傳來的金額？
- **故障場景**：跨商戶存取、Redis 停止時的限流行為。
- **tag**：`p01-merchant-gateway-done`
