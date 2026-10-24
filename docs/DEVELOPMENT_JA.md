# uploader.jp保全プロジェクト

**運営元：全国もちもちうどん連盟**  
**連絡先：uploaderjp-archive@udonya.moe**

Go 製 API、PostgreSQL の目録データベース、Go 製ノードエージェント、React 製 Web 画面で構成される分散保全プロジェクトです。現在は MVP／試験運用版であり、無人の本番運用に必要な機能・監査・復旧手順がすべて完成していることを保証するものではありません。

## はじめに

- ローカルで試す場合は、以下の「Docker Compose でのローカル開発」を参照してください。
- ストレージを提供する参加者は、先に [参加ノード運用ガイド](PARTICIPATING_NODE_JA.md) を読んでください。公開申請の承認後、申請単位の一回限りの登録トークンが発行されます。
- ドキュメントの一覧は [ドキュメント一覧](README.md)、GitHub 公開前の確認事項は [公開前チェックリスト](PUBLICATION_CHECKLIST.md) を参照してください。
- 不具合や運用の問い合わせ：uploaderjp-archive@udonya.moe。脆弱性の報告は [SECURITY.md](../SECURITY.md) を参照してください。

> **公開・運用上の注意：** 保全する権限のあるコンテンツだけを取り込んでください。保全と再配布の許可は別です。ソースコードと付属ドキュメントには MIT License を適用します。保全対象の第三者コンテンツには適用されません。利用規約・プライバシー・ポリシー・免責事項は運用環境に合わせて確認してください。

ローカルの Docker Compose では、障害復旧テスト用の予備保存先を確保するため、A ノード 1 台と B ノード 2 台の計 3 台を独立したデモノードとして起動します。本番環境では、各エージェントを別々のホストとストレージボリュームで実行してください。

## リポジトリ構成

- `cmd/` — API、ノード agent、Node Manager、コード生成コマンド
- `internal/` — API ハンドラー、設定、セキュリティ、生成済み DB コード
- `web/` — React + Vite の一般向けサイトと管理画面
- `atlas/` — DB スキーマとバージョン管理された SQL マイグレーション
- `deploy/participant-node/` — ストレージ参加者向けの独立した Compose 設定
- `deploy/node-manager/` — 試作版 Node Manager のローカル起動設定と GHCR 起動設定
- `docs/` — 運用手順、参加者向けガイド、設計資料
- `.github/` — CI、GHCR イメージ発行、Issue テンプレート、Pull Request テンプレート


## MVP-3 の実装範囲

- Go 製ノードエージェントは、運営が承認した申請ごとの一回限り登録トークンを使って中央 API に登録し、ノードごとの Bearer トークンを権限 `0600` の永続ファイルに保存します。ローカル Compose のみ `DEV_NODE_ENROLLMENT_TOKEN` を使用します。
- エージェントは、実際のファイルシステム容量・空き容量、管理対象オブジェクトの使用量、参加者が申告した容量上限、最終応答時刻を報告します。
- A/B ノードは個別に登録し、実容量、空き容量、使用量、申告容量、ハートビート状態を報告します。
- 管理者がファイルを中央の一時ステージング領域にアップロードすると、API は SHA-256 とサイズを計算し、条件を満たすオンラインノード 2 台にそれぞれ独立した保存タスクを作成します。
- 各ノードは認証済み API からファイルを取得して自身の管理ストレージに書き込み、ディスク上のデータを再ハッシュして結果を報告します。
- uploader.jp / getuploader.com の元ファイル取得は `source_fetch` タスクとしてノード agent に割り当てます。uploader.jp への各取得リクエスト間隔は agent ごとの `NODE_UPLOADER_MIN_INTERVAL` で制御し、中央 API のグローバルロックは使いません。
- オンラインノード上で検証済みの配置が 2 つ以上ある場合に限り、ファイルは `verified` になります。健全なコピーが 1 つだけなら `degraded` となり、タスクは最大 5 回まで再試行されます。
- 検証済みファイルに追加コピーが必要な場合、API は検証済みのコピーを持つノードに `replicate` タスクを登録します。そのエージェントはローカルのデータを検証して中央ステージング領域にアップロードし、複製先ノードが通常の保存処理で保存・検証します。
- バックグラウンドの調整処理は、応答が途絶えたノードをオフラインにし、そのノード上の未検証配置を `missing` に更新し、ファイルの保全状態を更新します。健全なコピー元と条件を満たす保存先があれば、代替コピーの作成も予約します。ノードが一時的にオフラインになっただけの場合、過去の検証済み配置の記録は保持し、ノード復帰まではそのコピーを利用不可として扱います。
- 必要な健全コピー数は 2、追加の予備コピーは 1（最大 3 コピー）です。オンラインの検証済み配置が 3 つを超えた場合、同じファイルに対するタスクが実行中でなく、ファイルを持つすべてのオンラインノードが 5 分以上継続してオンラインであり、最も新しい検証済み配置も検証から 5 分以上経過している場合に限り、余剰コピーの削除を 1 件ずつ予約します。削除はノードエージェントのタスクとして実行し、エージェントから成功報告が届いた場合のみ配置を `deleted` にします。削除に失敗した場合や結果が不確かな場合、その配置は健全なコピーとして数えません。
- 管理者は配置を確認し、複製を要求し、調整処理を手動実行できます。タスクと配置に関する操作は監査ログに記録されます。

これは MVP-3 の実装範囲であり、本番運用の準備がすべて完了したことを意味しません。中央ステージング領域は転送用の一時領域であり、恒久的な 3 つ目のコピーではありません。動作確認には、テスト用または保全する権限のあるコンテンツだけを使用してください。

## スキーマとコード生成の手順

1. スキーマの正本である `atlas/schema.hcl` を編集します。
2. `atlas migrate diff <name> --env local` でバージョン付きマイグレーションを生成します。
3. 生成された SQL マイグレーションと `atlas.sum` を確認してコミットします。
4. `atlas migrate apply --env local` でマイグレーションを適用します。
5. `make generate` で Go モデルと型付きクエリコードを再生成します。

GORM の `AutoMigrate` は使用しないでください。また、生成済みのモデル／クエリファイルを手作業で編集しないでください。MVP-2 では、既存の生成済み構造体にまだ存在しないリース／認証情報の 2 カラムに SQL を使用しています。

## GHCR コンテナイメージ

GitHub Actions で API、Web、ノード agent、Node Manager のイメージをビルドし、`ghcr.io/<GitHub所有者>/uploader-jp-preservation-<コンポーネント>` に発行します。`main` への push では `latest`、`v1.2.3` 形式のタグではバージョン付きイメージが作られます。Pull Request ではビルド検証のみを行い、イメージは発行しません。

発行設定、パッケージの公開設定、参加ノードでの pull 方法は [GHCR 運用ガイド](CONTAINER_REGISTRY_JA.md) を参照してください。GitHub Packages の初期可視性は必ず確認し、公開パッケージに秘密情報が含まれていないことを確かめてください。

## Docker Compose でのローカル開発

必要なもの：Docker Desktop / Docker Compose。Go 1.27 と Atlas CLI は、スキーマ編集やコード生成をローカルで行う場合にのみ必要です。

```sh
cp .env.example .env
docker compose up --build
```

Compose は PostgreSQL、Atlas マイグレーション、API、Go 製ノードエージェント 3 台（`local-node-a`、`local-node-b`、`local-node-c`）を起動します。各エージェントは個別に登録され、それぞれの永続ボリュームに認証情報を保存します。初回起動時は、3 台すべての最初のハートビートが届くまで少し待ってください。

```sh
curl -i http://localhost:8080/healthz
docker compose ps
docker compose logs --tail=100 node-agent node-agent-b node-agent-c
```

API は `http://localhost:8080` で待ち受けます。ヘルスチェック用エンドポイントは `GET /healthz` です。

### 初回ログイン

`.env` に設定した `BOOTSTRAP_ADMIN_EMAIL` と `BOOTSTRAP_ADMIN_PASSWORD` を使用します。

```sh
curl -s http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.invalid","password":"replace-this-bootstrap-password"}'
```

最初の管理者は、ユーザーテーブルが空の場合に限り作成されます。初期パスワードは bcrypt でハッシュ化され、平文では保存されません。初期設定後は、デプロイ環境の設定から初期パスワードを削除してください。

## MVP-3 の一連の動作確認

ログインで取得したアクセストークンを設定します。認証情報や Bearer トークンをログやチャットに貼り付けないでください。

```sh
export TOKEN='your-admin-access-token'

curl -sS http://localhost:8080/api/v1/nodes \
  -H "Authorization: Bearer $TOKEN"

ARTIFACT=$(curl -sS -X POST http://localhost:8080/api/v1/artifacts \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"title":"MVP-3 test artifact","source_url":"https://www.uploader.jp/"}')
export ARTIFACT_ID=$(printf '%s' "$ARTIFACT" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')

FILE=$(curl -sS -X POST "http://localhost:8080/api/v1/artifacts/$ARTIFACT_ID/files" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"original_filename":"sample.txt","size_bytes":0,"media_type":"text/plain","redistribution_policy":"in_progress"}')
export FILE_ID=$(printf '%s' "$FILE" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')

printf 'uploader.jp archive MVP-3 test\n' > /tmp/archive-sample.txt
curl -i -X POST "http://localhost:8080/api/v1/files/$FILE_ID/ingest" \
  -H "Authorization: Bearer $TOKEN" \
  -F 'file=@/tmp/archive-sample.txt;type=text/plain' -F 'priority=3'

curl -sS 'http://localhost:8080/api/v1/tasks' \
  -H "Authorization: Bearer $TOKEN"
curl -sS "http://localhost:8080/api/v1/files/$FILE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

期待される結果：取り込みエンドポイントが `202 Accepted` と 2 件のタスク、2 台の保存先ノードを返します。両方のタスクが `completed` になり、配置エンドポイントの `healthy_replica_count` が `2`、ファイルの `preservation_status` が `verified` になります。再配布ポリシーが `in_progress`（確認中）のため、`public_download_enabled` は `false` のままです。なお、再配布ポリシーが `pending`（未確認）の場合は保全処理を開始できません。

```sh
curl -sS "http://localhost:8080/api/v1/files/$FILE_ID/placements" \
  -H "Authorization: Bearer $TOKEN"
```

既存ファイルのコピーを追加作成したい場合や配置修復を試したい場合は、`POST /api/v1/files/:id/replicate` を使用します。API は追加の健全なコピーが必要で、条件を満たす保存先ノードがある場合に限り、コピー元ノードからの転送を予約します。テスト中に保全状態・配置をすぐに再調整するには、`POST /api/v1/admin/placements/reconcile` を呼び出します。

ノードの状態や失敗したタスクを確認するには、次を実行します。

```sh
curl -sS http://localhost:8080/api/v1/nodes -H "Authorization: Bearer $TOKEN"
curl -sS 'http://localhost:8080/api/v1/tasks?status=failed' -H "Authorization: Bearer $TOKEN"
docker compose logs --tail=200 node-agent node-agent-b node-agent-c api migrate
```

## MVP-1 の API 一覧

- `POST /api/v1/auth/login`、`GET /api/v1/me`
- `GET /api/v1/artifacts?q=...`、`POST /api/v1/artifacts`、`GET /api/v1/artifacts/:id`
- `POST /api/v1/artifacts/:id/files`、`GET /api/v1/files`、`GET /api/v1/files/:id`
- `PATCH /api/v1/files/:id/policy`、`PATCH /api/v1/files/:id/publication`
- `GET /api/v1/export?format=json|csv`
- `POST /api/v1/public-requests`、`GET /api/v1/public-requests`、`PATCH /api/v1/public-requests/:id`
- `POST /api/v1/public/node-enrollment-requests` — 公開ストレージノード参加申請。IP ごとに 1 時間 5 件まで。
- `GET /api/v1/admin/node-enrollment-requests`、`PATCH /api/v1/admin/node-enrollment-requests/:id` — 管理者が参加申請を審査し、承認時に申請ごとの一回限り・72 時間有効な登録トークンを発行します。
- `POST /api/v1/admin/users`、`GET /api/v1/admin/users`、`GET /api/v1/admin/audit-logs`

## MVP-2 / MVP-3 の API 一覧

- `POST /api/v1/nodes/register` — `X-Node-Enrollment-Token` を使った一度限りのノード登録。承認申請のノード名・種別・容量と一致する必要があります。`DEV_NODE_ENROLLMENT_TOKEN` による共有トークン登録はローカル開発環境だけで有効です。
- `POST /api/v1/nodes/:id/heartbeat` — ノードトークン認証付きの容量報告
- `GET /api/v1/nodes` — 管理者／レビュアー向けノード一覧
- `GET /api/v1/tasks?status=...` — 管理者／レビュアー向けタスク一覧
- `POST /api/v1/tasks/:id/retry` — 管理者専用。失敗した保存タスク／元ファイル取得タスクを再試行します。保存タスクのステージング元データが欠落・破損している場合は、multipart の `file` で代替データを送信できます。
- `POST /api/v1/files/:id/ingest` — 管理者専用の multipart アップロード。条件を満たす別々のノード 2 台への保存を予約します。
- `POST /api/v1/files/:id/ingest-source` — 管理者専用。uploader.jp / getuploader.com からの取得タスクをオンライン agent 1 台に割り当てます。agent がファイルを取得して中央 API に送信すると、API が SHA-256 を計算し、条件を満たす 2 台の保存タスクを作成します。パスワードや CAPTCHA の回避は行いません。
- `POST /api/v1/artifacts/:id/discover-source-files` — 管理者専用。一覧ページから見つかった個別ダウンロードページ URL をプレビューします。1 回の確認につき最大 10 ページ・候補 100 件で、管理画面で選択した後に順番に保全します。
- `GET /api/v1/files/:id/placements` — 管理者／レビュアー向け配置状態と健全なコピー数
- `POST /api/v1/files/:id/replicate` — 必要な場合に代替コピーの作成を手動で要求します（管理者専用）
- `POST /api/v1/admin/placements/reconcile` — 配置の即時再調整（管理者専用）
- `POST /api/v1/nodes/:id/tasks/claim` — ノードトークン認証付きの取得・保存・複製・余剰コピー削除タスク取得
- `POST /api/v1/nodes/:id/tasks/:taskID/source-content?lease_token=...` — agent が取得した元ファイルを中央 API に送信。API が検証後、2 台への保存タスクを作成
- `GET /api/v1/nodes/:id/tasks/:taskID/content?lease_token=...` — 保存先ノード向けの、認証付き中央ステージングデータ取得

### uploader.jp へのアクセス間隔

- 元ファイルの取得は中央 API ではなく、取得タスクを受け取ったノード agent が実行します。
- `NODE_UPLOADER_MIN_INTERVAL` は **agent ごと**に、uploader.jp への各 HTTP リクエスト（ページ取得、フォーム送信、リダイレクトを含む）の間隔を制限します。既定値は `30s` です。例えば `1m` にすると、その agent は前回のリクエスト開始から最低 1 分空けます。
- この制限を中央 API や全ノード共通のロックとして実装していません。別々の agent はそれぞれの設定に従って独立して動作します。
- 最終取得開始時刻は agent の状態ファイルに隣接するファイルへ保存されるため、agent を再起動しても間隔を引き継ぎます。値 `0s` は制限なしです。通常は既定値以上を推奨します。
- この機能を使うには、全ノードの agent を新しいイメージで再ビルド・再起動してください（`docker compose up -d --build node-agent node-agent-b node-agent-c node-agent-d`）。旧 agent は `source_fetch` タスクに対応していません。
- 一覧URLの候補確認は中央 API が最大 10 ページ・100 候補に制限して行います。これはファイル本体のダウンロードではなく、HTML 一覧を読む処理です。
- `POST /api/v1/nodes/:id/tasks/:taskID/replica-content?lease_token=...` — コピー元ノードからの認証付き複製データ中継アップロード
- `POST /api/v1/nodes/:id/tasks/:taskID/result` — 認証付きのハッシュ／サイズ結果報告

ノードの認証情報は登録時に一度だけ返され、データベースには SHA-256 ハッシュのみ保存されます。ノード登録用シークレットを漏らさないでください。ノードのストレージをエンドユーザーに直接公開することはありません。

### 管理者によるタスク再試行の例

ステージング元データが正常なら、代替ファイルを再アップロードせずに再試行を予約できます。

```sh
curl -sS -X POST "http://localhost:8080/api/v1/tasks/$TASK_ID/retry" \
  -H "Authorization: Bearer $TOKEN"
```

ステージング元データがない場合や整合性検証に失敗した場合は、正しい元ファイルを指定します（ファイルオブジェクトに記録されたサイズと SHA-256 が一致する必要があります）。

```sh
curl -sS -X POST "http://localhost:8080/api/v1/tasks/$TASK_ID/retry" \
  -H "Authorization: Bearer $TOKEN" \
  -F 'file=@/path/to/correct-file'
```

## MVP-3 の配置動作

- 新規保全には、実際の空き容量と参加者の申告容量の条件を満たすオンラインノードが 2 台必要です。条件を満たすノードが 2 台未満の場合、保全済みと誤認させないよう `409 Conflict` を返します。
- 保存先ごとに独立したタスク ID と一時ステージングデータを用意します。保存に成功した保存先のステージングデータは削除し、失敗した場合は管理者が再試行できるよう保持します。
- 複製タスクを受け取ったコピー元ノードは、転送前に元データのサイズと SHA-256 を再検証します。API も保存タスクを作る前にデータを再検証し、保存先エージェントは通常の取り込みと同じ書き込み・検証手順を実行します。
- 健全で検証済みの配置が 2 つ以上ある場合に限り、ファイルは `verified` になります。健全なコピーが 1 つなら `degraded`、現在到達可能な検証済みコピーがなければ `missing`、検証済み配置がなく保存処理が最終的に失敗した場合は `error` になります。
- 自動再調整は 30 秒ごとに実行され、2 分間ハートビートがないノードをオフラインとして扱います。オフラインになったノード上の物理データは削除しません。
- **余剰コピーの扱い：** 必要な健全コピー 2 つに予備 1 つを加えた状態を維持します。健全なコピーが 4 つ以上ある場合、ファイルを保持するすべてのオンラインノードが 5 分以上継続してオンラインで、対象配置も検証から 5 分以上経過しているときに限り、最も新しい検証済み配置の削除タスクを予約します。同じファイルの削除タスクは同時に 1 件までで、ほかのタスクが動作中なら削除を予約しません。ノードエージェントは管理対象の正確なストレージキーだけを削除します。既に存在しないオブジェクトの削除は冪等です。エージェントから成功報告が届いた場合のみ配置状態を `deleted` に変更します。削除に失敗した場合や結果が不確かな場合は `error` とし、健全なコピーとして数えません。

## 容量設定

- `NODE_CAPACITY_BYTES` は参加者が申告する容量上限です。`0` は参加者側の上限なしを意味しますが、実際の空きディスク容量による制約は残ります。
- エージェントはファイルシステムの実容量・空き容量と、管理対象オブジェクトのサイズを報告します。API はアップロード先を割り当てる前に、実際の空き容量と申告容量の両方を確認します。
- 容量情報は 30 秒ごとに再報告されます。API は最近ハートビートが届いているノードのみを保存先として選択します。

## 安全上の既定値と制限

- API で新規ファイルオブジェクトを作成した場合の既定値は、`preservation_status=uncollected`、`redistribution_policy=unknown`、`public_download_enabled=false` です。管理画面からの新規登録では、再配布ポリシーの初期選択は `pending`（未確認）です。
- `pending`（未確認）のファイルは保全を開始できません。`in_progress`（確認中）、`unknown`（不明）、`allowed`（許可済み）、`restricted`（制限あり）は保全できます。保全と再配布許可は別の扱いです。一般公開カタログに掲載されるのは、一般公開が明示的に有効化され、再配布ポリシーが `allowed` で、整合性条件を満たし、最近オンラインであるノードに検証済み配置があるファイルだけです。ダウンロードゲートウェイでも要求時に再判定します。
- 1 回のアップロード上限は 5 GiB です。この MVP では、大容量ファイルの分割・再開アップロードには対応していません。
- タスクの実行試行は 1 回につき最大 5 回です。保存タスクが最終的に失敗した場合、管理者は再試行できます。再試行時は `attempt_count` を `0` に戻し、ステージングデータが期待するサイズと SHA-256 に一致することを再検証してから、最大 5 回の試行を新たに行います。過去のタスク結果と再試行操作は監査ログに残ります。
- 失敗したタスクのステージング元データは、管理者が再試行できるよう保持します。再試行時に代替データをアップロードする場合は `-F 'file=@/path/to/correct-file'` を使用し、ファイルオブジェクトに記録されたサイズと SHA-256 に一致させてください。タスク成功後はステージングデータを削除します。ノードエージェントは各試行の前に未検証の古い保存データと放置された `.partial-*` ファイルを削除します。失敗したデータが `verified` として扱われることはありません。
- 取り込み時の multipart フィールド `priority` は省略可能で、既定値は `3` です。指定できる値は `0`〜`5` で、スケジューラーは `5` を優先し、`0` を最後に扱います。
- ローカル Compose の共有登録トークンは開発専用で、本番環境では `APP_ENV=production` にすると利用できません。`TRUSTED_PROXIES` は `X-Forwarded-For` を信頼するプロキシの IP/CIDR だけを指定してください。本番では承認済み申請ごとの一回限りトークンを使い、TLS、ノード識別・ライフサイクル管理、レート制限、監視、バックアップ／復元テスト、認証情報のローテーション／失効手順も整備してください。
- ローカルの 3 ノード構成は、1 台の物理ホスト上で配置・修復ロジックを実演するものです。障害ドメインは独立していません。本番ノードは別々のホスト／ストレージシステムで動かしてください。
- 代替コピーを予約できるのは、少なくとも 1 つの検証済みコピー元と、それとは別の条件を満たす保存先ノードがオンラインの場合だけです。ノードが 2 台しかない状態で 1 台がオフラインになると、オフラインノードが復帰するか 3 台目が参加するまで、残りのコピーは `degraded` のままです。
- 一覧ページの確認は、1 回につき最大 10 ページ・候補 100 件に制限されています。同じホスト上のページ送りリンクのみをたどり、ログイン認証、パスワード、CAPTCHA、JavaScript チャレンジを回避しません。Web 管理画面には OTel / Grafana ダッシュボードはまだ含まれていません。
- TLS、シークレット、バックアップ、レート制限、運用管理策を設定するまでは、サービスをインターネットに公開しないでください。

### 余剰コピー削除テスト用の 4 台目ノード（任意）

Compose には専用の `node_data_d` ボリュームを持つ `node-agent-d` が含まれています。これは、必要な 2 コピーと予備 1 コピーを維持する削除ルールのローカルテスト用です。実際の転送と削除をテストするには、まずノード D を起動し、通常の修復処理で D に検証済みコピーが作られるよう一時的にノード B と C を停止してから、B と C を再起動します。4 コピーすべてと対象ノードが 5 分以上安定した後、再調整処理によって最も新しい余剰配置だけが削除されるはずです。Docker ボリュームを削除したり、配置レコードを手動編集したりしないでください。

## 余剰コピー削除の自動回帰テスト

`127.0.0.1:55432` の使い捨て PostgreSQL インスタンスに対して、余剰コピー削除の統合テストを実行します。

```sh
./scripts/test-surplus-cleanup.sh
```

このスクリプトは `docker-compose.test.yml` の `postgres-test` サービスを別途起動し、Atlas の SQL マイグレーションを適用して `internal/api` の統合テストを実行した後、一時的なテスト用コンテナ／ネットワークのみを削除します。アプリケーション用 PostgreSQL（ポート `5432`）には接続せず、Docker ボリュームも削除しません。データベース URL がローカルの `archive_test` データベースを指していない場合、統合テストは実行を拒否します。

テストでは、4 コピーが安定した状態での削除、冪等性、3 コピーの維持、タスク実行中の削除抑止、ノード起動後やコピー検証後の待機時間を確認します。

## Web 管理画面（React + Vite）

React / Vite / React Router のアプリケーションは `web/` にあります。Docker Compose の `web` サービスは本番ビルドを `http://localhost:3000` で配信し、`/api/` を中央 Go API にリバースプロキシします。一般向けカタログは `/`、作品詳細は `/artifacts/:id`、保全リクエストは `/preservation-requests`、公開停止／削除／外部公開情報の申請は `/requests` です。管理画面は `/admin`（ログイン画面は `/admin/login`）です。一般向けカタログには、現在ダウンロード条件を満たしているファイルだけが表示されます。フロントエンドをローカルで開発する場合は次を実行します。

```sh
cd web
npm install
npm run dev
```

Vite は `/api` と `/healthz` を `http://localhost:8080` に転送します。`.env` に設定した初期管理者の認証情報でログインしてください。アプリには、簡易的な一般向けカタログ、作品詳細／ダウンロード画面、一般向け申請フォームに加え、管理者ダッシュボード、手動アップロードと uploader.jp URL 取り込みに対応した作品／ファイル目録、ノード・タスク監視、保全／公開停止申請の確認、RBAC によるユーザー一覧／作成、監査ログ、目録エクスポートが含まれます。認可の最終判断はサーバー側で行います。UI はセキュリティ境界ではありません。

## 本番運用に向けた注意

このリポジトリは運用可能な MVP ですが、無人で本番運用できることを保証するものではありません。実データを受け入れる前に、TLS、固有の登録認証情報、独立した障害ドメインを持つノードを用意し、バックアップと復元手順を検証してください。また、権利者への連絡・エスカレーション、保存期間、公開停止、ノード離脱、複製修復、インシデント対応の手順を定め、実際に動作確認してください。保全する権限のあるコンテンツだけを取り込んでください。メタデータを登録しただけでは、再配布の許可を得たことにはなりません。

## uploader.jp からの取り込み

管理画面のファイル登録フォームでは、ローカルファイル、個別の HTTPS ダウンロードページ URL（例：`https://uu.getuploader.com/NAME/download/123`）、またはアップローダーの一覧 URL を指定できます。一覧 URL の候補確認は最大 10 ページ・100 候補に制限されます。候補を表示しただけではタスクは作成されず、保全対象を選択して「取得タスクを登録」した時点で、ファイルごとに `source_fetch` タスクが作成されます。元ファイルの取得は中央 API ではなく、割り当てられたノード agent が実行します。取得に成功し、SHA-256 の計算と保存先ノード 2 台の確保ができた場合、API は `source_fetch` タスクを完了にして、2 件の `store` タスクと配置レコードをトランザクション内で作成します。取得失敗時はタスクに理由が記録され、認証・パスワード・CAPTCHA・容量不足など再試行しても改善しにくい失敗は自動再試行を止め、管理者が原因を確認してから再試行します。一般的な getuploader の hidden `token` フォームと `downloadx`／`download1.getuploader.com/g/...` へのリンクを処理します。テンプレート差異に備え、ラベルのない同一ホストのダウンロードフォーム（token、md5、またはその他の hidden フィールドあり）、HTML の meta refresh、HTML 内の JavaScript に静的に記載された許可ドメインのダウンロード URL も検出します。自動判定できない場合は、最終ページのホスト・パス、Content-Type、タイトル、リンク数、フォームの method/action/フィールド名を診断情報としてエラーに含めます。agent 起動ログと失敗ログには `uploader_fetcher_version` も記録し、新しいバイナリが反映されたか確認できます。hidden フィールドの値、Cookie、トークン値は診断ログに出しません。JavaScript を実行したり、認証・CAPTCHA を回避したりはしません。パスワード保護ページ、CAPTCHA、年齢確認など追加の確認操作が必要なページ、JavaScript のみで動作するチャレンジ、許可されていないリダイレクトがある場合は自動取得を停止します。その場合は通常のブラウザー操作で取得したファイルを手動アップロードしてください。uploader.jp への各 HTTP リクエスト（初回ページ取得、通常のトークンフォーム送信、リダイレクトを含む）の間隔は各 agent の `NODE_UPLOADER_MIN_INTERVAL` で個別に制限し、既定値は 30 秒です。全 agent 共通のグローバルな間隔制限は設けていないため、複数 agent はそれぞれの設定に従って独立して動作します。一覧ページの候補確認は中央 API が行い、ファイル本体のダウンロード間隔とは別です。一覧確認またはダウンロードの前に、運用者が配布元サイトの規約を確認してください。一般向け保全リクエストフォームでは個別ファイル URL と一覧 URL のどちらも受け付けます。管理画面でリクエストを「確認中」に変更すると作品情報を登録し、個別 URL の場合は対象ファイルも登録しますが、取得タスクは自動では開始しません。管理者は申請詳細の「URLを調査・取得タスク登録」から規約確認後に処理を開始します。一覧 URL の場合は候補を調べ、選択したファイルを取得対象にします。作成した作品 ID／ファイル ID は申請に紐付けられます。保存処理を開始しても一般公開は有効になりません。
