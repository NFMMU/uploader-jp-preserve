import { LockKeyhole, RefreshCw } from "lucide-react";
import { useState } from "react";
import CopyEmail from "../../CopyEmail";
import { Modal } from "../../components/common/Modal";
import { Status } from "../../components/common/Status";
import { Table } from "../../components/common/Table";
import { useData } from "../../hooks/useData";
import { bytes, date } from "../../lib/format";
import type { Obj } from "../../types/common";
import { ConversationLinks } from "../review/ConversationLinks";

export function NodeEnrollmentRequests({ api, refresh, notify }: any) {
	const [localRefresh, setLocalRefresh] = useState(0);
	const d = useData(
		api,
		"/admin/node-enrollment-requests",
		refresh + localRefresh,
	);
	const [busyID, setBusyID] = useState("");
	const [issued, setIssued] = useState<Obj | null>(null);
	const [conversationTarget, setConversationTarget] = useState<Obj | null>(
		null,
	);
	async function review(item: Obj, status: "approved" | "rejected") {
		if (
			status === "rejected" &&
			!window.confirm(`「${item.name}」の参加申請を却下しますか？`)
		)
			return;
		if (
			status === "approved" &&
			!window.confirm(
				`「${item.name}」を承認し、一回限りの登録トークンを発行しますか？トークンはこの画面で一度だけ表示されます。`,
			)
		)
			return;
		setBusyID(item.id);
		try {
			const result = await api(`/admin/node-enrollment-requests/${item.id}`, {
				method: "PATCH",
				body: JSON.stringify({ status }),
			});
			if (status === "approved")
				setIssued({
					name: item.name,
					token: result.enrollment_token,
					expires_at: result.expires_at,
				});
			else notify("参加申請を却下しました");
			setLocalRefresh((x) => x + 1);
		} catch (e) {
			notify((e as Error).message);
		} finally {
			setBusyID("");
		}
	}
	const items = d.data.items || [];
	return (
		<>
			<section className="panel table-panel">
				<div className="panel-head">
					<div>
						<h2>ストレージノード参加申請</h2>
						<p>申請を確認し、承認時に申請ごとの一回限りトークンを発行</p>
					</div>
					<button
						className="secondary"
						onClick={() => setLocalRefresh((x) => x + 1)}
						type="button"
					>
						<RefreshCw size={14} /> 更新
					</button>
				</div>
				<Table
					headers={[
						"申請日時",
						"ノード",
						"種別",
						"申告容量",
						"連絡先",
						"補足",
						"状態",
						"操作",
					]}
				>
					{d.loading ? (
						<tr>
							<td colSpan={8} className="empty">
								読み込み中…
							</td>
						</tr>
					) : d.error ? (
						<tr>
							<td colSpan={8} className="empty">
								{d.error}
							</td>
						</tr>
					) : items.length ? (
						items.map((x: Obj) => (
							<tr key={x.id}>
								<td>{date(x.created_at)}</td>
								<td>
									<b>{x.name}</b>
									<small className="cell-sub">
										<code>{String(x.id).slice(0, 8)}</code>
									</small>
								</td>
								<td>{x.node_type}</td>
								<td>
									{x.requested_capacity_bytes === 0
										? "上限なし"
										: bytes(x.requested_capacity_bytes)}
								</td>
								<td>
									<a href={`mailto:${x.contact_email}`}>{x.contact_email}</a>{" "}
									<CopyEmail email={String(x.contact_email || "")} />
								</td>
								<td className="error-cell" title={x.message || ""}>
									{x.message || "—"}
								</td>
								<td>
									<Status value={x.status} />
									{x.status === "approved" && x.token_expires_at && (
										<small className="cell-sub">
											期限 {date(x.token_expires_at)}
										</small>
									)}
								</td>
								<td>
									<div className="row-actions">
										{(x.status === "pending" || x.status === "approved") && (
											<>
												<button
													className="small-btn"
													disabled={!!busyID}
													onClick={() => review(x, "approved")}
													type="button"
												>
													{x.status === "approved" ? "再発行" : "承認・発行"}
												</button>
												<button
													className="small-btn"
													disabled={!!busyID}
													onClick={() => review(x, "rejected")}
													type="button"
												>
													却下
												</button>
											</>
										)}
										{x.status === "enrolled" && <small>登録済み</small>}
										<button
											className="small-btn"
											onClick={() => setConversationTarget(x)}
											type="button"
										>
											連絡・会話URL
										</button>
									</div>
								</td>
							</tr>
						))
					) : (
						<tr>
							<td colSpan={8} className="empty">
								参加申請はありません
							</td>
						</tr>
					)}
				</Table>
			</section>
			{conversationTarget && (
				<Modal
					title={`連絡・会話URL：${conversationTarget.name}`}
					close={() => setConversationTarget(null)}
				>
					<ConversationLinks
						api={api}
						refresh={refresh}
						notify={notify}
						targetType="node_enrollment_request"
						targetId={conversationTarget.id}
						role="admin"
					/>
				</Modal>
			)}
			{issued && (
				<Modal title="発行した登録トークン" close={() => setIssued(null)}>
					<p>{issued.name} 向け。参加者へ安全な方法で共有してください。</p>
					<div className="issued-token-body">
						<p className="policy-warning">
							<LockKeyhole size={16} />{" "}
							このトークンは再表示できません。画面を閉じる前にコピーしてください。
						</p>
						<code className="issued-token-value">{issued.token}</code>
						<div className="modal-actions">
							<button
								className="secondary"
								onClick={async () => {
									try {
										if (!navigator.clipboard)
											throw new Error("clipboard unavailable");
										await navigator.clipboard.writeText(String(issued.token));
										notify("トークンをコピーしました");
									} catch {
										notify(
											"コピーできませんでした。トークンを手動でコピーしてください。",
										);
									}
								}}
							>
								トークンをコピー
							</button>
							<button className="primary" onClick={() => setIssued(null)}>
								共有済み・閉じる
							</button>
						</div>
						<p className="muted">
							有効期限：{date(issued.expires_at)}
							<br />
							初回登録後は再利用できません。
						</p>
					</div>
				</Modal>
			)}
		</>
	);
}
