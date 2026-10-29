import { type FormEvent, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { publicAPI } from "../../../api/public";
import { Pagination } from "../../../components/common/Pagination";
import { PageMeta } from "../../../components/common/PageMeta";
import type { PublicArtifact } from "../../../types/public";

type PublicArtifactsResponse = {
	items: PublicArtifact[];
	page: number;
	limit: number;
	total: number;
};

export function PublicCatalog() {
	const [query, setQuery] = useState("");
	const [submitted, setSubmitted] = useState("");
	const [items, setItems] = useState<PublicArtifact[]>([]);
	const [page, setPage] = useState(1);
	const [limit, setLimit] = useState(20);
	const [total, setTotal] = useState(0);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState("");

	useEffect(() => {
		let active = true;

		setLoading(true);
		setError("");

		publicAPI<PublicArtifactsResponse>(
			`/public/artifacts?q=${encodeURIComponent(submitted)}&page=${page}&limit=${limit}`,
		)
			.then((data) => {
				if (active) {
					setItems(data.items || []);
					setTotal(Number(data.total ?? 0));
				}
			})
			.catch((e: Error) => {
				if (active) setError(e.message);
			})
			.finally(() => {
				if (active) setLoading(false);
			});

		return () => {
			active = false;
		};
	}, [submitted, page, limit]);

	function search(event: FormEvent) {
		event.preventDefault();
		setPage(1);
		setSubmitted(query.trim());
	}

	return (
		<>
			<PageMeta
				title="uploader.jp保全プロジェクト - 創作関連アセットの保全を目的としたプロジェクト"
				description="uploader.jpのサービス終了に伴う創作関連アセットの消失を防ぐことを目的としたプロジェクトです。"
				overwriteTitle={true}
			/>
			<h1>作品一覧</h1>

			<section
				className="project-description"
				aria-labelledby="project-description-title"
			>
				<h2 id="project-description-title">このプロジェクトについて</h2>
				<p>
					このプロジェクトはUTAU、MMD、YMMなどの制作に用いるアセットを保全するためのプロジェクトです。
				</p>
				<p>
					ゲームに関するアセットについては、受付はいたしますが対応の優先順位が低く対応されにくいことをご了承ください。
				</p>
			</section>

			<section className="public-service-cards" aria-label="再配布支援サービス">
				<Link to="/redistribution-support" className="public-service-card">
					<span className="public-service-label">ファイルを持っている方</span>
					<b>再配布の相談・権利者への問い合わせ代行</b>
					<span>ファイルを送って、権利確認を依頼する →</span>
				</Link>
				<Link to="/publisher-support" className="public-service-card">
					<span className="public-service-label">配布元・権利者の方</span>
					<b>配布・再配布の代行を相談する</b>
					<span>配布先探しや管理の手間を相談する →</span>
				</Link>
			</section>

			<p className="public-intro">公開が許可されたファイルを掲載しています。</p>

			<form className="public-search" onSubmit={search}>
				<input
					aria-label="作品を検索"
					value={query}
					onChange={(e) => setQuery(e.target.value)}
					placeholder="作品・作者・ファイル名・uploader.jpのURL"
				/>
				<button type="submit">検索</button>
			</form>

			{loading ? (
				<p className="public-muted">読み込み中…</p>
			) : error ? (
				<p className="public-error">{error}</p>
			) : items.length === 0 ? (
				<p className="public-muted">公開中の作品はありません。</p>
			) : (
				<div className="public-list">
					{items.map((item) => (
						<article className="public-list-item" key={item.id}>
							<div>
								<h2>
									<Link to={`/artifacts/${item.id}`}>{item.title}</Link>
								</h2>
								<p className="public-byline">
									{item.creator_name || "作者不明"} · ファイル{" "}
									{item.files.length} 件
								</p>
								{item.description && (
									<p className="public-description">{item.description}</p>
								)}
							</div>
							<Link className="public-text-link" to={`/artifacts/${item.id}`}>
								詳細 →
							</Link>
						</article>
					))}
				</div>
			)}

			{total > 0 && (
				<Pagination
					page={page}
					limit={limit}
					total={total}
					loading={loading}
					onPageChange={setPage}
					onLimitChange={(nextLimit) => {
						setLimit(nextLimit);
						setPage(1);
					}}
				/>
			)}
		</>
	);
}
