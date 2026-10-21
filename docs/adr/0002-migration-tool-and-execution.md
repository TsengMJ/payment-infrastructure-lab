# 0002 Migration 用 Atlas、獨立指令執行，down 只開放到 I-17.03

- 狀態：Accepted
- 日期：2026-10-21
- 相關迭代：I-00.04；收回 down 的迭代是 I-17.03

## 背景

P00–P16 是一個 Postgres、多個模組。每個模組要有自己的 schema（`merchant`、`payment`，之後還有 `ledger` 等），而且只有擁有者能寫它。[02-architecture-evolution.md](../../roadmap/practice-plan/02-architecture-evolution.md) 把這條訂成抽出服務的前提。P17 還會把「改 schema」和「換程式」拆開（expand / contract），因為新舊版本會同時在跑。

這次要決定三件事：用哪個 migration 工具、版本史怎麼切開、誰在什麼時候執行。現在還沒有業務表。

## 選項

1. **golang-migrate 或 goose，每個模組一個目錄。** Go 服務裡很常見，一份 up SQL 加一份 down SQL 就能跑。沒有 desired-state diff，也沒有在 down 之前對「會不會刪資料」做檢查。多個目錄若共用同一張版本表，版本號會撞；要自己把版本表隔開。
2. **Atlas，整個資料庫一條 migration 歷史。** 這是 Atlas 對「一個資料庫、多個 schema」的預設。diff、lint、drift、以及由目前狀態算出的 down plan 都在。代價是 `merchant` 和 `payment` 共用 `atlas_schema_revisions`，P17 要把 schema 交給別的服務時得先切開這條歷史。
3. **Atlas，每個模組一個目錄。** 同樣有 diff、checksum（`atlas.sum`）、以及 down 前的檢查。每個模組有自己的 SQL 目錄和自己的版本表 schema（`atlas_merchant`、`atlas_payment`），抽出服務時可以帶走自己的歷史。代價是 CLI 較重，而且一個資料庫裡會多出每個模組一個版本表 schema。

選項 1 的安裝數在 Go 服務裡比較多。選項 2 和 3 的差異不在「安不安全」，而在版本史能不能跟著模組走。付款 schema 要的是：變更從 desired state 算出來、檔案有 checksum、回滾前先檢查會不會刪資料。這是選 Atlas 的理由，不是「用的人最多」。

## 決定

採用選項 3。

- 工具釘在 `arigaio/atlas:1.3.0-alpine`。Makefile 依 [deploy/atlas/Dockerfile](../../deploy/atlas/Dockerfile) 建成本地 image `payment-lab-atlas:1.3.0`，不在機器上全域安裝。多裝的 `docker-cli` 只為了 `docker://postgres/18/dev`：官方 image 裡沒有 docker CLI，而這個 dev URL 會呼叫它來起一個拋棄式的 Postgres 18。
- Desired state 在 `schema/<module>.hcl`。改表的流程是改 hcl，再 `make migrate-diff`，不手改已經提交的 SQL。
- `make migrate-up` 是唯一的執行方式。應用程式啟動時不跑 migration。多個副本同時啟動會搶 advisory lock，也把 schema 變更和程式部署黏成一步，之後 expand / contract 拆不開。
- 每個 env 的 `schemas` 只包含自己的模組。database-scoped diff 會把 Postgres 自帶的 `public` 和其他模組的 schema 當成多餘物件而產生 `DROP SCHEMA`。`diff.skip.drop_schema` 把這種刪除擋在產生檔案之前。
- 第一次把某個目錄套到已經有其他模組 schema 的資料庫時，Atlas 會因為「資料庫不乾淨」而拒絕。`make migrate-up` 因此帶 `--allow-dirty`。版本表建立之後，checksum 與歷史不一致仍然會失敗。
- Down 不另外寫 `.down.sql`。`make migrate-down` 用 `atlas migrate down`，由目前狀態計算反向 plan，並保留預設的 destructive checks（不加 `--skip-checks`）。它只接受連線 host `127.0.0.1`。`--to-version 0` 不是一筆真實版本，所以改為撤銷該目錄裡已執行的全部檔案。
- 版本表放在 `atlas_<module>`，不放進業務 schema。業務 schema 裡目前沒有表。`make migrate-down` 會刪掉業務 schema；`atlas_<module>` 會留下，裡面的版本紀錄已經回到未執行。

## 後果

- 好處：每個模組的 schema 與 migration 歷史從第一天分開。本機可以把空 schema 建起來再撤掉，而且撤銷前 Atlas 會先算 plan。
- 代價：diff 與 down 需要 Docker socket 與一個拋棄式 Postgres 18。每個模組多一個 `atlas_<module>` schema。`--allow-dirty` 讓「資料庫裡還有別的模組」變成合法的起始狀態，不能把它理解成可以忽略 drift。
- Down 的期限：現在到 I-17.03 之前，`make migrate-down` 只對本機 `127.0.0.1` 開放。現在撤的是空 schema。I-17.03 起刪除這個 target，本 ADR 改為被後續決定取代。那時新舊版本會同時在跑，down 也無法把已刪的資金資料還原；schema 變更只向前，回滾走 expand / contract（產出是 `docs/runbooks/zero-downtime-migration.md`）。
- 何時要重新評估：I-17.03。若更早開始在 migration 裡放資金資料，down 就先停止，不等到那次迭代。
