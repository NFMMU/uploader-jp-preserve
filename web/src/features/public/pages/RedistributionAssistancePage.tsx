import { type FormEvent, useState } from "react";
import { Link } from "react-router-dom";
import { TurnstileField } from "../../../components/TurnstileField";

export function RedistributionAssistancePage({
	mode,
}: {
	mode: "redistribution" | "publisher";
}) {
	const isPublisher = mode === "publisher";
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	const [submittedID, setSubmittedID] = useState("");
	const [submittedMessage, setSubmittedMessage] = useState("");
	const [turnstileReset, setTurnstileReset] = useState(0);
	async function submit(event: FormEvent<HTMLFormElement>) {
		event.preventDefault();
		setBusy(true);
		setError("");
		const formElement = event.currentTarget;
		const form = new FormData(formElement);
		form.set(
			"turnstile_token",
			String(form.get("cf-turnstile-response") || ""),
		);
		const file = form.get("file");
		if (file instanceof File && file.size > 100 * 1024 * 1024) {
			setError("ファイルは100 MiB以下にしてください。");
			setBusy(false);
			return;
		}
		try {
			const response = await fetch("/api/v1/public/redistribution-assistance", {
				method: "POST",
				body: form,
			});
			const text = await response.text();
			let data: any = {};
			try {
				data = text ? JSON.parse(text) : {};
			} catch {
				data = { message: text };
			}
			if (!response.ok)
				throw new Error(
					data.error || data.message || `HTTP ${response.status}`,
				);
			setSubmittedID(data.id || "");
			setSubmittedMessage(data.message || "申請を受け付けました。");
			formElement.reset();
		} catch (e) {
			setError(e instanceof Error ? e.message : "送信できませんでした。");
		} finally {
			setBusy(false);
			setTurnstileReset((value) => value + 1);
		}
	}
	return (
		<>
			<Link className="public-text-link" to="/">
				← 作品一覧へ戻る
			</Link>
			<h1>
				{isPublisher
					? "配布元・権利者向け：配布代行の相談"
					: "手元にあるファイルの再配布相談"}
			</h1>
			{isPublisher ? (
				<p className="public-intro">
					配布先を探している、ファイルの置き場所や再配布対応が負担になっている配布元・権利者の方へ。ファイルまたは配布元URLを送っていただければ、保全・配布方法を相談し、対応可能な範囲で代行します。
				</p>
			) : (
				<p className="public-intro">
					uploader.jpにあったファイルを手元に持っていて、「良い作品なので再配布したいけれど、権利者への問い合わせが面倒」という方向けの窓口です。ファイルを送っていただければ、運営が権利者への問い合わせを試み、許諾が確認できた場合に限って再配布を検討します。
				</p>
			)}
			<section className="public-process">
				<h2>利用の流れ</h2>
				<ol>
					<li>フォームからファイルや作品情報を送る</li>
					<li>運営が内容を確認し、必要に応じて権利者・配布元へ問い合わせる</li>
					<li>
						許諾や配布条件を確認できた場合に、保全・再配布の可否を相談する
					</li>
				</ol>
				<p>
					送信しただけで公開・再配布されることはありません。権利者と連絡が取れない場合や許諾が得られない場合は、公開・再配布できないことがあります。
				</p>
			</section>
			{submittedID ? (
				<div className="public-success">
					<h2>相談を受け付けました</h2>
					<p>{submittedMessage}</p>
					<p className="public-muted">
						受付番号：<code>{submittedID}</code>
					</p>
					<p>運営が内容を確認し、必要に応じて入力された連絡先へ連絡します。</p>
					<button
						type="button"
						onClick={() => {
							setSubmittedID("");
							setSubmittedMessage("");
						}}
					>
						別の相談を送る
					</button>
				</div>
			) : (
				<form
					className="public-request-form"
					onSubmit={submit}
					encType="multipart/form-data"
				>
					<input
						type="hidden"
						name="request_type"
						value={
							isPublisher ? "publisher_assistance" : "redistribution_assistance"
						}
					/>
					<label>
						アップローダー・ファイル名 *
						<input
							name="title"
							required
							maxLength={500}
							placeholder="分かる範囲のアップローダー名やファイル名"
						/>
					</label>
					<label>
						作者・配布元
						<input
							name="creator_name"
							maxLength={255}
							placeholder="分かる場合"
						/>
					</label>
					<label>
						{isPublisher
							? "現在の配布ページURL（任意）"
							: "元の配布ページURL（任意）"}
						<input
							name="source_url"
							type="url"
							maxLength={4096}
							placeholder="https://..."
						/>
						<small>
							{isPublisher
								? "ファイルを添付しない場合は、配布元URLが必要です。"
								: "分かれば入力してください。"}
						</small>
					</label>
					<label>
						権利者への連絡先・関連情報（任意）
						<input
							name="rights_holder_contact"
							maxLength={1000}
							placeholder="公式サイト、連絡先、SNSなど"
						/>
					</label>
					<label>
						{isPublisher ? "ファイルを添付（任意）" : "手元にあるファイル *"}
						<input name="file" type="file" required={!isPublisher} />{" "}
						<small>
							1ファイル100
							MiBまで。送信ファイルは権利確認のための一時保管であり、一般公開されません。
						</small>
					</label>
					<label>
						相談内容 *
						<textarea
							name="details"
							rows={4}
							required
							minLength={5}
							maxLength={10000}
							placeholder={
								isPublisher
									? "配布代行を希望する理由、希望する配布方法、条件など"
									: "ファイルの由来、再配布したい理由、分かっている権利者情報など"
							}
						/>
					</label>
					<label>
						お名前・活動名（任意）
						<input name="submitter_name" maxLength={255} />
						<small>
							権利者ではなく、ご連絡者様のお名前・活動名をご記入ください。
						</small>
					</label>
					<label>
						連絡先 *
						<input
							name="contact"
							required
							maxLength={500}
							placeholder="返信を受け取れるメールアドレスなど"
						/>
					</label>
					<label className="check-label">
						<input type="checkbox" name="agree_to_terms" required />{" "}
						<span>
							送信ファイルを権利確認のために運営が一時保管・確認すること、権利者の許諾が確認できるまで公開・再配布しないことに同意します。
						</span>
					</label>
					<TurnstileField key={turnstileReset} />
					{error && <p className="public-error">{error}</p>}
					<button type="submit" disabled={busy}>
						{busy ? "送信中…" : "相談を送信する"}
					</button>
					<p className="public-footnote">
						第三者の権利を侵害する目的でファイルを送信しないでください。送信は再配布の許諾を意味せず、運営が再配布を保証するものでもありません。アップロード時の自動ウイルス検査は行いません。連絡先などの情報は、申請への対応に必要な範囲で取り扱います。
					</p>
				</form>
			)}
		</>
	);
}
