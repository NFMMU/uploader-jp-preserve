# 参加ノード運用ガイド（試験運用版）

運営元：全国もちもちうどん連盟  
連絡先：uploaderjp-archive@udonya.moe

この文書は、`uploader.jp保全プロジェクト` にストレージを提供する参加者向けの手順です。

## 参加する前に確認すること

参加ノードは、運営側から割り当てられた保全タスクを実行し、ファイルをローカルの永続ストレージに保存します。
ノードのストレージはインターネットへ直接公開しませんが、保全対象には第三者の著作物などが含まれる可能性があります。
参加前に、公開されている運営の保全方針、削除・公開停止への対応、保存期間、問い合わせ手順を[README](../README.md)から確認してください。

参加者は次の条件を理解したうえで参加してください。

- 使用するディスク容量と、ノードを稼働させる期間を自分で判断できること。
- ノードは運営の API に接続し、タスクを受け取り、保存状況と容量情報を報告すること。
- `source_fetch` タスクを割り当てられた場合、agent が uploader.jp／getuploader.com へアクセスすること。既定では、その agent から送る各 HTTP リクエストの開始間隔を最低 30 秒空けます。
- 参加ノードの保存領域を他のアプリケーションや一般ユーザー向けの共有フォルダーとして使わないこと。

## 1. 運営から受け取る情報

運営に次の情報を確認してください。

- `NODE_ENROLLMENT_TOKEN`（登録用の秘密情報）。公開 Issue、チャットログ、スクリーンショットなどに貼り付けないでください。
- `NODE_TYPE` に指定する値（`A` または `B`）。運営の指示に従ってください。
- 申告容量、アクセス間隔、障害時の連絡先・対応方針。
- 適用される保全方針、コンテンツの扱い、離脱手順。

登録用トークンを持っているだけで、参加条件や保存対象への同意が完了するわけではありません。

## 2. 必要な環境

- Docker Engine と Docker Compose v2 が動作するサーバー。
- 継続利用できるディスク領域。Docker の名前付きボリュームを使います。
- 中央 API への**送信方向**の HTTPS 接続。agent のポートをインターネットや LAN に公開する必要はありません。
- DNS と TLS 証明書検証が正常に動作すること。

この構成は、ホストの Docker ソケットを agent コンテナに渡しません。`deploy/node-manager` にある Node Manager は別の試作機能であり、Docker ソケットへの管理権限を必要とするため、この参加手順では使用しません。

## 3. 設定と起動

リポジトリを取得し、リポジトリのルートディレクトリで作業します。

```sh
# GitHub owner は nfmmu、リポジトリ名は uploader-archive です。
git clone https://github.com/nfmmu/uploader-archive.git
cd uploader-archive
cp deploy/participant-node/.env.example deploy/participant-node/.env
```

`deploy/participant-node/.env` を編集し、運営から受け取った値を設定します。`CENTRAL_API_URL` は実際の HTTPS URL に置き換え、`NODE_ENROLLMENT_TOKEN` は運営から受け取った値にしてください。`NODE_NAME` は運営と相談して一意の名前にします。

```sh
docker compose \
  --env-file deploy/participant-node/.env \
  -f deploy/participant-node/compose.yaml \
  up -d --build
```

このコマンドは agent イメージをローカルでビルドし、agent を起動します。初回は Go の依存関係を取得するため、ビルドに時間がかかることがあります。

## 4. 状態確認と日常操作

ログを確認します。

```sh
docker compose \
  --env-file deploy/participant-node/.env \
  -f deploy/participant-node/compose.yaml \
  logs --tail=100 node-agent
```

agent の状態を継続して見る場合は `logs -f node-agent` を使います。登録が成功すると、起動ログにノード ID と agent の状態が記録されます。トークン値や `node-credentials.json` の内容は公開しないでください。

再起動：

```sh
docker compose --env-file deploy/participant-node/.env -f deploy/participant-node/compose.yaml restart node-agent
```

更新：リポジトリを更新してから、同じ Compose コマンドに `--build --force-recreate` を付けて実行します。

```sh
docker compose \
  --env-file deploy/participant-node/.env \
  -f deploy/participant-node/compose.yaml \
  up -d --build --force-recreate node-agent
```

通常の停止：

```sh
docker compose --env-file deploy/participant-node/.env -f deploy/participant-node/compose.yaml stop node-agent
```

停止すると中央 API へのハートビートが止まり、ノードはオフラインとして扱われる場合があります。長時間停止や離脱の前には運営へ連絡してください。

## 5. 容量とアクセス間隔

`.env` の主要設定：

| 変数                         | 意味                                                               | 既定値                               |
| ---------------------------- | ------------------------------------------------------------------ | ------------------------------------ |
| `NODE_NAME`                  | 運営側に表示されるノード名                                         | サンプル値                           |
| `NODE_TYPE`                  | ノード種別。`A` または `B`                                         | `B`                                  |
| `NODE_CAPACITY_BYTES`        | 参加者が提供する容量上限（バイト）                                 | `10737418240`（10 GiB）              |
| `NODE_UPLOADER_MIN_INTERVAL` | この agent から uploader.jp へ送る各 HTTP リクエストの最低開始間隔 | `30s`                                |
| `NODE_DATA_VOLUME`           | 保存データと agent 認証情報を保持する Docker ボリューム名          | `uploader-jp-preservation-node-data` |

`NODE_CAPACITY_BYTES=0` は参加者側で申告容量の上限を設けない指定です。ディスク容量や運営側の割り当て条件が無制限になるわけではありません。申告容量は実際に使える空き容量より大きくしないでください。

`NODE_UPLOADER_MIN_INTERVAL` は**各 agent 単位**の制限で、中央 API が全参加者を一律に待たせるものではありません。複数の agent が同時に動けば、サイト側から見た合計アクセス数は増える可能性があります。運営の指定がない限り、既定値 `30s` を維持するか、より長い間隔を設定してください。値を `0s` にすると間隔制限が無効になります。通常は設定しないでください。

## 6. データの保持と離脱

保存データとノードの認証情報は、`participant_node_data` に対応する名前付き Docker ボリュームに保存されます。コンテナを作り直しても通常はボリュームが維持されます。

- **`docker compose down -v` を実行しないでください。** 保存データと認証情報を含むボリュームを削除するおそれがあります。
- `NODE_DATA_VOLUME` を変更すると別のボリュームを使い始め、既存データが見えなくなることがあります。変更前に運営へ相談してください。
- ディスク故障、容量逼迫、ホスト移行、長期停止、参加終了を予定している場合は、できるだけ事前に運営へ連絡してください。
- 離脱時は、運営から保全コピーの状況と停止許可を確認してから agent を停止してください。運営の確認前にボリュームや保存ファイルを手動削除しないでください。

## 7. トラブルシューティング

### `CENTRAL_API_URL` や登録エラー

- 運営から指定された URL であること、ホストから HTTPS 接続できることを確認します。
- 登録トークンの余分な空白やコピー漏れを確認します。トークンを Issue や公開ログに貼り付けないでください。

### 容量不足・保存失敗

- ホストの空き容量と `NODE_CAPACITY_BYTES` を確認します。
- データ用ボリュームを削除して直そうとせず、ログを保存して運営へ連絡してください。
- 保存の失敗ログについては、こちらでも確認していますが、ログをいただくことにより継続的な改善に活かすことができます。

### uploader.jp の取得失敗

- `source_fetch_failed` は、対象ページの構造、追加確認、アクセス制限などにより自動取得できなかったことを示す場合があります。
- agent は JavaScript を実行せず、パスワード・認証・CAPTCHA を回避しません。失敗を繰り返すのではなく、運営が対象を確認して対応します。
- ログを共有する場合は、URL に秘密情報が含まれていないか確認してください。Cookie、トークン、認証ヘッダー、個人情報を送らないでください。

## 8. 問い合わせ

参加登録、容量変更、障害、セキュリティ上の懸念、離脱の相談：

**全国もちもちうどん連盟**  
**uploaderjp-archive@udonya.moe**
