# 参加ノードの始め方

この手順は、ストレージを提供する参加者向けです。MVP／試験運用版のため、参加前に運営の案内と保存・削除方針を確認してください。

問い合わせ：uploaderjp-archive@udonya.moe

## 参加の流れ

1. 公開サイトの「ストレージノード参加申請」から申し込む。
2. 運営の承認を待つ。
3. 運営から指定された中央 API の HTTPS URL と、一回限りの登録トークンを受け取る。
4. このページの手順でノードを起動する。
5. 登録成功後、運営へ状態を確認する。

登録トークンは発行後72時間で失効し、最初の登録成功後は再利用できません。パスワードと同じように扱い、Issue・公開チャット・スクリーンショットに貼らないでください。

## 必要なもの

- Docker Engine と Docker Compose v2 が動く Linux ホスト
- 継続して使えるディスク容量
- 中央 API への送信方向の HTTPS 接続

ノードの保存領域や agent のポートをインターネットに公開する必要はありません。

## 1. 設定する

リポジトリを取得し、ルートで実行します。

```sh
git clone https://github.com/nfmmu/uploader-jp-preserve.git
cd uploader-jp-preserve
cp deploy/participant-node/.env.example deploy/participant-node/.env
```

`deploy/participant-node/.env` を編集します。

| 設定                         | 入れる値                                            |
| ---------------------------- | --------------------------------------------------- |
| `CENTRAL_API_URL`            | 運営から指定された HTTPS URL                        |
| `NODE_ENROLLMENT_TOKEN`      | 承認後に受け取った一回限りのトークン                |
| `NODE_NAME`                  | 申請時に承認されたノード名                          |
| `NODE_TYPE`                  | 基本的に `B` でご登録ください。                     |
| `NODE_CAPACITY_BYTES`        | 提供する容量（バイト）                              |
| `NODE_UPLOADER_MIN_INTERVAL` | uploader.jp へのリクエスト間隔。通常は `30s` のまま |

`NODE_CAPACITY_BYTES=10737418240` は 10 GiB、`0` は参加者側で容量上限を設定しない指定です。`0` にしても、実際のディスク容量を超えて保存できるわけではありません。無理のない容量を設定してください。

## 2. 起動する

> [!CAUTION]
> データの保存先がOSのインストールディスクではない方については、`deploy/participant-node/compose.yaml`ファイルを編集してボリュームの設定を変更する必要があります。

```sh
docker compose \
  --env-file deploy/participant-node/.env \
  -f deploy/participant-node/compose.yaml \
  up -d
```

## 3. 状態を確認する

```sh
docker compose \
  --env-file deploy/participant-node/.env \
  -f deploy/participant-node/compose.yaml \
  logs --tail=100 node-agent
```

初回登録に成功したら、`.env` の `NODE_ENROLLMENT_TOKEN` を空にして構いません。ノードの認証情報は永続ボリュームに保存されます。トークンや `node-credentials.json` の内容を他人に送らないでください。

## 更新・停止

更新：

```sh
docker compose --env-file deploy/participant-node/.env \
  -f deploy/participant-node/compose.yaml up -d --build --force-recreate
```

停止：

```sh
docker compose --env-file deploy/participant-node/.env \
  -f deploy/participant-node/compose.yaml stop node-agent
```

長期停止、ディスク故障、容量変更、参加終了の前には運営へ連絡してください。

## 大切な注意

- **`docker compose down -v` を実行しないでください。** 保存データとノード認証情報が削除されるおそれがあります。
- `NODE_DATA_VOLUME` を変更すると、既存データが見えなくなることがあります。
- 運営の確認前に、保存ファイルやデータ用ボリュームを手動削除しないでください。
- agent はアクセス制限、ログイン、CAPTCHA などを回避しません。取得失敗が続く場合はIssueを立ててください。
- ログを共有する前に、URL内の秘密情報や個人情報が含まれていないか確認してください。

問い合わせ：**uploaderjp-archive@udonya.moe**
