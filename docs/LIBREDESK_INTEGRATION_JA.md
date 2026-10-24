# LibreDesk 連携設定

このプロジェクトは LibreDesk の REST API を使い、対象の申請が届いたときに会話を作成して、会話 URL を申請に紐付けます。

## 自動作成の対象

- 新規ストレージノード参加申請
- 人が送信した保全リクエスト
- 削除リクエスト
- 移設先情報の提供リクエスト
- 手元のファイルの再配布相談
- 配布元・権利者からの配布代行相談

DuckDuckGo の URL 自動収集処理が直接登録する候補は、申請者からの新規申請ではないため、自動作成の対象外です。公開停止だけのリクエストも対象外です。

## 環境変数

`.env` に設定します。LibreDesk を使わない場合は、次の必須項目をすべて空欄にしておくと連携は無効です。

```dotenv
LIBREDESK_BASE_URL=https://support.example.com
LIBREDESK_API_KEY=your-agent-api-key
LIBREDESK_API_SECRET=your-agent-api-secret
LIBREDESK_INBOX_ID=1
LIBREDESK_TEAM_ID=
LIBREDESK_AGENT_ID=
LIBREDESK_FALLBACK_CONTACT_EMAIL=uploaderjp-archive@udonya.moe
LIBREDESK_CONVERSATION_URL_TEMPLATE={{base_url}}/app/inbox/conversations/{{uuid}}
```

- API キーは `conversations:write` 権限を持つエージェントから発行してください。
- `LIBREDESK_INBOX_ID` は会話を作成する有効なメール受信箱の ID です。
- `LIBREDESK_TEAM_ID` と `LIBREDESK_AGENT_ID` は必要に応じて指定します。
- 申請者の連絡先にメールアドレスが含まれる場合はそれを使います。連絡先が未入力またはメールアドレスではない場合は `LIBREDESK_FALLBACK_CONTACT_EMAIL` を使い、会話本文に連絡先の確認が必要である旨を記載します。フォールバック先は申請者本人ではないため、申請内容に連絡方法がない場合は別途確認してください。
- `LIBREDESK_CONVERSATION_URL_TEMPLATE` は LibreDesk の会話画面の URL 形式に合わせて確認・変更してください。`{{base_url}}` と `{{uuid}}` が置換されます。標準値の画面パスが利用中の LibreDesk バージョンと異なる場合は、実際の会話画面 URL に合わせてください。

API キーとシークレットは秘密情報です。Git にコミットしないでください。

## 会話 URL の管理

申請レビューの詳細画面、またはストレージノード参加申請の「連絡・会話URL」から、会話 URL を複数登録できます。URL は HTTP(S) のみ許可されます。自動作成された LibreDesk 会話 URL も同じ一覧に追加されます。

## エラー時の挙動

LibreDesk API の障害や認証失敗があっても、申請自体の受付は成功扱いを維持します。会話作成に失敗した場合は API ログに記録されます。LibreDesk の接続先、API 権限、受信箱 ID を確認してください。
