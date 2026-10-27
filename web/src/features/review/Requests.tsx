import { AlertTriangle, ClipboardList, ExternalLink } from "lucide-react";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Heading } from "../../components/common/Heading";
import { Status } from "../../components/common/Status";
import { useData } from "../../hooks/useData";
import { date } from "../../lib/format";
import type { Obj } from "../../types/common";

export function Requests({ api, refresh, notify, role, bumpRefresh }: any) {
	const nav = useNavigate();
	const d = useData(api, "/public-requests?limit=100", refresh);
	const [filter, setFilter] = useState("pending");
	const [selectedRequestIDs, setSelectedRequestIDs] = useState<string[]>([]);
	const [bulkPreserveTerms, setBulkPreserveTerms] = useState(false);
	const [bulkPreserveBusy, setBulkPreserveBusy] = useState(false);
	const [bulkReviewBusy, setBulkReviewBusy] = useState(false);
	const [bulkPreserveStatus, setBulkPreserveStatus] = useState("");
	const items = (d.data.items || []).filter(
		(x: Obj) => filter === "all" || x.status === filter,
	);
	function toggleRequest(id: string, checked: boolean) {
		setSelectedRequestIDs((old) =>
			checked
				? old.includes(id)
					? old
					: [...old, id]
				: old.filter((x) => x !== id),
		);
	}
	async function bulkReview(nextStatus: "accepted" | "rejected") {
		if (bulkReviewBusy) return;
		const selected = (d.data.items || []).filter((x: Obj) =>
			selectedRequestIDs.includes(x.id),
		);
		if (!selected.length) {
			notify("申請を選択してください");
			return;
		}
		if (nextStatus === "accepted" && role !== "admin") {
			notify("一括承認は管理者のみ利用できます");
			return;
		}
		if (
			nextStatus === "accepted" &&
			selected.some(
				(x: Obj) => !["deletion", "takedown"].includes(x.request_type),
			)
		) {
			notify(
				"一括承認は削除・公開停止申請のみ対応しています。保全などの申請は個別に確認してください",
			);
			return;
		}
		if (
			selected.some((x: Obj) => !["pending", "in_review"].includes(x.status))
		) {
			notify("未処理または確認中の申請を選択してください");
			return;
		}
		const reason = window.prompt(
			nextStatus === "accepted"
				? "一括承認の理由を入力してください"
				: "一括却下の理由を入力してください",
		);
		if (!reason || reason.trim().length < 3) return;
		if (
			nextStatus === "accepted" &&
			!window.confirm(
				`選択した ${selected.length} 件の申請を承認します。削除申請は保存ノードへの削除タスクを登録します。続行しますか？`,
			)
		)
			return;
		setBulkReviewBusy(true);
		try {
			let ok = 0;
			const failures: string[] = [];
			for (const item of selected) {
				try {
					await api(`/public-requests/${item.id}`, {
						method: "PATCH",
						body: JSON.stringify({ status: nextStatus, reason: reason.trim() }),
					});
					ok++;
				} catch (e) {
					failures.push(
						`${String(item.id).slice(0, 8)}: ${(e as Error).message}`,
					);
				}
			}
			setSelectedRequestIDs([]);
			bumpRefresh?.();
			notify(
				`${ok} 件を処理しました${failures.length ? `。失敗 ${failures.length} 件：${failures[0]}` : ""}`,
			);
		} finally {
			setBulkReviewBusy(false);
		}
	}
	async function bulkPreserveSelected() {
		const selected = (d.data.items || []).filter((x: Obj) =>
			selectedRequestIDs.includes(x.id),
		);
		if (role !== "admin") {
			notify("一括保全は管理者のみ利用できます");
			return;
		}
		if (!selected.length) {
			notify("保全リクエストを選択してください");
			return;
		}
		if (
			selected.some(
				(x: Obj) => x.request_type !== "preservation" || !x.source_url,
			)
		) {
			notify(
				"一括保全では、保全元URLがある保全リクエストだけを選択してください",
			);
			return;
		}
		if (
			selected.some((x: Obj) => !["pending", "in_review"].includes(x.status))
		) {
			notify("未処理または確認中の保全リクエストだけを選択してください");
			return;
		}
		if (!bulkPreserveTerms) {
			notify("利用規約を確認してチェックしてください");
			return;
		}
		if (
			!window.confirm(
				`選択した ${selected.length} 件の保全リクエストを一括処理します。個別URLは取得タスクを登録し、一覧URLは最大100件の掲載ファイルを調査して取得タスクを登録します。保全したファイルは一般公開されません。続行しますか？`,
			)
		)
			return;
		setBulkPreserveBusy(true);
		let requestProcessed = 0,
			queuedCount = 0,
			skipped = 0,
			noCandidates = 0;
		const failures: string[] = [];
		try {
			for (let i = 0; i < selected.length; i++) {
				const req = selected[i];
				try {
					setBulkPreserveStatus(
						`${i + 1}/${selected.length} 件目を処理中: ${req.title || req.source_url}`,
					);
					let artifactID = String(req.artifact_id || "");
					if (!artifactID) {
						const artifact: Obj = await api("/artifacts", {
							method: "POST",
							body: JSON.stringify({
								title: req.title || "保全リクエスト",
								creator_name: req.creator_name || undefined,
								source_url: req.source_url,
								description: req.details || undefined,
							}),
						});
						artifactID = artifact.id;
					}
					await api(`/public-requests/${req.id}`, {
						method: "PATCH",
						body: JSON.stringify({
							status: "in_review",
							reason: "一括保全の実行により確認中へ変更",
							artifact_id: artifactID,
							...(req.file_object_id
								? { file_object_id: req.file_object_id }
								: {}),
						}),
					});
					const existingResult = await api(
						`/files?artifact_id=${encodeURIComponent(artifactID)}&limit=100`,
					);
					const existingFiles: Obj[] = [...(existingResult.items || [])];
					if (isIndividualDownload(req.source_url)) {
						let file: Obj | undefined = req.file_object_id
							? existingFiles.find((x: Obj) => x.id === req.file_object_id)
							: undefined;
						if (!file)
							file = existingFiles.find(
								(x: Obj) => x.source_url === req.source_url,
							);
						if (file?.preservation_status === "verified") {
							skipped++;
							requestProcessed++;
							continue;
						}
						if (!file) {
							const createdFile: Obj = await api(
								`/artifacts/${artifactID}/files`,
								{
									method: "POST",
									body: JSON.stringify({
										original_filename: (req.title || "取得待ち").slice(0, 500),
										size_bytes: 0,
										media_type: "application/octet-stream",
										source_url: req.source_url,
										redistribution_policy: "unknown",
									}),
								},
							);
							file = createdFile;
							existingFiles.push(createdFile);
						}
						if (!file)
							throw new Error("保全対象ファイルを作成できませんでした");
						const targetFile: Obj = file;
						await api(`/public-requests/${req.id}`, {
							method: "PATCH",
							body: JSON.stringify({
								status: "in_review",
								reason: "一括保全の対象ファイルを登録",
								artifact_id: artifactID,
								file_object_id: targetFile.id,
							}),
						});
						const result = await api(`/files/${targetFile.id}/ingest-source`, {
							method: "POST",
							body: JSON.stringify({
								source_url: req.source_url,
								accept_terms: true,
								priority: 3,
								public_request_id: req.id,
							}),
						});
						if (result.source_fetch_task) queuedCount++;
					} else {
						const discovered = await api(
							`/artifacts/${artifactID}/discover-source-files`,
							{
								method: "POST",
								body: JSON.stringify({
									listing_url: req.source_url,
									accept_terms: true,
									public_request_id: req.id,
								}),
							},
						);
						const candidates: Obj[] = (discovered.items || []).slice(0, 100);
						if (!candidates.length) {
							noCandidates++;
							requestProcessed++;
							continue;
						}
						let linkedFileID = String(req.file_object_id || "");
						for (const item of candidates) {
							try {
								let file = existingFiles.find(
									(x: Obj) => x.source_url === item.url,
								);
								if (file?.preservation_status === "verified") {
									skipped++;
									continue;
								}
								if (!file) {
									const createdFile: Obj = await api(
										`/artifacts/${artifactID}/files`,
										{
											method: "POST",
											body: JSON.stringify({
												original_filename: (item.label || "取得待ち").slice(
													0,
													500,
												),
												size_bytes: 0,
												media_type: "application/octet-stream",
												source_url: item.url,
												redistribution_policy: "unknown",
											}),
										},
									);
									file = createdFile;
									existingFiles.push(createdFile);
								}
								if (!file)
									throw new Error("保全対象ファイルを作成できませんでした");
								const targetFile: Obj = file;
								if (!linkedFileID) {
									await api(`/public-requests/${req.id}`, {
										method: "PATCH",
										body: JSON.stringify({
											status: "in_review",
											reason: "一括保全の対象ファイルを登録",
											artifact_id: artifactID,
											file_object_id: targetFile.id,
										}),
									});
									linkedFileID = targetFile.id;
								}
								const result = await api(
									`/files/${targetFile.id}/ingest-source`,
									{
										method: "POST",
										body: JSON.stringify({
											source_url: item.url,
											accept_terms: true,
											priority: 3,
											public_request_id: req.id,
										}),
									},
								);
								if (result.source_fetch_task) queuedCount++;
							} catch (e) {
								failures.push(
									`${String(req.id).slice(0, 8)} / ${item.label || item.url}: ${(e as Error).message}`,
								);
							}
						}
					}
					requestProcessed++;
				} catch (e) {
					failures.push(
						`${String(req.id).slice(0, 8)}: ${(e as Error).message}`,
					);
				}
			}
		} finally {
			setBulkPreserveBusy(false);
			setSelectedRequestIDs([]);
			setBulkPreserveTerms(false);
			bumpRefresh?.();
			setBulkPreserveStatus(
				`一括保全終了：リクエスト ${requestProcessed}/${selected.length} 件処理、取得タスク ${queuedCount} 件登録、保全済みスキップ ${skipped} 件、候補なし ${noCandidates} 件、失敗 ${failures.length} 件${failures.length ? `。最初の失敗：${failures[0]}` : ""}`,
			);
			notify(
				`一括保全：取得タスク ${queuedCount} 件登録、保全済み ${skipped} 件スキップ、失敗 ${failures.length} 件`,
			);
		}
	}
	function isIndividualDownload(raw: string) {
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
	return (
		<>
			<Heading eyebrow="" title="申請" desc="保全・削除・公開停止の申請" />
			<div className="notice amber-notice">
				<div className="notice-icon">
					<AlertTriangle size={19} />
				</div>
				<div>
					<b>迷ったときは「確認中」に</b>
					<p>
						内容を確認し、必要なら申請者へ連絡してください。保全・公開・削除は別の処理です。削除の承認は保存先からの削除も実行するため、対象を確認してから承認してください。
					</p>
				</div>
			</div>
			<div className="review-summary-grid">
				<button
					type="button"
					className={
						"review-summary-card " + (filter === "pending" ? "selected" : "")
					}
					onClick={() => {
						setFilter("pending");
						setSelectedRequestIDs([]);
					}}
				>
					<span>未処理</span>
					<b>
						{
							(d.data.items || []).filter((x: Obj) => x.status === "pending")
								.length
						}
					</b>
					<small>まず確認する申請</small>
				</button>
				<button
					type="button"
					className={
						"review-summary-card " + (filter === "in_review" ? "selected" : "")
					}
					onClick={() => {
						setFilter("in_review");
						setSelectedRequestIDs([]);
					}}
				>
					<span>確認中</span>
					<b>
						{
							(d.data.items || []).filter((x: Obj) => x.status === "in_review")
								.length
						}
					</b>
					<small>対応を進めている申請</small>
				</button>
				<button
					type="button"
					className={
						"review-summary-card " + (filter === "all" ? "selected" : "")
					}
					onClick={() => {
						setFilter("all");
						setSelectedRequestIDs([]);
					}}
				>
					<span>すべて</span>
					<b>{(d.data.items || []).length}</b>
					<small>受付済みの申請</small>
				</button>
			</div>
			<div className="filter-bar">
				<div className="filter-tabs">
					{[
						["pending", "未処理"],
						["in_review", "確認中"],
						["all", "すべて"],
						["accepted", "承認"],
						["rejected", "却下"],
						["closed", "終了"],
					].map(([v, l]) => (
						<button
							key={v}
							className={filter === v ? "active" : ""}
							onClick={() => {
								setFilter(v);
								setSelectedRequestIDs([]);
							}}
						>
							{l}
						</button>
					))}
				</div>
				{role === "admin" && (
					<button
						className="secondary"
						disabled={bulkPreserveBusy}
						onClick={() => {
							const eligible = items.filter(
								(x: Obj) =>
									x.request_type === "preservation" &&
									x.source_url &&
									["pending", "in_review"].includes(x.status),
							);
							setSelectedRequestIDs(eligible.map((x: Obj) => x.id));
							setBulkPreserveTerms(false);
							notify(`${eligible.length} 件の保全リクエストを選択しました`);
						}}
					>
						<ClipboardList size={16} /> 保全リクエストを一括選択
					</button>
				)}
			</div>
			{selectedRequestIDs.length > 0 && role !== "viewer" && (
				<section className="bulk-toolbar panel">
					<div className="bulk-toolbar-title">
						<b>{selectedRequestIDs.length} 件の申請を選択中</b>
						<button
							className="small-btn"
							onClick={() => setSelectedRequestIDs([])}
						>
							選択解除
						</button>
					</div>
					<div className="bulk-toolbar-controls">
						{role === "admin" && (
							<>
								<button
									className="primary"
									disabled={
										bulkPreserveBusy ||
										!bulkPreserveTerms ||
										(d.data.items || [])
											.filter((x: Obj) => selectedRequestIDs.includes(x.id))
											.some(
												(x: Obj) =>
													x.request_type !== "preservation" ||
													!x.source_url ||
													!["pending", "in_review"].includes(x.status),
											)
									}
									onClick={bulkPreserveSelected}
								>
									{bulkPreserveBusy ? "一括保全中…" : "一括で保全"}
								</button>
								<label className="check-label">
									<input
										type="checkbox"
										checked={bulkPreserveTerms}
										disabled={bulkPreserveBusy}
										onChange={(e) => setBulkPreserveTerms(e.target.checked)}
									/>{" "}
									利用規約を確認しました
								</label>
								<button
									className="secondary"
									disabled={bulkPreserveBusy || bulkReviewBusy}
									onClick={() => bulkReview("accepted")}
								>
									公開停止・削除を一括承認
								</button>
							</>
						)}
						<button
							className="secondary"
							disabled={bulkPreserveBusy || bulkReviewBusy}
							onClick={() => bulkReview("rejected")}
						>
							一括却下
						</button>
					</div>
				</section>
			)}
			{bulkPreserveStatus && (
				<section className="panel">
					<div className="info-card">
						<div>
							<b>一括保全の結果</b>
							<p>{bulkPreserveStatus}</p>
						</div>
					</div>
				</section>
			)}
			<section className="review-request-list" aria-label="申請一覧">
				{d.loading ? (
					<div className="panel empty">申請を読み込んでいます…</div>
				) : d.error ? (
					<div className="panel empty">
						申請を読み込めませんでした：{d.error}
					</div>
				) : items.length ? (
					items.map((x: Obj) => (
						<article className="review-request-card" key={x.id}>
							<div className="review-request-select">
								<input
									type="checkbox"
									aria-label={`${x.title || x.request_type}を選択`}
									checked={selectedRequestIDs.includes(x.id)}
									disabled={role === "viewer"}
									onChange={(e) => toggleRequest(x.id, e.target.checked)}
								/>
							</div>
							<div className="review-request-main">
								<div className="review-request-meta">
									<span className="type-pill">
										{x.request_type === "takedown"
											? "公開停止"
											: x.request_type === "deletion"
												? "削除"
												: x.request_type === "preservation"
													? "保全"
													: x.request_type === "redistribution_assistance"
														? "再配布相談"
														: x.request_type === "publisher_assistance"
															? "配布代行"
															: "移設先情報"}
									</span>
									<Status value={x.status} />
									<span className="review-request-date">
										受付 {date(x.created_at)}
									</span>
									{x.intake_file_name && (
										<span className="review-request-date">
											添付ファイルあり
										</span>
									)}
								</div>
								<h3>
									{x.title ||
										x.linked_file_name ||
										x.linked_artifact_title ||
										"タイトル未入力の申請"}
								</h3>
								<p className="review-request-submitter">
									申請者：{x.submitter_name || "匿名"}　·　連絡先：
									{x.contact || "未記入"}
								</p>
								<p className="review-request-details">
									{String(x.details || "").length > 180
										? String(x.details).slice(0, 180) + "…"
										: x.details || "補足はありません"}
								</p>
								{x.source_url && (
									<a
										className="review-request-url"
										href={x.source_url}
										target="_blank"
										rel="noreferrer"
									>
										保全元URLを開く <ExternalLink size={12} />
									</a>
								)}
								{x.external_url && (
									<a
										className="review-request-url"
										href={x.external_url}
										target="_blank"
										rel="noreferrer"
									>
										移設先URLを開く <ExternalLink size={12} />
									</a>
								)}
							</div>
							<div className="review-request-actions">
								<button
									className="primary"
									onClick={() =>
										nav(`/admin/requests/${encodeURIComponent(x.id)}`)
									}
								>
									申請ページを開く <span>→</span>
								</button>
								{(x.file_object_id || x.artifact_id) && (
									<button
										className="secondary"
										onClick={() =>
											nav(
												`/admin/review?tab=files&${x.file_object_id ? "file_id=" + encodeURIComponent(x.file_object_id) : "artifact_id=" + encodeURIComponent(x.artifact_id)}`,
											)
										}
									>
										ファイル一覧
									</button>
								)}
							</div>
						</article>
					))
				) : (
					<div className="panel empty">この条件の申請はありません。</div>
				)}
			</section>
		</>
	);
}
