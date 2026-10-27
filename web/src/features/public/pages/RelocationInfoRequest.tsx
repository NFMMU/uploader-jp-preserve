import { type FormEvent, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { publicAPI } from "../../../api/public";
import { TurnstileField } from "../../../components/TurnstileField";

export function RelocationInfoRequest() {
	const [params] = useSearchParams();
	const artifactID = params.get("artifact_id") || "";
	const fileID = params.get("file_id") || "";
	const [url, setURL] = useState("");
	const [details, setDetails] = useState("");
	const [name, setName] = useState("");
	const [contact, setContact] = useState("");
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	const [submittedID, setSubmittedID] = useState("");
	const [turnstileReset, setTurnstileReset] = useState(0);
	async function submit(event: FormEvent<HTMLFormElement>) {
		event.preventDefault();
		setBusy(true);
		setError("");
		const turnstileToken = String(
			new FormData(event.currentTarget).get("cf-turnstile-response") || "",
		);
		const body: Record<string, string> = {
			request_type: "external_publication",
			external_url: url.trim(),
			details: details.trim() || "移設先情報を提供します。",
			turnstile_token: turnstileToken,
		};
		if (artifactID) body.artifact_id = artifactID;
		if (fileID) body.file_object_id = fileID;
		if (name.trim()) body.submitter_name = name.trim();
		if (contact.trim()) body.contact = contact.trim();
		try {
			const data = await publicAPI<{ id: string }>("/public-requests", {
				method: "POST",
				body: JSON.stringify(body),
			});
			setSubmittedID(data.id);
		} catch (e) {
			setError(e instanceof Error ? e.message : "送信できませんでした。");
		} finally {
			setBusy(false);
			setTurnstileReset((value) => value + 1);
		}
	}
	return (
		<>
			<h1>移設先情報の提供</h1>
			<p className="public-intro">
				保全対象のファイルが別の場所に移設・再公開されている場合、そのURLをお知らせください。情報は確認担当者が確認します。
			</p>
			{submittedID ? (
				<div className="public-success">
					<p>情報を受け付けました。</p>
					<p className="public-muted">
						受付番号: <code>{submittedID}</code>
					</p>
					<button
						type="button"
						onClick={() => {
							setSubmittedID("");
							setURL("");
							setDetails("");
						}}
					>
						続けて送信
					</button>
				</div>
			) : (
				<form className="public-request-form" onSubmit={submit}>
					{(artifactID || fileID) && (
						<p className="public-target">
							対象: {fileID ? "指定したファイル" : "指定した作品"}
						</p>
					)}
					<label>
						移設先URL *
						<input
							required
							type="url"
							maxLength={4096}
							value={url}
							onChange={(e) => setURL(e.target.value)}
							placeholder="https://example.com/moved-file"
						/>
					</label>
					<label>
						補足
						<textarea
							rows={4}
							maxLength={10000}
							value={details}
							onChange={(e) => setDetails(e.target.value)}
							placeholder="移設先の説明、確認できた日付など"
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
					<TurnstileField key={turnstileReset} />
					{error && <p className="public-error">{error}</p>}
					<button type="submit" disabled={busy}>
						{busy ? "送信中…" : "情報を送信する"}
					</button>
				</form>
			)}
		</>
	);
}
