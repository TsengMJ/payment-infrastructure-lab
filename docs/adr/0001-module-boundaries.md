# 0001 模組邊界由 Go internal 加 go-arch-lint 允許清單強制

- 狀態：Accepted
- 日期：2026-10-21
- 相關迭代：I-00.02

## 背景

P00–P16 是單一 Go module 的 Modular Monolith，P17 要把 webhook、connector、ledger、settlement、payout 抽成服務（[02-architecture-evolution.md](../../roadmap/practice-plan/02-architecture-evolution.md) 第 3 節）。抽得出來的前提是：模組之間只透過公開套件互相呼叫，而且依賴圖是刻意開放的，不是累積出來的。

需要擋下兩種違規：

1. 模組 A import 模組 B 的實作細節（`internal/ledger/internal/...`）。
2. 模組 A import 模組 B 的公開套件，但這條依賴沒有被設計過（例如 merchant 依賴 ledger）。

## 選項

1. **只靠 code review 與命名約定。** 不需要工具；但個人專案沒有第二個審查者，違規只會在 P17 抽服務時才被發現。
2. **Go `internal/` 目錄規則。** 編譯器強制，零設定；但只能擋第 1 種，`internal/merchant` 和 `internal/ledger` 都在同一個 `internal/` 之下，第 2 種完全合法。
3. **golangci-lint depguard。** 以檔案 glob 加 import 允許/拒絕清單表達。要表達「15 個模組各自只能依賴 platform 和明確開放的模組」，每個模組都要一條規則並列出其他模組，新增模組時要改所有規則。
4. **go-arch-lint。** 以 component 和 `mayDependOn` 描述一張允許圖；沒列出的依賴一律拒絕，沒對應到 component 的套件也會失敗。

## 決定

選項 2 加選項 4：

- 每個業務模組是 `internal/<module>`，公開面是 `internal/<module>/api.go` 所在的套件；實作放 `internal/<module>/internal/...`，由編譯器保護。
- `.go-arch-lint.yml` 中每個模組是一個 component，預設只能依賴 `platform`；`platform` 不能依賴任何模組。
- go-arch-lint 由 `go.mod` 的 `tool` directive 釘住版本，`make lint` 以 `go tool go-arch-lint check` 執行。
- 開放一條跨模組依賴時，必須在同一個變更裡把該邊加進 `mayDependOn`，並在 PR 說明為什麼是同步呼叫而不是事件。
- `cmd/` 在 I-00.07 有程式後加入，是唯一可以依賴所有模組公開套件的 composition root。

## 後果

- 好處：依賴圖就是設定檔本身，P17 盤點依賴時可以直接讀，或用 `go-arch-lint graph` 產生圖。新增套件若沒有對應到 component，lint 會失敗，不會默默逃過檢查。
- 代價：多一個工具與一份設定；每次開放依賴都要改設定檔。go-arch-lint 帶進不少間接依賴到 `go.mod`（只在 `go tool` 時使用，不會編進應用程式）。
- 何時要重新評估：P17 改成 `go.work` 多 module 時，服務之間的邊界改由 module 與契約強制，重新決定 go-arch-lint 是否只保留在 `payment-platform` 內部使用。
