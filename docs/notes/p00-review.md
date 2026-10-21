# P00 Foundation 階段檢核

## 架構圖

- [x] 更新 [docs/architecture.md](../architecture.md)：一個 api、一個 worker；`merchant` 與 `payment` schema 都還沒有表；observability profile 是 collector、Tempo、Prometheus、Grafana
- [x] 資料表關係：本階段沒有表，所以沒有關係圖

## 面試題（用自己的話回答，不看筆記）

1. 為什麼付款系統一開始選 Modular Monolith，而不是微服務？
   付款的不變量常常要在同一個交易裡看完。邊界還沒被真實流量磨過之前就拆成服務，換來的是網路失敗與分散式交易，不是更清楚的模組。先放在一個行程裡，用套件邊界把 merchant、payment 這些模組分開；P17 再把已經守住邊界的幾塊抽出去。

2. 你怎麼防止模組之間的依賴在幾個月後失控？
   別人的實作放在 `internal/`，編譯器就不讓你 import。公開套件之間誰可以呼叫誰，寫在 `.go-arch-lint.yml` 的 `mayDependOn`；沒寫上去的邊，`make lint` 失敗，CI 擋 PR。要新開一條同步呼叫，得在同一個變更裡改這張允許清單。

3. 為什麼整合測試要用真的 Postgres？
   付款要證明的是 commit 之後、另一個連線看不看得到，還有 migration 套上去的 schema 是不是那一份。mock 與「包一層 transaction 再 rollback」都看不到這件事。每個整合測試複製一份已經跑過 Atlas 的 template database，用的是 `postgres:18.6`，不是 compose 裡那顆共用的 `payment_dev`。

4. 你如何避免卡號或 token 出現在日誌裡？
   寫出日誌時依欄位名遮罩，不靠呼叫端記得處理。名單含 `card_number`、`authorization`、`pan`、`cvv`、`cvc`、`password`、`secret`、`token`、`api_key`，群組與 map 裡的鍵也算。設定裡的 `database_url` 另外標成 secret，印出來是 `***`。訊息本文不掃描，所以不能把卡號寫進 message，也不能把整包 HTTP body 丟進日誌。

## 故障場景

| 場景 | 注入方式 | 預期 | 實際 | 證據路徑 |
|---|---|---|---|---|
| 本階段無 |  |  |  |  |

## 還不懂的清單

- `govulncheck` 預設只對呼叫鏈上的符號失敗。被 import、但還沒呼叫到的漏洞目前不會擋 PR。等業務程式開始用到那些套件，這條界線還夠不夠，要再看一次。
- GitGuardian 的 Skip 存在他們的服務裡。同一段字若只改了空白，下一個分支會不會被當成新 incident，要等按完 Skip 之後才知道。

## 這個階段的迭代是否太大或太小

十次剛好：每次只加一項能獨立驗證的基礎（邊界、資料庫、migration、設定、日誌、HTTP、整合測試、trace、CI）。選做的 I-00.09a 沒有塞進來，也沒有擋 tag。CI 放在最後，所以更早的 PR 還沒有這組檢查；從下一個階段開始才每張 PR 都跑。
