# 運営・開発ガイド

このガイドは、運営者がローカルでシステムを起動して動作を確認するための最短手順です。

## 1. 必要なもの

- Docker Engine
- Docker Compose v2
- Git（ソースコードを取得する場合）

## 2. 起動する

リポジトリのルートで実行します。

```sh
cp .env.example .env
```

`.env` の `JWT_SECRET` と `BOOTSTRAP_ADMIN_PASSWORD` を変更してから、起動します。

```sh
docker compose up --build -d
docker compose logs -f api
```

アクセス先：

- Web 管理画面：<http://localhost:3000>
- API ヘルスチェック：<http://localhost:8080/healthz>

この Compose 構成はローカル確認用で、デモノードも起動します。インターネットへ公開する本番構成として、そのまま使用しないでください。

## 3. 公開フォームの Turnstile 設定

公開停止・削除、保全、再配布相談、配布元向け代行、移設先情報、ノード参加の各フォームには Cloudflare Turnstile を表示します。API は送信されたトークンを Cloudflare の Siteverify API で検証するため、フロント側だけのチェックではありません。

本番では Cloudflare ダッシュボードで Turnstile ウィジェットを作成し、公開するドメインを許可ホスト名に登録して、ルートの `.env` に次を設定してください。

```dotenv
APP_ENV=production
TURNSTILE_SITE_KEY=Cloudflareから取得したサイトキー
TURNSTILE_SECRET_KEY=Cloudflareから取得したシークレットキー
```

シークレットキーはブラウザーに配信されません。`APP_ENV=production` ではキーが未設定だと API は起動しません。ローカル開発ではキーを空欄にして認証を省略できます。

## 4. 初回ログイン

初回起動時に、`.env` の `BOOTSTRAP_ADMIN_EMAIL` と `BOOTSTRAP_ADMIN_PASSWORD` から管理者アカウントを作成します。初回ログイン後は管理画面から必要なユーザーを管理してください。

## 5. よく使うコマンド

```sh
# ログを見る
docker compose logs -f api

# 起動中のサービスを確認
docker compose ps

# 停止する
docker compose down

# 再ビルドして起動
docker compose up --build -d

# Go のテストを実行
go test ./...

# Web をビルド
cd web && npm install && npm run build
```

通常の `docker compose down` は名前付きボリュームを削除しません。`docker compose down -v` は DB や保存データを削除するため、必要性を確認せず実行しないでください。

## 6. 環境変数の使い分け

- ルートの `.env.example`：運営者・開発者向け。中央サービスとローカル動作確認に使います。
- `deploy/participant-node/.env.example`：ノード参加者向け。参加するノードだけに使います。

この2つを混ぜたり、運営者用 `.env` を参加者へ配布したりしないでください。ノード参加の手順は[参加ノードの始め方](PARTICIPATING_NODE_JA.md)を参照してください。

## 7. 詳細な開発手順

DB マイグレーション、API一覧、配置動作、テストの詳細は[開発詳細リファレンス](reference/DEVELOPMENT_DETAILS_JA.md)を参照してください。
