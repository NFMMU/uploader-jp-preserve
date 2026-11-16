# テストガイド

この文書では、Go API・バックエンド、Web フロントエンド、PostgreSQL を使う統合テストの実行方法と追加方針を定義します。

## テストの種類

| 対象 | 配置 | 実行方法 | 外部サービス |
| --- | --- | --- | --- |
| Go の単体テスト | 各 Go パッケージの `*_test.go` | `go test ./...` | 不要 |
| Go の DB 統合テスト | `internal/api/*_integration_test.go` | `make test-integration` | Docker、PostgreSQL |
| Go の配置・再試行統合テスト | `internal/api/*_integration_test.go` | `make test-integration` | Docker、PostgreSQL、Atlas migrations |
| Web のテスト | `web/**/*.test.mjs` 等 | `make test-web` | Node.js |
| Web の静的検査 | `web/src` 等 | `cd web && npm run lint:check` | Node.js |

Web は Node.js 組み込みの `node:test` と `node:assert` を使用します。現状のテスト規模では、Vitest/Jest 等の追加依存を導入するより、Node.js 標準機能で統一する方が軽量です。ブラウザー DOM や React コンポーネントの統合テストが必要になった時点で、対象に応じた環境を追加します。

## ローカルでの実行

### 基本テスト（DB不要）

リポジトリルートで実行します。

```sh
# Go
go test ./...

# Web
cd web
npm ci
npm test
npm run lint:check
npm run build
```

または `make test` で Go 単体テストと Web テストを実行できます。

### PostgreSQL 統合テスト

Docker Compose で専用の使い捨て DB を起動し、Atlas migrations を適用してから、DB 統合テストを実行します。

```sh
make test-integration
```

この処理は `scripts/test-integration.sh` が担当します。テスト用 DB は専用 Compose プロジェクトで起動し、終了時にコンテナを停止・削除します。既存の開発用 DB や名前付きボリュームは使用しません。

統合テストは `127.0.0.1:55432/archive_test` 以外の接続先を拒否します。テスト用 DB に実データや重要データを置かないでください。

### すべてのテスト

```sh
make test-all
```

これは Go 単体テスト、Web テスト、PostgreSQL 統合テストを実行します。Web の lint と build は CI で実行します。

## テストを追加する時のルール

### Go

- テスト対象と同じ package に `*_test.go` を追加します。
- 純粋関数・変換・バリデーションは DB を使わないテーブル駆動テストを優先します。
- HTTP handler は `httptest` を使い、成功・入力不正・権限不足・未存在・境界値を確認します。
- DB の永続化・トランザクション・ロック・検索条件は PostgreSQL 統合テストで確認します。SQLite に置き換えて PostgreSQL 固有の挙動を見落とさないようにします。
- 統合テストは合成データを使用し、テスト専用 DB 内で完結させます。
- 外部 API・ネットワークは `httptest` や注入可能な interface で置き換えます。
- 時刻・乱数・ID に依存するテストは固定値または注入可能な依存を使い、実行順に依存させません。

### Web

- テストファイル名は `*.test.mjs` または Node.js が標準検出できる形式にします。
- `node:test` の `test`、`beforeEach`、`afterEach` と `node:assert/strict` を使用します。
- 外部通信は `fetch` 等をスタブ化し、テスト後に必ず元へ戻します。
- 時刻・グローバル状態・モジュールキャッシュを変更する場合は、テスト間で状態を復元します。
- テストは独立して実行できるようにし、順序や共有状態に依存させません。
- UI のロジックは可能な範囲で純粋関数へ分離し、コンポーネントテストが必要な場合は別途 DOM 環境を導入します。

## CI

Pull Request と push で以下を実行します。

1. Go 単体テストと PostgreSQL 統合テスト（race detector、coverage を含む）
2. Web テスト全件
3. Biome による lint・format 検査（ファイルを書き換えない）
4. Web TypeScript/Vike build
5. Docker Compose 設定検証

CI のテストがスキップされたり、DB が準備されていない状態を成功扱いにしないことを原則とします。意図的にスキップするテストは、理由と実行条件を明記してください。
