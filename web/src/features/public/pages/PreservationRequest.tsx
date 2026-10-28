import { type FormEvent, useState } from "react";
import { publicAPI } from "../../../api/public";
import { PageMeta } from "../../../components/common/PageMeta";
import { TurnstileField } from "../../../components/TurnstileField";

export function PreservationRequest() {
	const [title, setTitle] = useState("");
	const [creator, setCreator] = useState("");
	const [sourceURL, setSourceURL] = useState("");
	const [sourcePassword, setSourcePassword] = useState("");
	const [details, setDetails] = useState("");
	const [name, setName] = useState("");
	const [contact, setContact] = useState("");
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	const [submittedID, setSubmittedID] = useState("");
	const [submittedMessage, setSubmittedMessage] = useState("");
	const [turnstileReset, setTurnstileReset] = useState(0);
	async function submit(event: FormEvent<HTMLFormElement>) {
		event.preventDefault();
		setBusy(true);
		setError("");
		const turnstileToken = String(
			new FormData(event.currentTarget).get("cf-turnstile-response") || "",
		);
		try {
			const data = await publicAPI<{
				id?: string;
				duplicate?: boolean;
				message?: string;
			}>("/public-requests", {
				method: "POST",
				body: JSON.stringify({
					turnstile_token: turnstileToken,
					request_type: "preservation",
					title: title.trim(),
					creator_name: creator.trim(),
					source_url: sourceURL.trim(),
					source_password: sourcePassword,
					details: details.trim() || "詳細はありません",
					submitter_name: name.trim(),
					contact: contact.trim(),
				}),
			});
			setSubmittedID(data.id || (data.duplicate ? "duplicate" : ""));
			setSubmittedMessage(
				data.message ||
					(data.duplicate
						? "このURLはすでに登録・申請済みのため、重複リクエストを破棄しました。"
						: "リクエストを受け付けました。"),
			);
			setSourcePassword("");
		} catch (e) {
			setError(e instanceof Error ? e.message : "送信できませんでした。");
		} finally {
			setBusy(false);
			setTurnstileReset((value) => value + 1);
		}
	}
	return (
		<>
			<PageMeta
				title="保全リクエスト"
				description="保全を行ってほしいアップローダー・ファイルをリクエストできます。"
			/>
			<h1>保全リクエスト</h1>
			<p className="public-intro">
				残してほしいファイル、またはアップローダーのURLを送ってください。
			</p>
			{submittedID ? (
				<div className="public-success">
					<p>{submittedMessage}</p>
					{submittedID !== "duplicate" && (
						<p className="public-muted">
							受付番号: <code>{submittedID}</code>
						</p>
					)}
					<p className="public-muted">内容を確認してから保全を検討します。</p>
					<button
						type="button"
						onClick={() => {
							setSubmittedID("");
							setSubmittedMessage("");
							setTitle("");
							setCreator("");
							setSourceURL("");
							setSourcePassword("");
							setDetails("");
						}}
					>
						続けて送信
					</button>
				</div>
			) : (
				<form className="public-request-form" onSubmit={submit}>
					<label>
						アップローダー・ファイル名 *
						<input
							required
							maxLength={500}
							value={title}
							onChange={(e) => setTitle(e.target.value)}
							placeholder="アップローダー・ファイル名"
						/>
					</label>
					<label>
						作者・配布者
						<input
							maxLength={255}
							value={creator}
							onChange={(e) => setCreator(e.target.value)}
							placeholder="分かる場合"
						/>
					</label>
					<label>
						uploader.jp のURL *
						<input
							required
							type="url"
							maxLength={4096}
							pattern="[Hh][Tt][Tt][Pp][Ss]://(?:(?:[A-Za-z0-9]|-)+\.)*(?:[Uu][Pp][Ll][Oo][Aa][Dd][Ee][Rr]\.[Jj][Pp]|[Gg][Ee][Tt][Uu][Pp][Ll][Oo][Aa][Dd][Ee][Rr]\.[Cc][Oo][Mm])\.?(?::443)?(?:[/?#].*)?"
							value={sourceURL}
							onChange={(e) => setSourceURL(e.target.value)}
							placeholder="https://uu.getuploader.com/名前/ または /download/123"
						/>
						<small>
							個別ファイルのURLでも、アップローダーの一覧ページURLでも送れます。
						</small>
					</label>
					<label>
						ダウンロードパスワード（任意）
						<input
							type="password"
							maxLength={1024}
							autoComplete="off"
							value={sourcePassword}
							onChange={(e) => setSourcePassword(e.target.value)}
							placeholder="パスワードが設定されている場合"
						/>
						<small>
							暗号化して保管し、保全時の正規のパスワード入力に使用します。取得担当のノード運営者がファイル内容を扱える点にご注意ください。CAPTCHA等の追加認証は自動処理しません。
						</small>
					</label>
					<label>
						補足
						<textarea
							rows={4}
							maxLength={10000}
							value={details}
							onChange={(e) => setDetails(e.target.value)}
							placeholder="作品情報や保全を希望する理由"
						/>
					</label>
					<label>
						お名前（任意）
						<input
							maxLength={255}
							value={name}
							onChange={(e) => setName(e.target.value)}
						/>
					</label>
					<label>
						連絡先（任意）
						<input
							maxLength={500}
							value={contact}
							onChange={(e) => setContact(e.target.value)}
							placeholder="メールアドレスなど"
						/>
					</label>
					<p className="public-footnote">
						一覧URLの場合は、管理者が掲載ファイルを確認してから順番に保全します。送信だけで自動取得や一般公開は始まりません。
					</p>
					<TurnstileField key={turnstileReset} />
					{error && <p className="public-error">{error}</p>}
					<button type="submit" disabled={busy}>
						{busy ? "送信中…" : "保全を依頼する"}
					</button>
				</form>
			)}
		</>
	);
}
