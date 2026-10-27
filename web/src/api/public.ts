import type { APIError } from "../types/common";

const PUBLIC_API_ERROR_MESSAGES: Record<string, string> = {
    "valid request_type and details are required":
        "申請種類と申請内容を正しく入力してください。",
    "source_url is required for preservation requests":
        "保存申請ではアップロード元URLが必要です。",
    "source_url must be an HTTPS uploader.jp/getuploader.com URL":
        "アップロード元URLは、uploader.jpまたはgetuploader.comのHTTPS URLを指定してください。",
    "title is required for preservation requests":
        "保存申請ではタイトルを入力してください。",
    "source_password is only supported for preservation requests":
        "ソースパスワードは保存申請でのみ指定できます。",
    "external_url is required for external_publication requests":
        "外部公開申請では公開先URLを入力してください。",
    "external_url must be an http(s) URL":
        "公開先URLは有効なhttpまたはhttpsのURLを指定してください。",
    "artifact_id does not exist":
        "指定された成果物が存在しません。",
    "file_object_id does not exist":
        "指定されたファイルが存在しません。",
    "could not check existing source URL":
        "アップロード元URLの確認に失敗しました。",
    "could not check existing preservation requests":
        "既存の保存申請の確認に失敗しました。",
    "could not submit request":
        "申請を送信できませんでした。しばらく待ってから再度お試しください。",
    "invalid file id":
        "ファイル識別子が正しくありません。",
    "file is not available for public download":
        "このファイルは現在公開されていないため、ダウンロードできません。",
    "invalid download id":
        "ダウンロード識別子が正しくありません。",
    "download bearer token required":
        "ダウンロードに必要な認証情報がありません。",
    "download job unavailable":
        "ダウンロード処理が見つかりません。",
    "download is not ready":
        "ダウンロードの準備がまだ完了していません。",
    "file is no longer public":
        "このファイルは公開対象外になったためダウンロードできません。",
    "prepared content expired":
        "ダウンロード可能な期間が終了しました。",
    "could not create download token":
        "ダウンロード用トークンを作成できませんでした。",
    "could not create download job":
        "ダウンロード処理を開始できませんでした。",
    "could not search public catalog":
        "公開カタログを検索できませんでした。",
    "could not load public files":
        "公開ファイル一覧を取得できませんでした。",
};

const STATUS_ERROR_MESSAGES: Record<number, string> = {
    400: "入力内容が正しくありません。",
    401: "認証が必要です。",
    403: "この操作を実行する権限がないか、対象が利用できません。",
    404: "対象が見つかりません。",
	408: "リクエストがタイムアウトしました。",
    409: "処理が競合しました。内容を更新してから再度お試しください。",
	410: "対象の情報は期限切れか削除されています。",
    429: "リクエストが集中しています。しばらく待ってから再度お試しください。",
    500: "サーバーで問題が発生しました。しばらく待ってから再度お試しください。",
	502: "不正なリクエストを受け取りました。",
    503: "現在サービスを利用できません。しばらく待ってから再度お試しください。",
	504: "中継サーバーからの応答がタイムアウトしました。"
};

function translatePublicAPIError(
	status: number,
	message: string | undefined
): APIError {
	if (message && PUBLIC_API_ERROR_MESSAGES[message]) {
		return {
			error: status,
			message: PUBLIC_API_ERROR_MESSAGES[message]
		};
	};
	return {
		error: status,
		message: STATUS_ERROR_MESSAGES[status] ?? "リクエストを処理できませんでした。しばらく待ってから再度お試しください。"
	};
}

export async function publicAPI<T>(
	path: string,
	options: RequestInit = {},
): Promise<T> {
	const response = await fetch(`/api/v1${path}`, {
		...options,
		headers: {
			...(options.body ? { "Content-Type": "application/json" } : {}),
			...options.headers,
		},
	});
	const text = await response.text();
	let data: any = {};
	try {
		data = text ? JSON.parse(text) : {};
	} catch {
		data = { message: text };
	}
	if (!response.ok) {
		const api_error = translatePublicAPIError(response.status, data.error || data.message);
		throw new Error(
			api_error.message
		);
	};
	return data as T;
}
