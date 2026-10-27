import { ExternalLink, Files, Pencil, SaveCheck } from "lucide-react";
import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { downloadRequestIntakeFile } from "../../api/admin";
import { Heading } from "../../components/common/Heading";
import { Status } from "../../components/common/Status";
import { useData } from "../../hooks/useData";
import { bytes, date } from "../../lib/format";
import type { Obj } from "../../types/common";
import { ConversationLinks } from "./ConversationLinks";

export function RequestDetailPage({
	api,
	refresh,
	notify,
	role,
	token,
	bumpRefresh,
}: any) {
	const { id = "" } = useParams();
	const nav = useNavigate();
	const [localRefresh, setLocalRefresh] = useState(0);

	const [filesPage, setFilesPage] = useState(1);
	const [filesTotal, setFilesTotal] = useState(0);
	const filesLimit = 20;
	const filesTotalPages = Math.max(1, Math.ceil(filesTotal / filesLimit));

	// 基本データの取得
	const requestData = useData(
		api,
		`/public-requests/${encodeURIComponent(id)}`,
		refresh + localRefresh,
	);
	const request: Obj | undefined = requestData.data.item;
	const artifactID = String(request?.artifact_id || "");

	// ファイルデータの取得
	const [files, setFiles] = useState<Obj[]>([]);
	const [filesLoading, setFilesLoading] = useState(false);
	const [filesError, setFilesError] = useState("");

	async function fetchAllArtifactFiles(aid: string): Promise<Obj[]> {
		const limit = 100;
		const allFiles: Obj[] = [];
		let page = 1;
		let total: number | undefined;

		while (total === undefined || allFiles.length < total) {
			const result = await api(
				`/files?artifact_id=${encodeURIComponent(aid)}&page=${page}&limit=${limit}`,
			);
			const items: Obj[] = result.items || [];

			allFiles.push(...items);

			if (typeof result.total === "number") {
				total = result.total;
			} else if (items.length < limit) {
				break;
			}

			if (items.length === 0) break;
			page++;
		}

		return allFiles;
	}

	useEffect(() => {
		let cancelled = false;

		async function loadFiles() {
			setFiles([]);
			setFilesTotal(0);
			setFilesError("");

			if (!artifactID && !request?.file_object_id) {
				setFilesLoading(false);
				return;
			}

			setFilesLoading(true);

			try {
				let loadedFiles: Obj[];
				let total: number;

				if (artifactID) {
					const result = await api(
						`/files?artifact_id=${encodeURIComponent(artifactID)}&page=${filesPage}&limit=${filesLimit}`,
					);

					loadedFiles = result.items || [];
					total = Number(result.total || 0);
				} else {
					const result = await api(
						`/files/${encodeURIComponent(String(request!.file_object_id))}`,
					);

					loadedFiles = result.file ? [result.file] : [];
					total = loadedFiles.length;
				}

				if (!cancelled) {
					setFiles(loadedFiles);
					setFilesTotal(total);
				}
			} catch (e) {
				if (!cancelled) {
					setFilesError((e as Error).message);
				}
			} finally {
				if (!cancelled) {
					setFilesLoading(false);
				}
			}
		}

		void loadFiles();

		return () => {
			cancelled = true;
		};
	}, [
		api,
		artifactID,
		request?.file_object_id,
		filesPage,
		refresh,
		localRefresh,
	]);

	useEffect(() => {
		setFilesPage(1);
	}, [artifactID, request?.file_object_id]);

	const [status, setStatus] = useState("in_review");
	const [reason, setReason] = useState("");
	const [saving, setSaving] = useState(false);
	const [password, setPassword] = useState("");
	const [activeArtifactID, setActiveArtifactID] = useState("");
	const [terms, setTerms] = useState(false);
	const [preserving, setPreserving] = useState(false);
	const [preserveMessage, setPreserveMessage] = useState("");
	const [intakeIngesting, setIntakeIngesting] = useState(false);
	const [intakeIngestMessage, setIntakeIngestMessage] = useState("");
	const [candidates, setCandidates] = useState<Obj[]>([]);
	const [selectedURLs, setSelectedURLs] = useState<string[]>([]);
	const [isTitleEditing, setIsTitleEditing] = useState<boolean>(false);
	const [title, setTitle] = useState<string>("タイトル未入力の申請");
	useEffect(() => {
		if (request) {
			setStatus(request.status === "pending" ? "in_review" : request.status);
			setReason("");
		}
	}, [request?.id, request?.status]);
	useEffect(() => {
		setTitle(request?.title || "");
	}, [request?.id, request?.title]);
	async function ingestAcceptedUpload() {
		if (!request) return;
		if (
			!window.confirm(
				"権利者の再配布許諾と配布条件を確認済みですか？ この操作はファイルを保存ノードへ保全しますが、一般公開は有効化しません。",
			)
		)
			return;
		setIntakeIngesting(true);
		setIntakeIngestMessage("保全タスクを登録しています…");
		try {
			const result = await api(`/public-requests/${request.id}/ingest-upload`, {
				method: "POST",
			});
			setIntakeIngestMessage(
				`保全タスクを登録しました。ファイルID: ${result.file_object_id || "登録済み"}。公開設定は別途確認してください。`,
			);
			notify("カタログ登録と保全タスクの登録が完了しました");
			setLocalRefresh((x) => x + 1);
			bumpRefresh?.();
		} catch (e) {
			setIntakeIngestMessage(`登録できませんでした：${(e as Error).message}`);
			notify((e as Error).message);
		} finally {
			setIntakeIngesting(false);
		}
	}
	async function saveStatus() {
		if (!request || reason.trim().length < 3) {
			notify("対応理由を3文字以上で入力してください");
			return;
		}
		if (
			request.request_type === "deletion" &&
			status === "accepted" &&
			role !== "admin"
		) {
			notify("削除申請の承認は管理者のみです");
			return;
		}
		if (
			request.request_type === "deletion" &&
			status === "accepted" &&
			!window.confirm(
				"削除申請を承認すると、公開停止に加えて保存ノードへの削除タスクが登録されます。対象を確認しましたか？",
			)
		)
			return;
		setSaving(true);
		try {
			await api(`/public-requests/${request.id}`, {
				method: "PATCH",
				body: JSON.stringify({ status, reason: reason.trim() }),
			});
			notify("申請ステータスを更新しました");
			setReason("");
			setLocalRefresh((x) => x + 1);
			bumpRefresh?.();
		} catch (e) {
			notify((e as Error).message);
		} finally {
			setSaving(false);
		}
	}
	async function ensureArtifact(req: Obj) {
		if (req.artifact_id) {
			setActiveArtifactID(String(req.artifact_id));
			return String(req.artifact_id);
		}
		const artifact: Obj = await api("/artifacts", {
			method: "POST",
			body: JSON.stringify({
				title: req.title || "保全リクエスト",
				creator_name: req.creator_name || undefined,
				source_url: req.source_url,
				description: req.details || undefined,
			}),
		});
		await api(`/public-requests/${req.id}`, {
			method: "PATCH",
			body: JSON.stringify({
				status: req.status === "pending" ? "in_review" : req.status,
				reason: "申請詳細ページから作品情報を登録",
				artifact_id: artifact.id,
			}),
		});
		setActiveArtifactID(String(artifact.id));
		setLocalRefresh((x) => x + 1);
		bumpRefresh?.();
		return String(artifact.id);
	}
	function individualURL(raw: string) {
		try {
			const u = new URL(raw);
			return (
				u.pathname.toLowerCase().includes("/download/") ||
				(u.hostname.toLowerCase().startsWith("dl") &&
					u.hostname.toLowerCase().includes("getuploader.com") &&
					u.pathname.toLowerCase().includes("/g/"))
			);
		} catch {
			return false;
		}
	}
	async function beginPreservation() {
		if (!request || !request.source_url || !terms) return;
		setPreserving(true);
		setPreserveMessage("保全対象を準備しています…");
		setCandidates([]);
		setSelectedURLs([]);
		try {
			const aid = await ensureArtifact(request);
			if (individualURL(request.source_url)) {
				const current = await fetchAllArtifactFiles(aid);

				let file: Obj | undefined;
				if (request.file_object_id) {
					try {
						const result = await api(
							`/files/${encodeURIComponent(String(request.file_object_id))}`,
						);
						file = result.file;
					} catch {
						// 直接取得できない場合は、作品内の全ファイルから検索する。
					}
				}
				if (!file) {
					file = current.find((x: Obj) => x.source_url === request.source_url);
				}

				if (file?.preservation_status === "verified") {
					setPreserveMessage(
						"このファイルはすでに検証済みです。重複取得を行いません。",
					);
					return;
				}
				if (!file) {
					file = await api(`/artifacts/${aid}/files`, {
						method: "POST",
						body: JSON.stringify({
							original_filename: (request.title || "取得待ち").slice(0, 500),
							size_bytes: 0,
							media_type: "application/octet-stream",
							source_url: request.source_url,
							redistribution_policy: "unknown",
						}),
					});
				}
				if (!file) throw new Error("ファイル登録結果を取得できませんでした");
				await api(`/public-requests/${request.id}`, {
					method: "PATCH",
					body: JSON.stringify({
						status: "in_review",
						reason: "保全対象ファイルを登録",
						artifact_id: aid,
						file_object_id: file.id,
					}),
				});
				const result = await api(`/files/${file.id}/ingest-source`, {
					method: "POST",
					body: JSON.stringify({
						source_url: request.source_url,
						accept_terms: true,
						priority: 3,
						password,
						public_request_id: request.id,
					}),
				});
				setPreserveMessage(
					result.source_fetch_task
						? `取得タスクを ${result.source_fetch_task.node_name || "取得ノード"} に登録しました。取得成功後に保存タスクが作成されます。`
						: result.message || "取得タスクを登録しました。",
				);
				setLocalRefresh((x) => x + 1);
				bumpRefresh?.();
			} else {
				const result = await api(`/artifacts/${aid}/discover-source-files`, {
					method: "POST",
					body: JSON.stringify({
						listing_url: request.source_url,
						accept_terms: true,
						password,
						public_request_id: request.id,
					}),
				});
				const found: Obj[] = result.items || [];
				setCandidates(found);
				setSelectedURLs(found.map((x: Obj) => x.url));
				setPreserveMessage(
					`${result.pages_scanned ?? 0} ページを調査し、${found.length} 件のファイル候補を見つけました。取得するものを選択してください。`,
				);
			}
		} catch (e) {
			setPreserveMessage(`処理に失敗しました：${(e as Error).message}`);
		} finally {
			setPreserving(false);
		}
	}
	async function queueSelected() {
		const aid = activeArtifactID || String(request?.artifact_id || "");
		if (!request || !aid || !selectedURLs.length) return;
		setPreserving(true);
		let queued = 0,
			skipped = 0;
		const errors: string[] = [];
		try {
			const existing = await fetchAllArtifactFiles(aid);
			let linked = String(request.file_object_id || "");
			for (const item of candidates.filter((x: Obj) =>
				selectedURLs.includes(x.url),
			)) {
				try {
					let file: Obj | undefined = existing.find(
						(x: Obj) => x.source_url === item.url,
					);
					if (file?.preservation_status === "verified") {
						skipped++;
						continue;
					}
					if (!file) {
						const created: Obj = await api(`/artifacts/${aid}/files`, {
							method: "POST",
							body: JSON.stringify({
								original_filename: (item.label || "取得待ち").slice(0, 500),
								size_bytes: 0,
								media_type: "application/octet-stream",
								source_url: item.url,
								redistribution_policy: "unknown",
							}),
						});
						file = created;
						existing.push(created);
					}
					if (!file?.id) throw new Error("ファイル登録結果にIDがありません");
					if (!linked) {
						await api(`/public-requests/${request.id}`, {
							method: "PATCH",
							body: JSON.stringify({
								status: "in_review",
								reason: "保全対象ファイルを登録",
								artifact_id: aid,
								file_object_id: file.id,
							}),
						});
						linked = file.id;
					}
					const result = await api(`/files/${file.id}/ingest-source`, {
						method: "POST",
						body: JSON.stringify({
							source_url: item.url,
							accept_terms: true,
							priority: 3,
							password,
							public_request_id: request.id,
						}),
					});
					if (result.source_fetch_task) queued++;
				} catch (e) {
					errors.push(`${item.label || item.url}: ${(e as Error).message}`);
				}
			}
			setPreserveMessage(
				`取得タスク ${queued} 件登録、検証済みスキップ ${skipped} 件、失敗 ${errors.length} 件${errors.length ? `。最初の失敗：${errors[0]}` : ""}`,
			);
			setCandidates([]);
			setSelectedURLs([]);
			setActiveArtifactID("");
			setLocalRefresh((x) => x + 1);
			bumpRefresh?.();
		} catch (e) {
			setPreserveMessage(`処理に失敗しました：${(e as Error).message}`);
		} finally {
			setPreserving(false);
		}
	}
	async function saveTitle(value: string) {
		if (!request) {
			notify("内部エラー");
			return;
		}
		setSaving(true);
		try {
			await api(`/public-requests/${request.id}`, {
				method: "PATCH",
				body: JSON.stringify({
					title: value,
					reason: "アップローダー名の修正",
				}),
			});
			notify("タイトルを更新しました");
			setLocalRefresh((x) => x + 1);
			bumpRefresh?.();
			setIsTitleEditing(false);
		} catch (e) {
			notify((e as Error).message);
		} finally {
			setSaving(false);
		}
	}
	if (requestData.loading)
		return (
			<>
				<Heading
					eyebrow="申請詳細"
					title="申請を読み込んでいます"
					desc="申請情報を取得しています。"
				/>
				<div className="panel empty">読み込み中…</div>
			</>
		);
	if (requestData.error || !request)
		return (
			<>
				<Heading
					eyebrow="申請詳細"
					title="申請が見つかりません"
					desc={
						requestData.error ||
						"一覧から削除されたか、表示権限がない可能性があります。"
					}
					actions={
						<button
							className="secondary"
							onClick={() => nav("/admin/requests")}
						>
							申請一覧へ戻る
						</button>
					}
				/>
			</>
		);
	return (
		<div className="request-page">
			<div className="request-page-top">
				<button
					type="button"
					className="back-link"
					onClick={() => nav("/admin/requests")}
				>
					← 申請一覧に戻る
				</button>
				<span className="type-pill">
					{request.request_type === "preservation"
						? "保全"
						: request.request_type === "deletion"
							? "削除"
							: request.request_type === "takedown"
								? "公開停止"
								: request.request_type === "redistribution_assistance"
									? "再配布相談"
									: request.request_type === "publisher_assistance"
										? "配布代行"
										: "移設先情報"}
				</span>
				<Status value={request.status} />
			</div>
			<Heading
				eyebrow="申請詳細"
				title={
					<>
						<div
							style={{
								display: "flex",
								flexDirection: "row",
								justifyContent: "flex-start",
								alignItems: "center",
								gap: "4px",
							}}
						>
							{isTitleEditing ? (
								<input
									maxLength={500}
									value={title}
									onChange={(v) => setTitle(v.target.value)}
									placeholder="アップローダー・ファイル名"
									aria-label="申請のタイトル(アップローダー・ファイル名)"
								/>
							) : (
								title ||
								request.linked_file_name ||
								request.linked_artifact_title ||
								"タイトル未入力の申請"
							)}
							<button
								type="button"
								aria-label={
									isTitleEditing ? "タイトルを保存" : "タイトルを編集"
								}
								className="small-btn"
								disabled={saving}
								onClick={() => {
									if (isTitleEditing) {
										saveTitle(title);
									} else {
										setIsTitleEditing(true);
									}
								}}
							>
								{isTitleEditing ? (
									<SaveCheck size={12} />
								) : (
									<Pencil size={12} />
								)}
							</button>
						</div>
					</>
				}
				desc={`受付日時：${date(request.created_at)}　／　申請ID：${request.id}`}
			/>
			<div className="request-page-grid">
				<div className="request-page-main">
					<section className="panel request-info-panel">
						<div className="panel-head">
							<div>
								<h2>申請内容</h2>
								<p>申請者から届いた情報</p>
							</div>
						</div>
						<dl className="request-page-dl">
							<dt>申請者</dt>
							<dd>{request.submitter_name || "匿名"}</dd>
							<dt>連絡先</dt>
							<dd>{request.contact || "未記入"}</dd>
							<dt>作者・配布者</dt>
							<dd>{request.creator_name || "未記入"}</dd>
							<dt>権利者への連絡先</dt>
							<dd>{request.rights_holder_contact || "未記入"}</dd>
							<dt>申請内容</dt>
							<dd className="request-page-details">
								{request.details || "補足はありません"}
							</dd>
							<dt>保全元URL</dt>
							<dd>
								{request.source_url ? (
									<a href={request.source_url} target="_blank" rel="noreferrer">
										{request.source_url} <ExternalLink size={13} />
									</a>
								) : (
									"—"
								)}
							</dd>
							<dt>移設先URL</dt>
							<dd>
								{request.external_url ? (
									<a
										href={request.external_url}
										target="_blank"
										rel="noreferrer"
									>
										{request.external_url} <ExternalLink size={13} />
									</a>
								) : (
									"—"
								)}
							</dd>
							<dt>パスワード</dt>
							<dd>
								{request.has_source_password
									? "申請時に提供あり（暗号化保存）"
									: "未提供"}
							</dd>
						</dl>
					</section>
					{request.intake_file_name && (
						<section className="panel request-info-panel">
							<div className="panel-head">
								<div>
									<h2>申請時の添付ファイル</h2>
									<p>
										権利確認のために送信されたファイル。公開用の保存ファイルとは別です。
									</p>
								</div>
							</div>
							<dl className="request-page-dl">
								<dt>ファイル名</dt>
								<dd>{request.intake_file_name}</dd>
								<dt>サイズ</dt>
								<dd>
									{request.intake_file_size
										? bytes(request.intake_file_size)
										: "不明"}
								</dd>
								<dt>SHA-256</dt>
								<dd>
									<code>{request.intake_file_sha256 || "—"}</code>
								</dd>
								<dt>送信日時</dt>
								<dd>{date(request.intake_file_uploaded_at)}</dd>
							</dl>
							{["rejected", "closed"].includes(request.status) ? (
								<p className="public-footnote">
									申請が却下・終了となったため、添付ファイルは削除済みです。
								</p>
							) : (
								<button
									type="button"
									className="secondary"
									onClick={() =>
										downloadRequestIntakeFile(
											request.id,
											token,
											notify,
											request.intake_file_name,
										)
									}
								>
									添付ファイルを取得
								</button>
							)}
						</section>
					)}
					<section className="panel request-files-panel">
						<div className="panel-head">
							<div>
								<div style={{ display: "flex", flexDirection: "row" }}>
									<h2>関連ファイル</h2>
									<span className="type-pill">{filesTotal} 件</span>
								</div>
								<p>この申請に紐付く作品・ファイル</p>
							</div>
							{request.artifact_id && (
								<button
									className="secondary"
									onClick={() =>
										nav(
											`/admin/review?tab=files&artifact_id=${encodeURIComponent(request.artifact_id)}`,
										)
									}
								>
									作品内の全ファイルを一括操作
								</button>
							)}
						</div>
						{!request.artifact_id && !request.file_object_id ? (
							<div className="empty">
								公開カタログのファイルはまだ紐付いていません。添付ファイルがある場合は、上の「申請時の添付ファイル」から確認できます。
							</div>
						) : filesLoading ? (
							<div className="empty">ファイルを読み込んでいます…</div>
						) : filesError ? (
							<div className="empty">
								ファイル一覧を読み込めませんでした：{filesError}
							</div>
						) : files.length ? (
							<div className="request-file-list">
								{files.map((file: Obj) => (
									<article className="request-file-row" key={file.id}>
										<div className="request-file-icon">
											<Files size={18} />
										</div>
										<div className="request-file-info">
											<b>
												{file.original_filename ||
													file.artifact_title ||
													"ファイル名未設定"}
											</b>
											<small>{file.source_url || file.id}</small>
											<div className="request-file-status">
												<Status
													value={
														file.preservation_status || file.status || "queued"
													}
												/>
												<span>
													{file.size_bytes
														? bytes(file.size_bytes)
														: "サイズ未取得"}
												</span>
												<span>
													{file.redistribution_policy === "allowed"
														? "公開許可済み"
														: "一般公開しない"}
												</span>
											</div>
										</div>
										<button
											type="button"
											className="secondary"
											onClick={() =>
												nav(
													`/admin/review?tab=files&file_id=${encodeURIComponent(file.id)}`,
												)
											}
										>
											ファイル操作
										</button>
									</article>
								))}
								{filesTotal > filesLimit && (
									<div className="request-file-pagination">
										<span>
											{(filesPage - 1) * filesLimit + 1}～
											{Math.min(filesPage * filesLimit, filesTotal)} 件 / 全
											{filesTotal} 件
										</span>

										<div className="request-file-pagination__controls">
											<button
												type="button"
												disabled={filesLoading || filesPage <= 1}
												onClick={() => setFilesPage((page) => page - 1)}
											>
												前へ
											</button>

											<span className="request-file-pagination__page">
												{filesPage} / {filesTotalPages}
											</span>

											<button
												type="button"
												disabled={filesLoading || filesPage >= filesTotalPages}
												onClick={() => setFilesPage((page) => page + 1)}
											>
												次へ
											</button>
										</div>
									</div>
								)}
							</div>
						) : (
							<div className="empty">関連ファイルはまだありません。</div>
						)}
					</section>
					{role !== "viewer" && (
						<section className="panel request-conversations-panel">
							<div className="panel-head">
								<div>
									<h2>連絡・会話</h2>
									<p>LibreDesk の会話 URL を複数登録できます。</p>
								</div>
							</div>
							<ConversationLinks
								api={api}
								refresh={refresh + localRefresh}
								notify={notify}
								targetType="public_request"
								targetId={request.id}
								role={role}
							/>
						</section>
					)}
				</div>
				<aside className="request-page-sidebar">
					{role !== "viewer" && (
						<section className="panel request-action-panel">
							<div className="panel-head">
								<div>
									<h2>申請を処理</h2>
									<p>確認後に状態を更新します。</p>
								</div>
							</div>
							<div className="form-stack">
								<label>
									対応ステータス
									<select
										value={status}
										onChange={(e) => setStatus(e.target.value)}
									>
										<option value="in_review">確認中</option>
										<option
											value="accepted"
											disabled={
												request.request_type === "deletion" && role !== "admin"
											}
										>
											承認
										</option>
										<option value="rejected">却下</option>
										<option value="closed">終了</option>
									</select>
								</label>
								<label>
									対応理由 *
									<textarea
										rows={4}
										minLength={3}
										value={reason}
										onChange={(e) => setReason(e.target.value)}
										placeholder="判断理由や申請者への連絡メモを入力"
									/>
								</label>
								<button
									className="primary wide"
									disabled={saving || reason.trim().length < 3}
									onClick={saveStatus}
								>
									{saving ? "保存中…" : "対応内容を保存"}
								</button>
							</div>
						</section>
					)}
					{role === "admin" &&
						["redistribution_assistance", "publisher_assistance"].includes(
							request.request_type,
						) &&
						request.intake_file_name &&
						request.status === "accepted" &&
						!request.artifact_id &&
						!request.file_object_id && (
							<section className="panel request-action-panel">
								<div className="panel-head">
									<div>
										<h2>許諾確認後の保全</h2>
										<p>
											権利者の再配布許諾と配布条件を確認した後に実行します。添付ファイルは自動ウイルス検査されません。
										</p>
									</div>
								</div>
								<div className="form-stack">
									<p>
										ファイルを非公開の保存ノードへ保全します。これだけでは一般公開・ダウンロードは有効になりません。
									</p>
									<button
										type="button"
										className="primary wide"
										disabled={intakeIngesting}
										onClick={ingestAcceptedUpload}
									>
										{intakeIngesting
											? "登録中…"
											: "許諾確認済み：カタログ登録・保全"}
									</button>
									{intakeIngestMessage && (
										<div className="info-card">
											<div>
												<b>処理結果</b>
												<p>{intakeIngestMessage}</p>
											</div>
										</div>
									)}
								</div>
							</section>
						)}
					{role === "admin" &&
						request.request_type === "preservation" &&
						request.source_url &&
						["pending", "in_review"].includes(request.status) && (
							<section className="panel request-action-panel">
								<div className="panel-head">
									<div>
										<h2>保全を実行</h2>
										<p>ファイルを取得して保存タスクを登録します。</p>
									</div>
								</div>
								<div className="form-stack">
									<label>
										ダウンロードパスワード（任意）
										<input
											type="password"
											maxLength={1024}
											autoComplete="off"
											value={password}
											onChange={(e) => setPassword(e.target.value)}
											placeholder={
												request.has_source_password
													? "申請時に提供済み。上書きする場合のみ入力"
													: "必要な場合に入力"
											}
										/>
										<small>
											空欄なら申請時に暗号化保存されたパスワードを使用します。追加認証は回避しません。
										</small>
									</label>
									<label className="check-label">
										<input
											type="checkbox"
											checked={terms}
											onChange={(e) => setTerms(e.target.checked)}
										/>{" "}
										利用規約を確認しました
									</label>
									<button
										type="button"
										className="primary wide"
										disabled={!terms || preserving}
										onClick={beginPreservation}
									>
										{preserving ? "処理中…" : "URLを調査・取得タスク登録"}
									</button>
									{preserveMessage && (
										<div className="info-card">
											<div>
												<b>処理結果</b>
												<p>{preserveMessage}</p>
											</div>
										</div>
									)}
									{candidates.length > 0 && (
										<>
											<div className="candidate-list">
												{candidates.map((item: Obj) => (
													<label className="candidate-row" key={item.url}>
														<input
															type="checkbox"
															checked={selectedURLs.includes(item.url)}
															onChange={(e) =>
																setSelectedURLs((old) =>
																	e.target.checked
																		? [...old, item.url]
																		: old.filter((x) => x !== item.url),
																)
															}
														/>
														<span>
															<b>{item.label || "ファイル候補"}</b>
															<small>{item.url}</small>
														</span>
													</label>
												))}
											</div>
											<button
												type="button"
												className="secondary"
												onClick={() =>
													setSelectedURLs(candidates.map((x: Obj) => x.url))
												}
											>
												すべて選択
											</button>
											<button
												type="button"
												className="primary wide"
												disabled={preserving || selectedURLs.length === 0}
												onClick={queueSelected}
											>
												選択した {selectedURLs.length} 件を保全
											</button>
										</>
									)}
								</div>
							</section>
						)}
				</aside>
			</div>
		</div>
	);
}
