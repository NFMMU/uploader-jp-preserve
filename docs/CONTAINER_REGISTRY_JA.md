# GitHub Container Registry（GHCR）運用ガイド

このリポジトリには、GitHub Actions でコンテナイメージをビルドし、GitHub Container Registry（`ghcr.io`）へ発行するワークフローがあります。

## 発行するイメージ

GitHub リポジトリの所有者名を小文字にした値を `<owner>` とすると、次のイメージを発行します。このプロジェクトの GitHub owner は `nfmmu` です。したがって、通常のイメージ名は `ghcr.io/nfmmu/uploader-jp-preservation-<コンポーネント>` になります。

- `ghcr.io/<owner>/uploader-jp-preservation-api`
- `ghcr.io/<owner>/uploader-jp-preservation-web`
- `ghcr.io/<owner>/uploader-jp-preservation-node-agent`
- `ghcr.io/<owner>/uploader-jp-preservation-node-manager`

各イメージは `linux/amd64` と `linux/arm64` 向けにビルドします。

## いつ発行されるか

`.github/workflows/publish-ghcr.yml` は次のタイミングで実行されます。

- `main` ブランチへの push：ビルドして `main` と `latest` などのタグを発行
- `v1.2.3` のような `v*.*.*` タグへの push：バージョンタグと SHA タグを発行
- Pull Request：イメージをビルドして検証しますが、GHCR へ push しません
- `workflow_dispatch`：手動実行

GitHub Actions の `GITHUB_TOKEN` に `packages: write` を許可しています。通常、別途 PAT をリポジトリシークレットへ登録する必要はありません。

## 初回セットアップ

1. GitHub にリポジトリを push します。
2. GitHub の **Actions** タブで `Publish container images to GHCR` の結果を確認します。
3. リポジトリの所有者アカウント／Organization の **Packages** で生成されたパッケージを確認します。
4. 公開配布する場合は、各パッケージの Package settings から可視性を `Public` に変更します。初期状態では非公開になる場合があるため、匿名で pull できることを確認してから参加者へ案内してください。

パッケージを Public にする前に、イメージに秘密情報、テスト用認証情報、非公開データが含まれていないことを確認してください。Dockerfile に `.env` を COPY しないでください。

## 参加ノードで GHCR イメージを使う

`deploy/participant-node/.env.example` を `.env` にコピーし、`GHCR_OWNER=nfmmu` を設定します（リポジトリを別の owner に移した場合は変更してください）。さらに、公開サイトから参加申請を行い、運営の承認後に受け取る API URL と申請ごとの一回限りノード登録トークンを設定してください。

```sh
cd deploy/participant-node
cp .env.example .env
# .env を編集する

docker compose --env-file .env -f compose.ghcr.yaml pull
docker compose --env-file .env -f compose.ghcr.yaml up -d
docker compose --env-file .env -f compose.ghcr.yaml logs -f
```

`latest` はデフォルトブランチの最新イメージです。再現性を重視する運用では、`IMAGE_TAG` にリリース番号（例：`1.2.3`）を指定して固定してください。非公開パッケージの場合は、ホスト上で GitHub Container Registry にログインする必要があります。読み取り専用の fine-grained / classic PAT を利用する場合は、必要最小限の `read:packages` 権限にしてください。トークンを `.env` や Git に保存しないでください。

## Node Manager を GHCR イメージで使う場合

`deploy/node-manager/.env.example` を `.env` にコピーし、`GHCR_OWNER=nfmmu` と `IMAGE_TAG` を設定してから、次のように起動します。

```sh
cd deploy/node-manager
cp .env.example .env
# .env を編集する

docker compose --env-file .env -f compose.ghcr.yaml pull
docker compose --env-file .env -f compose.ghcr.yaml up -d
```

**重要：** Node Manager は Docker ソケットをマウントするため、ホスト上のコンテナを操作できる強い権限を持ちます。信頼できる管理者だけがアクセスできるホストで実行し、公開インターネットへ直接公開しないでください。

## ローカルビルドとの使い分け

従来の `docker-compose.yml` と `deploy/*/compose.yaml` はソースコードからビルドする開発用構成として残しています。公開済みのイメージを pull して使う場合は、このガイドで案内する `compose.ghcr.yaml` を使ってください。

## トラブルシューティング

- `denied` / `unauthorized`：パッケージの可視性、イメージ名、タグ、GHCR ログイン状態を確認してください。
- `manifest unknown`：Actions の publish ジョブが成功したか、タグが存在するか確認してください。
- 古いイメージが起動する：`docker compose ... pull` の後に `docker compose ... up -d --force-recreate` を実行してください。
- 公開 URL を変更した：イメージを作り直す必要がある設定か、環境変数だけで変更できる設定かを確認してください。
