import {
	ArrowUpRight,
	CloudUpload,
	Download,
	Files,
	FolderOpen,
	LockKeyhole,
	Plus,
	Search,
} from "lucide-react";
import type React from "react";
import { useEffect, useRef, useState } from "react";
import { NavLink, useLocation, useNavigate, useParams } from "react-router-dom";
import { downloadExport } from "@/api/admin";
import { Heading } from "@/components/common/Heading";
import { Modal } from "@/components/common/Modal";
import { Pagination } from "@/components/common/Pagination";
import { Table } from "@/components/common/Table";
import { FileRow } from "@/components/files/FileRow";
import { ArtifactMetadataModal } from "@/features/review/ArtifactMetadataModal";
import { FileMetadataModal } from "@/features/review/FileMetadataModal";
import { useData } from "@/hooks/useData";
import type { Obj } from "@/types/common";

function ArtifactCatalog({ api, refresh, notify, role }: any) {
	const nav = useNavigate();
	const loc = useLocation();
	const initialQuery = new URLSearchParams(loc.search).get("q") || "";
	const [query, setQuery] = useState(initialQuery);
	const [page, setPage] = useState(1);
	const [limit, setLimit] = useState(20);
	const [createOpen, setCreateOpen] = useState(false);
	const [busy, setBusy] = useState(false);
	const [form, setForm] = useState({
		title: "",
		creator_name: "",
		source_url: "",
		description: "",
	});
	const path = `/catalog/artifacts?page=${page}&limit=${limit}${query.trim() ? "&q=" + encodeURIComponent(query.trim()) : ""}`;
	const data = useData(api, path, refresh);
	const items: Obj[] = data.data.items || [];
	const total = Number(data.data.total || 0);

	async function createArtifact(event: React.FormEvent) {
		event.preventDefault();
		setBusy(true);
		try {
			const artifact = await api("/catalog/artifacts", {
				method: "POST",
				body: JSON.stringify(form),
			});
			notify("作品を登録しました");
			nav(`/admin/artifacts/${encodeURIComponent(artifact.id)}`);
		} catch (error) {
			notify((error as Error).message);
		} finally {
			setBusy(false);
		}
	}

	return (
		<>
			<Heading
				eyebrow="CATALOG MANAGEMENT"
				title="作品一覧"
				desc="作品を選択すると、その作品に属するファイルと保全・公開状態をまとめて管理できます。"
				actions={
					<>
						<button
							className="secondary"
							type="button"
							onClick={() => downloadExport("csv", notify)}
						>
							<Download size={16} /> CSV出力
						</button>
						{role === "admin" && (
							<button
								className="primary"
								type="button"
								onClick={() => setCreateOpen(true)}
							>
								<Plus size={16} /> 作品を登録
							</button>
						)}
					</>
				}
			/>
			<section className="panel artifact-catalog-toolbar">
				<div className="filter-bar file-filter-bar">
					<div className="search-box">
						<Search size={17} />
						<input
							value={query}
							onChange={(event) => {
								setQuery(event.target.value);
								setPage(1);
							}}
							placeholder="作品名・作者・作品ID・元ページURLで検索"
							aria-label="作品を検索"
						/>
					</div>
					<span className="result-count" aria-live="polite">
						{data.loading ? "読み込み中…" : `${total.toLocaleString()} 作品`}
					</span>
				</div>
			</section>
			{data.loading ? (
				<section className="panel empty">作品一覧を読み込んでいます…</section>
			) : data.error ? (
				<section className="panel policy-warning">
					作品一覧を取得できませんでした: {data.error}
				</section>
			) : items.length === 0 ? (
				<section className="panel empty">
					<FolderOpen size={28} />
					<b>
						{query ? "一致する作品がありません" : "登録された作品がありません"}
					</b>
					<span>
						{query
							? "検索語を変更してください。"
							: "作品を登録すると、ここからファイルを管理できます。"}
					</span>
					{query && (
						<button
							className="secondary"
							type="button"
							onClick={() => setQuery("")}
						>
							検索をクリア
						</button>
					)}
				</section>
			) : (
				<div className="artifact-catalog-grid">
					{items.map((item) => (
						<NavLink
							className="panel artifact-catalog-card"
							to={{
								pathname: `/admin/artifacts/${encodeURIComponent(item.id)}`,
								search: query.trim()
									? `?q=${encodeURIComponent(query.trim())}`
									: "",
							}}
							key={item.id}
						>
							<div className="artifact-catalog-card-head">
								<span className="artifact-catalog-icon">
									<FolderOpen size={19} />
								</span>
								<span className="artifact-catalog-count">
									<Files size={14} />{" "}
									{Number(item.file_count || 0).toLocaleString()} ファイル
								</span>
							</div>
							<div className="artifact-catalog-card-body">
								<b>{item.title || "名称未設定"}</b>
								<span>{item.creator_name || "作者不明"}</span>
								{item.description && <p>{item.description}</p>}
								<small>
									<code>{item.id}</code>
								</small>
							</div>
							<div className="artifact-catalog-card-footer">
								<span>作品内のファイルを管理</span>
								<ArrowUpRight size={17} />
							</div>
						</NavLink>
					))}
				</div>
			)}
			<Pagination
				page={page}
				limit={limit}
				total={total}
				loading={data.loading}
				onPageChange={setPage}
				onLimitChange={(nextLimit) => {
					setLimit(nextLimit);
					setPage(1);
				}}
			/>
			{createOpen && (
				<Modal title="作品を登録" close={() => setCreateOpen(false)}>
					<form className="form-stack" onSubmit={createArtifact}>
						<label>
							作品名 *
							<input
								required
								value={form.title}
								onChange={(event) =>
									setForm({ ...form, title: event.target.value })
								}
							/>
						</label>
						<label>
							作者・配布者
							<input
								value={form.creator_name}
								onChange={(event) =>
									setForm({ ...form, creator_name: event.target.value })
								}
							/>
						</label>
						<label>
							元ページURL *
							<input
								required
								type="url"
								placeholder="https://www.uploader.jp/..."
								value={form.source_url}
								onChange={(event) =>
									setForm({ ...form, source_url: event.target.value })
								}
							/>
						</label>
						<label>
							説明
							<textarea
								rows={3}
								value={form.description}
								onChange={(event) =>
									setForm({ ...form, description: event.target.value })
								}
							/>
						</label>
						<div className="modal-actions">
							<button
								type="button"
								className="secondary"
								onClick={() => setCreateOpen(false)}
							>
								キャンセル
							</button>
							<button type="button" className="primary" disabled={busy}>
								{busy ? "登録中…" : "登録してファイルを追加"}
							</button>
						</div>
					</form>
				</Modal>
			)}
		</>
	);
}

export function Catalog({ api, refresh, notify, role }: any) {
	const [unused, setRefreshDummy] = useState(0);
	const loc = useLocation();
	const nav = useNavigate();
	const routeParams = useParams();
	const params = new URLSearchParams(loc.search);
	const linkedFileID = params.get("file_id") || "";
	const linkedArtifactID =
		routeParams.artifactID || params.get("artifact_id") || "";
	const [query, setQuery] = useState(params.get("q") || "");
	const [preservationFilter, setPreservationFilter] = useState("");
	const [policyFilter, setPolicyFilter] = useState("");
	const [publicationFilter, setPublicationFilter] = useState("");
	const [selectedIDs, setSelectedIDs] = useState<string[]>([]);
	const [bulkPolicy, setBulkPolicy] = useState("restricted");
	const [modal, setModal] = useState(false);
	const [detail, setDetail] = useState<Obj | null>(null);
	const [metadata, setMetadata] = useState<Obj | null>(null);
	const [artifact, setArtifact] = useState<Obj | null>(null);
	const [artifactRecord, setArtifactRecord] = useState<Obj | null>(null);
	const [artifactMetadataOpen, setArtifactMetadataOpen] = useState(false);
	const [pendingFileId, setPendingFileId] = useState("");
	const [busy, setBusy] = useState(false);
	const [upload, setUpload] = useState<File | null>(null);
	const fileInputRef = useRef<HTMLInputElement | null>(null);
	const [sourceURL, setSourceURL] = useState("");
	const [fileDescription, setFileDescription] = useState("");
	const [acceptSourceTerms, setAcceptSourceTerms] = useState(false);
	const [listingURL, setListingURL] = useState("");
	const [acceptListingTerms, setAcceptListingTerms] = useState(false);
	const [discovering, setDiscovering] = useState(false);
	const [candidates, setCandidates] = useState<Obj[]>([]);
	const [selectedCandidates, setSelectedCandidates] = useState<string[]>([]);
	const [batchBusy, setBatchBusy] = useState(false);
	const [batchStatus, setBatchStatus] = useState("");
	const [priority, setPriority] = useState("3");
	const [policy, setPolicy] = useState("pending");
	const [pub, setPub] = useState(false);
	const [form, setForm] = useState({
		title: "",
		creator_name: "",
		source_url: "",
		description: "",
	});
	const [page, setPage] = useState(1);
	const [limit, setLimit] = useState(20);
	const [reason, setReason] = useState("");

	let filePath = `/catalog/files?page=${page}&limit=${limit}`;
	if (query.trim()) filePath += "&q=" + encodeURIComponent(query.trim());
	if (preservationFilter)
		filePath +=
			"&preservation_status=" + encodeURIComponent(preservationFilter);
	if (policyFilter)
		filePath += "&redistribution_policy=" + encodeURIComponent(policyFilter);
	if (publicationFilter)
		filePath += "&publication=" + encodeURIComponent(publicationFilter);
	if (linkedArtifactID)
		filePath += "&artifact_id=" + encodeURIComponent(linkedArtifactID);

	const f = useData(api, filePath, refresh + unused);
	const files: Obj[] = f.data.items || [];
	const artifactData = useData(
		api,
		linkedArtifactID
			? `/catalog/artifacts/${encodeURIComponent(linkedArtifactID)}`
			: "/catalog/artifacts?limit=1",
		refresh + unused,
	);
	useEffect(() => {
		const fetchedArtifact = artifactData.data.artifact;
		if (
			linkedArtifactID &&
			fetchedArtifact &&
			String(fetchedArtifact.id) === String(linkedArtifactID)
		) {
			setArtifactRecord(fetchedArtifact);
		} else {
			setArtifactRecord(null);
		}
	}, [linkedArtifactID, artifactData.data.artifact]);

	useEffect(() => {
		if (role === "author" || !linkedFileID || f.loading) return;
		let live = true;
		const local = (f.data.items || []).find((x: Obj) => x.id === linkedFileID);
		(local
			? Promise.resolve(local)
			: api(`/catalog/files/${encodeURIComponent(linkedFileID)}`).then(
					(result: Obj) => result.file,
				)
		)
			.then((target: Obj) => {
				if (!live || !target) return;
				setDetail(target);
				setPolicy(target.redistribution_policy || "unknown");
				setPub(!!target.public_download_enabled);
			})
			.catch((e: Error) => live && notify(e.message))
			.finally(() => live && nav("/admin/review?tab=files", { replace: true }));
		return () => {
			live = false;
		};
	}, [linkedFileID, role, f.loading, f.data.items, api, notify, nav]);

	function resetAddFileForm() {
		setPendingFileId("");
		setUpload(null);
		if (fileInputRef.current) fileInputRef.current.value = "";
		setSourceURL("");
		setFileDescription("");
		setAcceptSourceTerms(false);
		setListingURL("");
		setAcceptListingTerms(false);
		setDiscovering(false);
		setCandidates([]);
		setSelectedCandidates([]);
		setBatchStatus("");
		setPriority("3");
		setPolicy("pending");
	}

	function openAddFile(targetArtifact: Obj) {
		resetAddFileForm();
		setArtifact(targetArtifact);
	}

	function closeAddFile() {
		setArtifact(null);
		resetAddFileForm();
	}

	// biome-ignore lint/correctness/useExhaustiveDependencies: Reset selection whenever the artifact context changes.
	useEffect(() => {
		setSelectedIDs([]);
		setPage(1);
	}, [linkedArtifactID]);

	function openMetadata(x: Obj) {
		setMetadata(x);
	}

	async function runBulk(action: "policy" | "publication", extra: Obj = {}) {
		if (selectedIDs.length === 0) {
			notify("先にファイルを選択してください");
			return;
		}
		try {
			if (role !== "author" && action === "policy") {
				const tmpReason = window.prompt("変更理由を入力");
				if (tmpReason?.trim() == null) throw Error("変更理由は必須です");
				setReason(tmpReason);
			}
			const body: Obj = {
				file_ids: selectedIDs,
				action,
				reason: reason.trim(),
				...extra,
			};
			const result = await api("/catalog/files/bulk-action", {
				method: "POST",
				body: JSON.stringify(body),
			});
			notify(`${result.count} 件を一括処理しました`);
			setSelectedIDs([]);
			setRefreshDummy((x) => x + 1);
		} catch (e) {
			notify((e as Error).message);
		}
	}
	async function deleteSelectedFiles() {
		if (selectedIDs.length === 0) return;
		if (
			!window.confirm(
				`選択した ${selectedIDs.length} 件のファイルを論理削除します。続行しますか？`,
			)
		)
			return;
		setBusy(true);
		const failures: string[] = [];
		try {
			for (const id of selectedIDs) {
				try {
					await api(`/catalog/files/${encodeURIComponent(id)}`, {
						method: "DELETE",
					});
				} catch (error) {
					failures.push(`${id}: ${(error as Error).message}`);
				}
			}
			setSelectedIDs([]);
			setRefreshDummy((x) => x + 1);
			notify(
				failures.length
					? `${selectedIDs.length - failures.length} 件を削除、${failures.length} 件失敗しました`
					: `${selectedIDs.length} 件を削除しました`,
			);
		} finally {
			setBusy(false);
		}
	}

	async function deleteSingleFile(id: string) {
		if (!window.confirm("このファイルを論理削除しますか？")) return;
		try {
			await api(`/catalog/files/${encodeURIComponent(id)}`, {
				method: "DELETE",
			});
			setSelectedIDs((current) =>
				current.filter((selectedID) => selectedID !== id),
			);
			notify("ファイルを削除しました");
			setRefreshDummy((x) => x + 1);
		} catch (error) {
			notify((error as Error).message);
		}
	}

	function toggleSelected(id: string, checked: boolean) {
		setSelectedIDs((old) =>
			checked
				? old.includes(id)
					? old
					: [...old, id]
				: old.filter((x) => x !== id),
		);
	}
	function toggleAll(checked: boolean) {
		const selectable = files
			.filter((x: Obj) => !x.deletion_requested_at && !x.deleted_at)
			.map((x: Obj) => x.id);
		setSelectedIDs((old) =>
			checked
				? Array.from(new Set([...old, ...selectable]))
				: old.filter((id) => !selectable.includes(id)),
		);
	}

	async function create(e: React.FormEvent) {
		e.preventDefault();
		setBusy(true);
		try {
			const a = await api("/catalog/artifacts", {
				method: "POST",
				body: JSON.stringify(form),
			});
			notify("作品を登録しました");
			setModal(false);
			setForm({ title: "", creator_name: "", source_url: "", description: "" });
			openAddFile(a);
			setRefreshDummy((x) => x + 1);
		} catch (e) {
			notify((e as Error).message);
		} finally {
			setBusy(false);
		}
	}
	async function addFile(e: React.FormEvent) {
		e.preventDefault();
		const url = sourceURL.trim();
		if (
			!artifact ||
			(!upload && !url) ||
			(upload && url) ||
			(url && !acceptSourceTerms) ||
			policy === "pending"
		)
			return;
		setBusy(true);
		try {
			let fileId = pendingFileId;
			if (!fileId) {
				const file = await api(`/catalog/artifacts/${artifact.id}/files`, {
					method: "POST",
					body: JSON.stringify({
						original_filename: upload?.name || "取得待ち",
						size_bytes: upload?.size || 0,
						media_type: upload?.type || "application/octet-stream",
						source_url: url || undefined,
						description: fileDescription,
						redistribution_policy: policy,
						public_download_enabled: false,
					}),
				});
				fileId = file.id;
				setPendingFileId(fileId);
			}
			let ingestResult: any;
			if (upload) {
				const fd = new FormData();
				fd.append("file", upload);
				fd.append("priority", priority);
				ingestResult = await api(`/catalog/files/${fileId}/ingest`, {
					method: "POST",
					body: fd,
				});
			} else {
				ingestResult = await api(`/catalog/files/${fileId}/ingest-source`, {
					method: "POST",
					body: JSON.stringify({
						source_url: url,
						accept_terms: acceptSourceTerms,
						priority: Number(priority),
					}),
				});
			}
			const createdTasks = Array.isArray(ingestResult?.tasks)
				? ingestResult.tasks.length
				: 0;
			notify(
				ingestResult?.source_fetch_task
					? `取得タスクを ${ingestResult.source_fetch_task.node_name || "agent"} に登録しました。取得成功後に保存タスクが作成されます。`
					: createdTasks
						? `登録しました。保存タスク ${createdTasks} 件を作成しました。`
						: "登録しました。保全処理を受け付けました。",
			);
			closeAddFile();
			setRefreshDummy((x) => x + 1);
		} catch (e) {
			notify((e as Error).message);
		} finally {
			setBusy(false);
		}
	}
	async function discoverListing() {
		if (!artifact || !listingURL.trim() || !acceptListingTerms) return;
		setDiscovering(true);
		setCandidates([]);
		setSelectedCandidates([]);
		setBatchStatus("一覧を確認しています…");
		try {
			const result = await api(
				`/catalog/artifacts/${artifact.id}/discover-source-files`,
				{
					method: "POST",
					body: JSON.stringify({
						listing_url: listingURL.trim(),
						accept_terms: acceptListingTerms,
					}),
				},
			);
			const found = (result.items || []) as Obj[];
			setCandidates(found);
			setSelectedCandidates(found.map((x: Obj) => x.url));
			setBatchStatus(
				`${result.pages_scanned} ページから ${found.length} 件を検出しました。` +
					(result.truncated
						? "上限100件に達したため、残りは次回確認してください。"
						: ""),
			);
		} catch (e) {
			setBatchStatus((e as Error).message);
		} finally {
			setDiscovering(false);
		}
	}
	async function importSelected() {
		if (!artifact || selectedCandidates.length === 0 || policy === "pending")
			return;
		const chosen = candidates.filter((x: Obj) =>
			selectedCandidates.includes(x.url),
		);
		setBatchBusy(true);
		let ok = 0;
		let fetchTaskCount = 0;
		let taskCount = 0;
		const failures: string[] = [];
		for (let i = 0; i < chosen.length; i++) {
			const item = chosen[i];
			setBatchStatus(
				`取得タスク登録中 ${i + 1}/${chosen.length}: ${item.label || item.url}`,
			);
			try {
				const file = await api(`/catalog/artifacts/${artifact.id}/files`, {
					method: "POST",
					body: JSON.stringify({
						original_filename: (item.label || "取得待ち").slice(0, 500),
						size_bytes: 0,
						media_type: "application/octet-stream",
						source_url: item.url,
						redistribution_policy: policy,
					}),
				});
				const queued = await api(`/catalog/files/${file.id}/ingest-source`, {
					method: "POST",
					body: JSON.stringify({
						source_url: item.url,
						accept_terms: true,
						priority: Number(priority),
					}),
				});
				ok++;
				if (queued.source_fetch_task) fetchTaskCount++;
				taskCount += Array.isArray(queued?.tasks) ? queued.tasks.length : 0;
			} catch (e) {
				failures.push(`${item.label || item.url}: ${(e as Error).message}`);
			}
		}
		setBatchBusy(false);
		setSelectedCandidates([]);
		setCandidates([]);
		setBatchStatus(
			`完了: ${ok} 件受付、agent取得タスク ${fetchTaskCount} 件登録、保存タスク ${taskCount} 件作成、${failures.length} 件失敗。` +
				(fetchTaskCount
					? "保存タスクは各 agent の取得成功後に作成されます。 "
					: "") +
				(failures.length ? `最初の失敗: ${failures[0]}` : ""),
		);
		notify(
			`一括保全: ${ok} 件受付、agent取得タスク ${fetchTaskCount} 件登録、失敗 ${failures.length} 件`,
		);
		setRefreshDummy((x) => x + 1);
	}
	async function savePolicy() {
		if (!detail) return;
		const reason = window.prompt("変更理由を入力してください");
		if (!reason || reason.trim().length < 3) return;
		try {
			await api(`/catalog/files/${detail.id}/policy`, {
				method: "PATCH",
				body: JSON.stringify({
					redistribution_policy: policy,
					reason: reason.trim(),
				}),
			});
			if (policy !== "allowed") {
				// The API disables publication whenever redistribution is not allowed.
				setPub(false);
			} else if (role === "admin" || role === "author") {
				await api(`/catalog/files/${detail.id}/publication`, {
					method: "PATCH",
					body: JSON.stringify({ enabled: pub, reason: reason.trim() }),
				});
			}
			notify("公開ポリシーを更新しました");
			setDetail(null);
			setReason("");
			setRefreshDummy((x) => x + 1);
		} catch (e) {
			notify((e as Error).message);
		}
	}

	if (
		(role === "admin" || role === "reviewer") &&
		!linkedArtifactID &&
		!linkedFileID &&
		!detail &&
		!metadata &&
		!modal &&
		!artifact
	) {
		return (
			<ArtifactCatalog
				api={api}
				refresh={refresh + unused}
				notify={notify}
				role={role}
			/>
		);
	}

	return (
		<>
			<Heading
				eyebrow=""
				title={linkedArtifactID ? "作品内のファイル" : "作品・ファイル"}
				desc={
					role === "author"
						? "割り当てられた作品のファイルを確認・削除"
						: linkedArtifactID
							? "この作品に属するファイルの保全状態・公開条件を管理"
							: "作品を選択して、作品単位でファイルを管理"
				}
				actions={
					<>
						{linkedArtifactID && (
							<button
								className="secondary"
								onClick={() =>
									nav(
										role === "author"
											? "/admin/artifacts"
											: "/admin/review?tab=files",
									)
								}
								type="button"
							>
								作品一覧へ戻る
							</button>
						)}
						{role !== "author" && (
							<button
								type="button"
								className="secondary"
								onClick={() => downloadExport("csv", notify)}
							>
								<Download size={16} /> CSV出力
							</button>
						)}
						{linkedArtifactID && artifactRecord && role !== "viewer" && (
							<button
								className="secondary"
								type="button"
								disabled={busy}
								onClick={() => setArtifactMetadataOpen(true)}
							>
								作品情報を編集
							</button>
						)}
						{linkedArtifactID && role === "admin" && artifactRecord && (
							<button
								className="primary"
								onClick={() => openAddFile(artifactRecord)}
								type="button"
							>
								<Plus size={16} /> ファイルを追加
							</button>
						)}
						{role === "admin" && !linkedArtifactID && (
							<button
								className="primary"
								onClick={() => setModal(true)}
								type="button"
							>
								<Plus size={16} /> 作品を登録
							</button>
						)}
					</>
				}
			/>
			{(role === "admin" || role === "reviewer") &&
				linkedArtifactID &&
				artifactRecord && (
					<section
						className="panel related-requests-panel"
						aria-labelledby="related-preservation-requests"
					>
						<div className="panel-heading">
							<h2 id="related-preservation-requests">関連するリクエスト</h2>
							<span className="muted">
								{(artifactData.data.related_requests || []).length} 件
							</span>
						</div>
						{(artifactData.data.related_requests || []).length > 0 ? (
							<ul className="related-request-list">
								{(artifactData.data.related_requests || []).map(
									(request: Obj) => (
										<li key={request.id}>
											<a
												href={`/admin/requests/${encodeURIComponent(request.id)}`}
											>
												<strong>{request.title || "タイトル未設定"}</strong>
												<span>
													{request.status} ·{" "}
													{new Date(request.created_at).toLocaleString("ja-JP")}
												</span>
											</a>
										</li>
									),
								)}
							</ul>
						) : (
							<p className="muted">関連する保全リクエストはありません。</p>
						)}
					</section>
					)}
			<div className="filter-bar file-filter-bar">
				<div className="search-box">
					<Search size={17} />
					<input
						value={query}
						onChange={(e) => {
							setQuery(e.target.value);
							setPage(1);
						}}
						placeholder="作品名・ファイル名・作者・IDで検索"
					/>
				</div>
				<select
					aria-label="保全状態で絞り込み"
					value={preservationFilter}
					onChange={(e) => {
						setPreservationFilter(e.target.value);
						setPage(1);
					}}
				>
					<option value="">すべての保全状態</option>
					<option value="uncollected">未取得</option>
					<option value="queued">待機中</option>
					<option value="stored">保存済み</option>
					<option value="verified">検証済み</option>
					<option value="degraded">劣化</option>
					<option value="missing">実体不明</option>
					<option value="error">エラー</option>
				</select>
				<select
					aria-label="公開条件で絞り込み"
					value={policyFilter}
					onChange={(e) => {
						setPolicyFilter(e.target.value);
						setPage(1);
					}}
				>
					<option value="">すべての公開条件</option>
					{(role === "author"
						? [
								{ value: "allowed", label: "再配布可" },
								{ value: "restricted", label: "制限付き" },
							]
						: [
								{ value: "allowed", label: "再配布可" },
								{ value: "restricted", label: "制限付き" },
								{ value: "in_progress", label: "確認中" },
								{ value: "pending", label: "未確認" },
								{ value: "unknown", label: "不明" },
							]
					).map(({ value, label }) => (
						<option key={value} value={value}>
							{label}
						</option>
					))}
				</select>
				<select
					aria-label="公開状態で絞り込み"
					value={publicationFilter}
					onChange={(e) => {
						setPublicationFilter(e.target.value);
						setPage(1);
					}}
				>
					<option value="">公開状態：すべて</option>
					<option value="public">公開中</option>
					<option value="private">非公開</option>
					{role !== "author" && (
						<>
							<option value="restricted">制限付き</option>
							<option value="pending">未確認</option>
							<option value="unknown">条件不明</option>
						</>
					)}
				</select>
				<span className="result-count">{f.data.total ?? files.length} 件</span>
			</div>
			{selectedIDs.length > 0 && role === "author" && (
				<section className="bulk-toolbar panel">
					<div className="bulk-toolbar-title">
						<b>{selectedIDs.length} 件を選択中</b>
						<button
							type="button"
							className="small-btn"
							onClick={() => setSelectedIDs([])}
						>
							選択解除
						</button>
					</div>
					<button
						type="button"
						className="danger-btn"
						disabled={busy}
						onClick={deleteSelectedFiles}
					>
						選択したファイルを削除
					</button>
				</section>
			)}
			{selectedIDs.length > 0 && role !== "viewer" && role !== "author" && (
				<section className="bulk-toolbar panel">
					<div className="bulk-toolbar-title">
						<b>{selectedIDs.length} 件を選択中</b>
						<button
							type="button"
							className="small-btn"
							onClick={() => setSelectedIDs([])}
						>
							選択解除
						</button>
					</div>
					<div className="bulk-toolbar-controls">
						<label>
							公開条件
							<select
								value={bulkPolicy}
								onChange={(e) => setBulkPolicy(e.target.value)}
							>
								<option value="allowed">再配布可</option>
								<option value="restricted">制限付き</option>
								<option value="in_progress">確認中</option>
								<option value="pending">未確認</option>
								<option value="unknown">不明</option>
							</select>
						</label>
						<button
							type="button"
							className="secondary"
							onClick={() =>
								runBulk("policy", { redistribution_policy: bulkPolicy })
							}
						>
							公開条件を一括変更
						</button>
						{role === "admin" && (
							<>
								<button
									type="button"
									className="danger-btn"
									disabled={busy}
									onClick={deleteSelectedFiles}
								>
									選択したファイルを削除
								</button>
								<button
									type="button"
									className="primary"
									onClick={() => runBulk("publication", { enabled: true })}
								>
									一括公開
								</button>
								<button
									type="button"
									className="secondary"
									onClick={() => runBulk("publication", { enabled: false })}
								>
									一括公開停止
								</button>
							</>
						)}
					</div>
				</section>
			)}
			<section className="panel table-panel">
				<Table
					headers={[
						"",
						"作品 / ファイル",
						"保全状態",
						"公開条件 / 状態",
						"サイズ",
						"登録日時",
						"操作",
					]}
				>
					{f.loading ? (
						<tr>
							<td colSpan={7} className="empty">
								読み込み中…
							</td>
						</tr>
					) : f.error ? (
						<tr>
							<td colSpan={7} className="empty">
								{f.error}
							</td>
						</tr>
					) : files.length ? (
						<>
							<tr className="select-all-row">
								<td>
									<input
										type="checkbox"
										aria-label="表示中のファイルをすべて選択"
										checked={
											files.filter(
												(x: Obj) => !x.deletion_requested_at && !x.deleted_at,
											).length > 0 &&
											files
												.filter(
													(x: Obj) => !x.deletion_requested_at && !x.deleted_at,
												)
												.every((x: Obj) => selectedIDs.includes(x.id))
										}
										disabled={role === "viewer"}
										onChange={(e) => toggleAll(e.target.checked)}
									/>
								</td>
								<td colSpan={6} className="muted">
									表示中の {files.length} 件を選択
								</td>
							</tr>
							{files.map((x: Obj) => (
								<FileRow
									key={x.id}
									x={x}
									role={role}
									isSelected={selectedIDs.includes(x.id)}
									toggleSelected={toggleSelected}
									setDetail={setDetail}
									setPolicy={setPolicy}
									setPub={setPub}
									openMetadata={openMetadata}
									onDelete={() => deleteSingleFile(x.id)}
								/>
							))}
						</>
					) : (
						<tr>
							<td colSpan={7} className="empty">
								ファイルがありません
							</td>
						</tr>
					)}
				</Table>
			</section>
			<Pagination
				page={page}
				limit={limit}
				total={Number(f.data.total ?? 0)}
				loading={f.loading}
				onPageChange={setPage}
				onLimitChange={(nextLimit) => {
					setLimit(nextLimit);
					setPage(1);
				}}
			/>
			{modal && (
				<Modal title="作品を登録" close={() => setModal(false)}>
					<form onSubmit={create} className="form-stack">
						<label>
							作品名 *
							<input
								required
								value={form.title}
								onChange={(e) => setForm({ ...form, title: e.target.value })}
							/>
						</label>
						<label>
							作者・配布者
							<input
								value={form.creator_name}
								onChange={(e) =>
									setForm({ ...form, creator_name: e.target.value })
								}
							/>
						</label>
						<label>
							元ページURL *
							<input
								required
								type="url"
								placeholder="https://www.uploader.jp/..."
								value={form.source_url}
								onChange={(e) =>
									setForm({ ...form, source_url: e.target.value })
								}
							/>
						</label>
						<label>
							説明
							<textarea
								rows={3}
								value={form.description}
								onChange={(e) =>
									setForm({ ...form, description: e.target.value })
								}
							/>
						</label>
						<div className="modal-actions">
							<button
								type="button"
								className="secondary"
								onClick={() => setModal(false)}
							>
								キャンセル
							</button>
							<button type="button" className="primary" disabled={busy}>
								{busy ? "登録中…" : "次へ"}
							</button>
						</div>
					</form>
				</Modal>
			)}
			{artifact && (
				<Modal title="ファイルを追加・保全" close={closeAddFile}>
					<form className="form-stack" onSubmit={addFile}>
						<p className="muted">
							作品: <b>{artifact.title}</b>
						</p>
						<label>
							手元のファイル
							<input
								ref={fileInputRef}
								type="file"
								onChange={(e) => {
									setUpload(e.target.files?.[0] || null);
									if (e.target.files?.[0]) {
										setSourceURL("");
										setAcceptSourceTerms(false);
										setListingURL("");
										setAcceptListingTerms(false);
										setCandidates([]);
										setSelectedCandidates([]);
										setPendingFileId("");
									}
								}}
							/>
							<small>最大5 GiB。保存先は2ノード必要です。</small>
						</label>
						<label>
							または uploader.jp のダウンロードURL
							<input
								type="url"
								value={sourceURL}
								onChange={(e) => {
									setSourceURL(e.target.value);
									if (e.target.value.trim()) {
										setUpload(null);
										setListingURL("");
										setAcceptListingTerms(false);
										setCandidates([]);
										setSelectedCandidates([]);
										if (fileInputRef.current) fileInputRef.current.value = "";
									} else setPendingFileId("");
								}}
								placeholder="https://uu.getuploader.com/名前/download/123"
							/>
							<small>
								個別ファイル用。パスワードや画像認証の回避は行いません。
							</small>
						</label>
						{sourceURL.trim() && (
							<label className="check-label">
								<input
									type="checkbox"
									checked={acceptSourceTerms}
									onChange={(e) => setAcceptSourceTerms(e.target.checked)}
								/>{" "}
								uploader.jp の利用規約を確認し、同意しました
							</label>
						)}
						<div className="form-divider" />
						<label>
							またはアップローダーの一覧URL
							<input
								type="url"
								value={listingURL}
								onChange={(e) => {
									setListingURL(e.target.value);
									if (e.target.value.trim()) {
										setSourceURL("");
										setAcceptSourceTerms(false);
										setUpload(null);
										setPendingFileId("");
										if (fileInputRef.current) fileInputRef.current.value = "";
									}
									setCandidates([]);
									setSelectedCandidates([]);
									setBatchStatus("");
								}}
								placeholder="https://uu.getuploader.com/名前/"
							/>
							<small>
								一覧ページを調べ、掲載ファイルを選んで順番に取得します（最大100件・最大10ページ）。
							</small>
						</label>
						{listingURL.trim() && (
							<label className="check-label">
								<input
									type="checkbox"
									checked={acceptListingTerms}
									onChange={(e) => setAcceptListingTerms(e.target.checked)}
								/>{" "}
								uploader.jp の利用規約を確認しました
							</label>
						)}
						<div className="modal-actions">
							<button
								type="button"
								className="secondary"
								disabled={
									!listingURL.trim() ||
									!acceptListingTerms ||
									discovering ||
									batchBusy
								}
								onClick={discoverListing}
							>
								{discovering ? "確認中…" : "一覧を調べる"}
							</button>
						</div>
						{batchStatus && <p className="muted">{batchStatus}</p>}
						{candidates.length > 0 && (
							<div className="candidate-list">
								<div className="candidate-list-head">
									<b>検出したファイル ({candidates.length})</b>
									<button
										type="button"
										className="small-btn"
										onClick={() =>
											setSelectedCandidates(
												selectedCandidates.length === candidates.length
													? []
													: candidates.map((x: Obj) => x.url),
											)
										}
									>
										{selectedCandidates.length === candidates.length
											? "選択解除"
											: "全件選択"}
									</button>
								</div>
								{candidates.map((x: Obj) => (
									<label className="candidate-row" key={x.url}>
										<input
											type="checkbox"
											checked={selectedCandidates.includes(x.url)}
											onChange={(e) =>
												setSelectedCandidates(
													e.target.checked
														? [...selectedCandidates, x.url]
														: selectedCandidates.filter((u) => u !== x.url),
												)
											}
										/>
										<span>
											{x.label || x.url}
											<small>{x.url}</small>
										</span>
									</label>
								))}
								<button
									type="button"
									className="primary wide"
									disabled={
										batchBusy ||
										selectedCandidates.length === 0 ||
										policy === "pending"
									}
									onClick={importSelected}
								>
									{batchBusy
										? "取得タスク登録中…"
										: `選択した ${selectedCandidates.length} 件の取得タスクを登録`}
								</button>
							</div>
						)}
						{!listingURL.trim() && (
							<label>
								ファイルの説明
								<textarea
									rows={3}
									value={fileDescription}
									disabled={busy || !!pendingFileId}
									onChange={(e) => setFileDescription(e.target.value)}
									placeholder="このファイルの内容や用途など"
								/>
							</label>
						)}
						<label>
							公開条件
							<select
								value={policy}
								disabled={busy || batchBusy}
								onChange={(e) => setPolicy(e.target.value)}
							>
								<option value="pending">未確認（保全不可）</option>
								<option value="unknown">不明</option>
								<option value="in_progress">確認中</option>
								<option value="allowed">許可済み</option>
								<option value="restricted">制限あり</option>
							</select>
						</label>
						<p className="muted">
							保全と一般公開は別の判定です。公開条件が未確認のファイルは保全を開始できません。
						</p>
						<label>
							保存優先度（0〜5）
							<select
								value={priority}
								onChange={(e) => setPriority(e.target.value)}
							>
								{[5, 4, 3, 2, 1, 0].map((x) => (
									<option key={x} value={x}>
										{x}
									</option>
								))}
							</select>
						</label>
						<div className="modal-actions">
							<button
								type="button"
								className="secondary"
								onClick={() => {
									closeAddFile();
								}}
							>
								キャンセル
							</button>
							<button
								type="button"
								className="primary"
								disabled={
									busy ||
									(!upload && !sourceURL.trim()) ||
									(!!upload && !!sourceURL.trim()) ||
									(!!sourceURL.trim() && !acceptSourceTerms) ||
									policy === "pending"
								}
							>
								<CloudUpload size={16} />
								{busy
									? "処理中…"
									: sourceURL.trim()
										? "取得して保全開始"
										: "登録して保全開始"}
							</button>
						</div>
					</form>
				</Modal>
			)}
			{detail && (
				<Modal title="公開設定" close={() => setDetail(null)}>
					<div className="detail-summary">
						<b>{detail.artifact_title || detail.original_filename}</b>
						<small>{detail.original_filename}</small>
						<small>{detail.id}</small>
						{detail.source_url && (
							<small>
								<a href={detail.source_url} target="_blank" rel="noreferrer">
									取得元URLを開く
								</a>
							</small>
						)}
						{detail.deleted_at && (
							<div className="policy-warning">
								<LockKeyhole size={16} /> 削除済みのファイルです。
							</div>
						)}
						{detail.deletion_requested_at && !detail.deleted_at && (
							<div className="policy-warning">
								<LockKeyhole size={16} /> ノードへの削除タスクを処理中です。
							</div>
						)}
						<div className="policy-warning">
							<LockKeyhole size={16} /> 公開は権利確認後に有効にしてください。
						</div>
					</div>
					<div className="form-stack">
						<label>
							公開条件
							<select
								value={policy}
								onChange={(e) => setPolicy(e.target.value)}
							>
								{role === "author" &&
									!["allowed", "restricted"].includes(policy) && (
										<option value={policy} disabled>
											{policy === "unknown"
												? "不明"
												: policy === "pending"
													? "未確認"
													: policy === "in_progress"
														? "確認中"
														: "現在の設定"}
										</option>
									)}
								<option value="allowed">再配布可</option>
								<option value="restricted">制限付き</option>
								{role !== "author" && (
									<>
										<option value="unknown">不明</option>
										<option value="pending">未確認（保全不可）</option>
										<option value="in_progress">確認中</option>
									</>
								)}
							</select>
						</label>
						<label className="check-label">
							<input
								type="checkbox"
								checked={pub}
								disabled={role !== "admin" && role !== "author"}
								onChange={(e) => setPub(e.target.checked)}
							/>{" "}
							一般公開する
						</label>
						<p className="muted">
							公開には再配布可の確認、検証済み状態、オンライン保存先が必要です。
						</p>
						<label>
							{`変更理由${role !== "author" ? "(任意)" : null}`}
							<input
								type={"text"}
								value={reason}
								required={role !== "author"}
								onChange={(e) => setReason(e.target.value)}
							/>
						</label>
						<div className="modal-actions">
							<button
								type="button"
								className="secondary"
								onClick={() => setDetail(null)}
							>
								閉じる
							</button>
							<button
								type="button"
								className="primary"
								onClick={savePolicy}
								disabled={
									!!detail.deleted_at ||
									!!detail.deletion_requested_at ||
									(role === "author" &&
										!["allowed", "restricted"].includes(policy))
								}
							>
								設定を保存
							</button>
						</div>
					</div>
				</Modal>
			)}
			{metadata && (
				<FileMetadataModal
					file={metadata}
					api={api}
					notify={notify}
					busy={busy}
					setBusy={setBusy}
					close={() => {
						if (busy) return;
						setMetadata(null);
					}}
					onSaved={() => {
						setMetadata(null);
						setRefreshDummy((x) => x + 1);
					}}
					isAuthor={role === "author"}
				/>
			)}
			{artifactMetadataOpen && artifactRecord && (
				<ArtifactMetadataModal
					artifact={artifactRecord}
					api={api}
					notify={notify}
					busy={busy}
					setBusy={setBusy}
					close={() => {
						if (busy) return;
						setArtifactMetadataOpen(false);
					}}
					onSaved={() => {
						setArtifactMetadataOpen(false);
						setRefreshDummy((x) => x + 1);
					}}
					isAuthor={role === "author"}
				/>
			)}
		</>
	);
}
