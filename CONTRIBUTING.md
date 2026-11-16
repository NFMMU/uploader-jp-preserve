# コントリビューションガイド

運営元：全国もちもちうどん連盟  
連絡先：uploaderjp-archive@udonya.moe

## 変更を提案する前に

- 既存の Issue がないか確認し、大きな変更は先に提案内容を共有してください。
- 保全対象の著作物や個人情報、非公開 URL、認証情報を Issue や Pull Request に添付しないでください。
- uploader.jp 等のサイトに負荷をかけるテストを、運営の許可なく実サイトに対して実行しないでください。テストはモックやローカルの fixture を優先してください。
- 認証、認可、保全・公開ポリシー、データ削除に関する変更には、セキュリティ上の影響とテスト方法を説明してください。

## 開発時の確認

Go の変更：

```sh
gofmt -w <変更したGoファイル>
go test ./...
```

Web の変更：

```sh
cd web
npm ci
npm test
npm run lint:check:local
npm run format:check:local
npm run build
```

DB 統合テスト：

```sh
make test-integration
```

Compose 設定の変更：

```sh
cp .env.example .env
docker compose -f docker-compose.example.yml config --quiet
docker compose -f docker-compose.test.yml config --quiet
docker compose -f deploy/participant-node/compose.yaml config --quiet
```

統合テストは使い捨てテスト用データベースで実行してください。実データや参加者の保存ボリュームをテストに使わないでください。詳細は [テストガイド](docs/TESTING_JA.md) を参照してください。

## Pull Request に含める内容

- 変更の目的とユーザーへの影響
- 実行したテストと、その結果
- DB スキーマ変更がある場合は Atlas マイグレーションと生成コードの扱い
- 設定値・環境変数・運用手順・参加者向け文書の更新
- セキュリティ、著作権、プライバシー、外部サイトへのアクセス頻度への影響

ライセンスが確定するまでは、コントリビューションの受け入れ・再利用条件も運営側で明確にしてください。
