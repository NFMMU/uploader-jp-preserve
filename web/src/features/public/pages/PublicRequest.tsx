import { type FormEvent, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { publicAPI } from "../../../api/public";
import { TurnstileField } from "../../../components/TurnstileField";

export function PublicRequest() {
	const [params] = useSearchParams();
	const artifactID = params.get("artifact_id") || "";
	const fileID = params.get("file_id") || "";
	const [type, setType] = useState("takedown");
	const [name, setName] = useState("");
	const [contact, setContact] = useState("");
	const [details, setDetails] = useState("");
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
			request_type: type,
			details: details.trim(),
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
			setDetails("");
		} catch (e) {
			setError(e instanceof Error ? e.message : "送信できませんでした。");
		} finally {
			setBusy(false);
			setTurnstileReset((value) => value + 1);
		}
	}
	return (
		<>
			<h1>申請</h1>
			<p className="public-intro">
				公開停止や削除の申請はこちら。移設先URLの情報提供は専用ページから送信できます。
			</p>
			{submittedID ? (
				<div className="public-success">
					<p>申請を受け付けました。</p>
					<p className="public-muted">
						受付番号: <code>{submittedID}</code>
					</p>
					<button type="button" onClick={() => setSubmittedID("")}>
						続けて申請
					</button>
				</div>
			) : (
				<form className="public-request-form" onSubmit={submit}>
					{artifactID && (
						<input type="hidden" name="artifact_id" value={artifactID} />
					)}
					{fileID && (
						<input type="hidden" name="file_object_id" value={fileID} />
					)}
					{(artifactID || fileID) && (
						<p className="public-target">
							対象: {fileID ? "指定したファイル" : "指定した作品"}
						</p>
					)}
					<label>
						申請の種類
						<select value={type} onChange={(e) => setType(e.target.value)}>
							<option value="takedown">公開停止</option>
							<option value="deletion">削除</option>
						</select>
					</label>
					<label>
						内容
						<textarea
							required
							minLength={5}
							maxLength={10000}
							rows={5}
							value={details}
							onChange={(e) => setDetails(e.target.value)}
							placeholder="申請の理由や補足"
						/>
					</label>
					<label>
						お名前（任意）
						<input
							value={name}
							onChange={(e) => setName(e.target.value)}
							maxLength={255}
						/>
					</label>
					<label>
						連絡先（任意）
						<input
							value={contact}
							onChange={(e) => setContact(e.target.value)}
							maxLength={500}
							placeholder="メールアドレスなど"
						/>
					</label>
					<TurnstileField key={turnstileReset} />
					{error && <p className="public-error">{error}</p>}
					<button type="submit" disabled={busy}>
						{busy ? "送信中…" : "申請する"}
					</button>
				</form>
			)}
		</>
	);
}
