import { Check, RefreshCw, Search, Users } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Heading } from "@/components/common/Heading";
import { useData } from "@/hooks/useData";
import type { Obj } from "@/types/common";

export function AuthorAssignmentsPage({ api, refresh, notify }: any) {
	const users = useData(api, "/admin/users", refresh);
	const artifacts = useData(api, "/catalog/artifacts?all=true", refresh);
	const authors: Obj[] = (users.data.items || []).filter(
		(user: Obj) => user.role === "author",
	);
	const [authorID, setAuthorID] = useState("");
	const [assignedIDs, setAssignedIDs] = useState<string[]>([]);
	const [search, setSearch] = useState("");
	const [loadingAssignments, setLoadingAssignments] = useState(false);
	const [saving, setSaving] = useState(false);
	const [assignmentError, setAssignmentError] = useState("");

	useEffect(() => {
		if (!authors.length) {
			setAuthorID("");
			setAssignedIDs([]);
			return;
		}
		if (!authorID || !authors.some((author) => author.id === authorID)) {
			setAuthorID(authors[0].id);
		}
	}, [users.data.items, authorID]);

	useEffect(() => {
		if (!authorID) return;
		let active = true;
		setLoadingAssignments(true);
		setAssignedIDs([]);
		setAssignmentError("");
		api(`/admin/author-assignments/${encodeURIComponent(authorID)}`)
			.then((result: Obj) => {
				if (active) setAssignedIDs(result.artifact_ids || []);
			})
			.catch((error: Error) => {
				if (active) setAssignmentError(error.message);
			})
			.finally(() => active && setLoadingAssignments(false));
		return () => {
			active = false;
		};
	}, [authorID, refresh]);

	const filteredArtifacts = useMemo(() => {
		const query = search.trim().toLocaleLowerCase();
		const items: Obj[] = artifacts.data.items || [];
		if (!query) return items;
		return items.filter((artifact) =>
			[
				String(artifact.title || ""),
				String(artifact.creator_name || ""),
				String(artifact.id || ""),
			]
				.join(" ")
				.toLocaleLowerCase()
				.includes(query),
		);
	}, [artifacts.data.items, search]);

	function toggleArtifact(id: string, checked: boolean) {
		setAssignedIDs((current) =>
			checked
				? current.includes(id)
					? current
					: [...current, id]
				: current.filter((item) => item !== id),
		);
	}

	async function save() {
		if (!authorID || saving || loadingAssignments || assignmentError) return;
		setSaving(true);
		try {
			await api(`/admin/author-assignments/${encodeURIComponent(authorID)}`, {
				method: "PUT",
				body: JSON.stringify({ artifact_ids: assignedIDs }),
			});
			notify("作品の割り当てを保存しました");
		} catch (error) {
			notify(
				error instanceof Error
					? error.message
					: "割り当てを保存できませんでした",
			);
		} finally {
			setSaving(false);
		}
	}

	const selectedAuthor = authors.find((author) => author.id === authorID);

	return (
		<>
			<Heading
				eyebrow="ADMINISTRATION"
				title="Author の作品割り当て"
				desc="Author が閲覧・ファイル削除できる作品を管理します。"
				actions={
					<button
						className="primary"
						onClick={save}
						disabled={
							!authorID ||
							saving ||
							loadingAssignments ||
							!!assignmentError ||
							artifacts.loading ||
							!!artifacts.error
						}
					>
						<Check size={16} /> {saving ? "保存中…" : "割り当てを保存"}
					</button>
				}
			/>

			{users.error && (
				<div className="policy-warning">
					ユーザー一覧を取得できませんでした: {users.error}
				</div>
			)}
			{artifacts.error && (
				<div className="policy-warning">
					作品一覧を取得できませんでした: {artifacts.error}
				</div>
			)}

			<section className="panel form-stack">
				<label>
					Author
					<select
						value={authorID}
						onChange={(event) => setAuthorID(event.target.value)}
						disabled={users.loading || authors.length === 0}
					>
						{authors.length === 0 ? (
							<option value="">Author がいません</option>
						) : (
							authors.map((author) => (
								<option key={author.id} value={author.id}>
									{author.display_name || author.email}（{author.email}）
								</option>
							))
						)}
					</select>
				</label>
				{selectedAuthor && (
					<div className="info-card">
						<Users size={18} />
						<div>
							<b>{selectedAuthor.display_name || selectedAuthor.email}</b>
							<p>{selectedAuthor.email}</p>
						</div>
						<span className="result-count">
							{assignedIDs.length} 作品を割り当て中
						</span>
					</div>
				)}
			</section>

			<section className="panel table-panel">
				<div className="panel-head">
					<div>
						<h2>割り当て可能な作品</h2>
						<p>チェックした作品が Author の操作対象になります。</p>
					</div>
					<div className="search-box">
						<Search size={17} />
						<input
							value={search}
							onChange={(event) => setSearch(event.target.value)}
							placeholder="作品名・作者・IDで検索"
						/>
					</div>
				</div>
				{loadingAssignments || artifacts.loading || users.loading ? (
					<div className="empty">読み込み中…</div>
				) : assignmentError ? (
					<div className="policy-warning">
						割り当てを取得できませんでした: {assignmentError}
					</div>
				) : authors.length === 0 ? (
					<div className="empty">
						Author ロールのユーザーがいません。先にユーザー管理から Author
						を作成してください。
					</div>
				) : filteredArtifacts.length === 0 ? (
					<div className="empty">該当する作品がありません。</div>
				) : (
					<div className="form-stack" style={{ padding: 20 }}>
						{filteredArtifacts.map((artifact: Obj) => (
							<label className="check-label" key={artifact.id}>
								<input
									type="checkbox"
									checked={assignedIDs.includes(artifact.id)}
									onChange={(event) =>
										toggleArtifact(artifact.id, event.target.checked)
									}
								/>
								<span>
									<b>{artifact.title || "名称未設定"}</b>
									<small className="muted" style={{ display: "block" }}>
										{artifact.creator_name || "作者不明"} · {artifact.id}
									</small>
								</span>
							</label>
						))}
					</div>
				)}
				<div className="panel-foot">
					<span>
						選択中: <b>{assignedIDs.length}</b> 作品
					</span>
					<button
						className="secondary"
						type="button"
						onClick={() => setAssignedIDs([])}
						disabled={!assignedIDs.length || saving}
					>
						<RefreshCw size={15} /> すべて解除
					</button>
				</div>
			</section>
		</>
	);
}
