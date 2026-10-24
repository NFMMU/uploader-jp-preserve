# uploader.jp保全プロジェクト

**運営：全国もちもちうどん連盟**  
**連絡先：** [uploaderjp-archive@udonya.moe](mailto:uploaderjp-archive@udonya.moe)  
**GitHub owner：** `nfmmu`

アップローダー上のファイルを、権利・利用条件を確認しながら複数ノードに保全するための試験運用プロジェクトです。保全と一般公開・再配布は別の判断として扱います。

> **注意：** 現在は MVP／試験運用版です。本番運用の安全性や可用性を保証するものではありません。保全する権限のあるコンテンツのみを扱ってください。

## まず読む

- [参加ノード運用ガイド](docs/PARTICIPATING_NODE_JA.md) — ストレージ提供者向け
- [URL自動収集ガイド](docs/URL_COLLECTION_JA.md) — 複数の公開サイトを限定巡回して uploader.jp リンクを収集し、重複URLを除外する仕組み
- 公開サイトの「ストレージノード参加」ページから申請し、運営承認後に一回限りの登録トークンを受け取ります。
- [GHCR コンテナイメージの利用方法](docs/CONTAINER_REGISTRY_JA.md) — イメージ公開・取得
- [ドキュメント一覧](docs/README.md)
- [GitHub 公開前チェックリスト](docs/PUBLICATION_CHECKLIST.md)
- [開発・ローカル起動ガイド](docs/DEVELOPMENT_JA.md) — Docker Compose、API、テスト、DB マイグレーション
- [セキュリティ方針](SECURITY.md)

## ローカル起動

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

## コンテナイメージ

GitHub Actions で API、Web、node-agent、node-manager のイメージを GHCR に公開する構成です。設定と公開方法は [GHCR ガイド](docs/CONTAINER_REGISTRY_JA.md) を参照してください。

## 管理機能の注意

- 「申請・保全レビュー」から関連ファイル・作品を直接開けます。
- ファイル一覧では複数選択、一括での公開条件変更・公開／非公開切替、メタデータ編集、状態・公開条件の絞り込みができます。
- 管理者は「URL自動収集」からDuckDuckGoで関連ページを自動検索し、検索結果の公開HTTPSページを限定巡回して uploader.jp / getuploader.com のダウンロードリンクを保全リクエストとして登録できます。検索語は任意で指定でき、空欄ならMMDモデル・モーション等を自動検索します。重複URLはレジストリと既存申請・ファイル一覧を照合して破棄します。収集だけでダウンロードや公開は開始せず、権利確認後にレビュー画面から取得を開始します。
- ファイル一覧で選択したファイルから削除申請を作成できます。申請・保全レビューで管理者が一括承認すると公開を停止し、保存ノードに削除タスクを登録します。全保存先から削除成功の報告が届くまでは「削除処理中」として管理し、完了後に通常のファイル一覧・公開カタログから非表示にします。オフラインのノードがある場合、完了まで時間がかかります。
- 保全状態（検証済み・劣化など）は実際の保存・検証処理に基づいて自動更新されます。手動で変更できるのは公開条件・公開設定などの管理情報です。

## 利用規約・プライバシー・免責事項

- [利用規約](docs/TERMS_JA.md)（公開サイト `/terms` にも掲載）
- [プライバシー・ポリシー](docs/PRIVACY_POLICY_JA.md)（公開サイト `/privacy` にも掲載）
- [免責事項](docs/DISCLAIMER_JA.md)（公開サイト `/disclaimer` にも掲載）

各ページは運用開始前のドラフトです。実際のインフラ構成、保存期間、問い合わせ・削除対応手順に合わせて確認し、必要に応じて法律専門家のレビューを受けてください。

## ライセンス

ソースコードと付属ドキュメントには [MIT License](LICENSE) を適用します。保全対象の第三者コンテンツ、アップロードされたファイル、商標、外部サイトの情報には、このソフトウェアライセンスは適用されず、それぞれの権利・利用条件が引き続き適用されます。
