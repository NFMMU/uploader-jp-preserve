# uploader.jp保全プロジェクト

**運営：全国もちもちうどん連盟**  
**連絡先：** [uploaderjp-archive@udonya.moe](mailto:uploaderjp-archive@udonya.moe)

アップローダー上のファイルを、権利・利用条件を確認しながら複数ノードに保全するための試験運用プロジェクトです。

## まず読む

- [参加ノード運用ガイド](docs/PARTICIPATING_NODE_JA.md) — ストレージ提供者向け
- [GHCR コンテナイメージの利用方法](docs/CONTAINER_REGISTRY_JA.md) — イメージ公開・取得
- [ドキュメント一覧](docs/README.md)
- [GitHub 公開前チェックリスト](docs/PUBLICATION_CHECKLIST.md)
- [開発・ローカル起動ガイド](docs/DEVELOPMENT_JA.md) — Docker Compose、API、テスト、DB マイグレーション
- [セキュリティ方針](SECURITY.md)

## 開発者向けローカルデモ

Docker と Docker Compose を用意し、リポジトリのルートで実行します。

```sh
cp .env.example .env
docker compose up --build
```

API のヘルスチェック： <http://localhost:8080/healthz>

## リポジトリ構成

- `cmd/`, `internal/` — Go API、agent、Node Manager
- `web/` — 一般向けサイトと管理画面
- `atlas/` — DB スキーマと SQL マイグレーション
- `deploy/` — 参加ノード・Node Manager の起動設定
- `docs/` — 運用・開発ドキュメント
- `.github/workflows/` — CI と GHCR イメージ公開

## 利用規約・プライバシー・免責事項

- [利用規約](docs/TERMS_JA.md)（公開サイト `/terms` にも掲載）
- [プライバシー・ポリシー](docs/PRIVACY_POLICY_JA.md)（公開サイト `/privacy` にも掲載）
- [免責事項](docs/DISCLAIMER_JA.md)（公開サイト `/disclaimer` にも掲載）

## ライセンス

ソースコードと付属ドキュメントには [MIT License](LICENSE) を適用します。保全対象の第三者コンテンツ、アップロードされたファイル、商標、外部サイトの情報には、このソフトウェアライセンスは適用されず、それぞれの権利・利用条件が引き続き適用されます。
