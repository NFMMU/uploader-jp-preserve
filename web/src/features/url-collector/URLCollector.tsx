import {
	CheckCircle2,
	ClipboardList,
	Globe,
	Search,
	ShieldCheck,
} from "lucide-react";
import type React from "react";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Heading } from "../../components/common/Heading";
import { Stat } from "../../components/common/Stat";
import { Table } from "../../components/common/Table";
import type { Obj } from "../../types/common";

export function URLCollector({ api, refresh, notify, role, bumpRefresh }: any) {
	const [searchTerms, setSearchTerms] = useState("");
	const [acceptTerms, setAcceptTerms] = useState(false);
	const [busy, setBusy] = useState(false);
	const [result, setResult] = useState<Obj | null>(null);
	const nav = useNavigate();
	async function scan(e: React.FormEvent) {
		e.preventDefault();
		if (!acceptTerms) return;
		const terms = Array.from(
			new Set(
				searchTerms
					.split(/[\n,\r]+/)
					.map((x: string) => x.trim())
					.filter(Boolean),
			),
		);
		if (terms.length > 5) {
			notify("検索語は最大5件までです");
			return;
		}
		setBusy(true);
		setResult(null);
		try {
			const searchData = await api("/admin/url-collection/search", {
				method: "POST",
				body: JSON.stringify({
					search_terms: terms,
					accept_terms: acceptTerms,
				}),
			});
			const seeds = (searchData.seed_urls || []) as string[];
			if (!seeds.length) {
				setResult({
					seed_count: 0,
					pages_scanned: 0,
					candidate_count: 0,
					requests_created: 0,
					duplicates_discarded: 0,
					failed: 0,
					items: [],
					search_queries: searchData.queries || [],
					search_results: searchData.results || [],
					search_errors: searchData.errors || [],
					errors: [],
				});
				notify("検索結果から巡回可能なHTTPSページが見つかりませんでした");
				return;
			}
			const data = await api("/admin/url-collection/scan", {
				method: "POST",
				body: JSON.stringify({ seed_urls: seeds, accept_terms: acceptTerms }),
			});
			setResult({
				...data,
				search_queries: searchData.queries || [],
				search_results: searchData.results || [],
				search_errors: searchData.errors || [],
				search_seed_count: seeds.length,
			});
			bumpRefresh();
			notify(
				`自動収集完了：新規申請 ${data.requests_created} 件、重複破棄 ${data.duplicates_discarded} 件`,
			);
		} catch (e) {
			notify((e as Error).message);
		} finally {
			setBusy(false);
		}
	}
	if (role !== "admin")
		return (
			<section className="panel">
				<h2>URL自動収集</h2>
				<p>この機能は管理者のみ利用できます。</p>
			</section>
		);
	const allErrors = [
		...(result?.search_errors || []),
		...(result?.errors || []),
	];
	return (
		<>
			<Heading
				eyebrow="URL DISCOVERY"
				title="URL自動収集"
				desc="DuckDuckGoで公開Webを検索し、検索結果のページを自動巡回して uploader.jp / getuploader.com のリンクを探します。開始URLを手入力する必要はありません。"
				actions={
					<button
						className="secondary"
						onClick={() => nav("/admin/review?tab=requests")}
					>
						<ClipboardList size={16} /> 申請・保全レビューへ
					</button>
				}
			/>
			<section className="panel form-panel">
				<div className="panel-head">
					<div>
						<h2>Web検索から自動収集</h2>
						<p>
							検索語を空欄にすると、MMDモデル・モーション・エフェクト・ステージ・配布の5種類を自動検索します。
						</p>
					</div>
					<Search size={20} />
				</div>
				<form className="form-stack" onSubmit={scan}>
					<label>
						検索語（任意、1行に1件・最大5件）
						<textarea
							rows={4}
							value={searchTerms}
							onChange={(e) => setSearchTerms(e.target.value)}
							placeholder={"例：\nMMD モデル\nMMD モーション\n東方 MMD 配布"}
						/>
					</label>
					<p className="muted">
						DuckDuckGoの公開HTML検索を利用します（検索APIキーは不要です）。各検索語から最大2ページ、合計最大10ページを巡回元として選びます。巡回は同一サイト内を浅く行い、合計20ページ・候補100件までです。検索結果やサイト側の制限によって見つかる件数は変わります。公開HTTPSページのみを対象とし、ログイン・CAPTCHAを回避せず、プライベートIPや内部ネットワークへの接続も拒否します。
					</p>
					<label className="check-label">
						<input
							type="checkbox"
							checked={acceptTerms}
							onChange={(e) => setAcceptTerms(e.target.checked)}
						/>{" "}
						検索結果の各サイトの利用規約・robots.txt等を尊重し、公開情報のみを節度ある頻度で調査します。
					</label>
					<div className="modal-actions">
						<button className="primary" disabled={busy || !acceptTerms}>
							{busy ? "検索・巡回中…" : "DuckDuckGoで検索して自動収集"}
						</button>
					</div>
				</form>
			</section>
			<div className="notice">
				<div className="notice-icon">
					<ShieldCheck size={19} />
				</div>
				<div>
					<b>自動収集は候補登録まで</b>
					<p>
						検出URLは「申請・保全レビュー」に未確認の保全リクエストとして登録されます。検索・巡回だけでファイル本体のダウンロードや一般公開は開始しません。重複URLは既存の収集レジストリ・申請・カタログと照合して破棄します。権利条件を確認し、レビュー画面から取得を開始してください。
					</p>
				</div>
			</div>
			{result && (
				<>
					<div className="stat-grid">
						<Stat
							label="調査ページ"
							value={result.pages_scanned ?? 0}
							sub={`検索結果ページ ${result.search_seed_count ?? 0} 件`}
							icon={Globe}
						/>
						<Stat
							label="検出候補"
							value={result.candidate_count ?? 0}
							sub={result.truncated ? "上限に到達" : "候補URL"}
							icon={Search}
							tone="violet"
						/>
						<Stat
							label="新規申請"
							value={result.requests_created ?? 0}
							sub="レビュー待ち"
							icon={ClipboardList}
							tone="green"
						/>
						<Stat
							label="重複破棄"
							value={result.duplicates_discarded ?? 0}
							sub={`失敗 ${result.failed ?? 0} 件`}
							icon={CheckCircle2}
							tone="amber"
						/>
					</div>
					{Array.isArray(result.search_queries) &&
						result.search_queries.length > 0 && (
							<section className="panel">
								<h2>実行した検索</h2>
								<ul>
									{result.search_queries.map((q: string, i: number) => (
										<li key={i}>{q}</li>
									))}
								</ul>
								{Array.isArray(result.search_results) &&
									result.search_results.length > 0 && (
										<>
											<h3>巡回対象の検索結果</h3>
											<ul>
												{result.search_results.map((x: Obj, i: number) => (
													<li key={x.url || i}>
														<a href={x.url} target="_blank" rel="noreferrer">
															{x.title || x.url}
														</a>
													</li>
												))}
											</ul>
										</>
									)}
							</section>
						)}
					{allErrors.length > 0 && (
						<section className="panel">
							<h2>一部の検索・ページを処理できませんでした</h2>
							<ul>
								{allErrors.map((e: string, i: number) => (
									<li key={i}>{e}</li>
								))}
							</ul>
						</section>
					)}
					<section className="panel table-panel">
						<div className="panel-head">
							<div>
								<h2>収集結果</h2>
								<p>重複URLは新しい申請を作らず破棄しました。</p>
							</div>
							<button
								className="secondary"
								onClick={() => nav("/admin/review?tab=requests")}
							>
								申請を確認 →
							</button>
						</div>
						<Table headers={["候補", "URL", "検索結果ページ", "結果"]}>
							{(result.items || []).map((x: Obj, i: number) => (
								<tr key={x.url || i}>
									<td>{x.label || "候補URL"}</td>
									<td>
										<a href={x.url} target="_blank" rel="noreferrer">
											{x.url}
										</a>
									</td>
									<td>
										<a href={x.seed_url} target="_blank" rel="noreferrer">
											巡回元を開く
										</a>
									</td>
									<td>
										{x.status === "request_created" ? (
											<span className="badge green">
												<i />
												申請登録
											</span>
										) : x.status === "duplicate" ? (
											<span className="badge amber">
												<i />
												重複破棄
											</span>
										) : (
											<span className="badge red">
												<i />
												{x.status === "failed" ? "失敗" : "スキップ"}
											</span>
										)}
									</td>
								</tr>
							))}
						</Table>
					</section>
				</>
			)}
		</>
	);
}
