import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { publicAPI } from "../../../api/public";
import { Pagination } from "../../../components/common/Pagination";
import { PageMeta } from "../../../components/common/PageMeta";
import type { PublicArtifact } from "../../../types/public";
import { PublicFileRow } from "./PublicFileRow";

type PublicArtifactDetailResponse = {
	item: PublicArtifact;
	files_page: number;
	files_limit: number;
	files_total: number;
};

export function PublicArtifactDetail() {
	const { id = "" } = useParams();

	const [item, setItem] = useState<PublicArtifact | null>(null);
	const [page, setPage] = useState(1);
	const [limit, setLimit] = useState(20);
	const [total, setTotal] = useState(0);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState("");

	useEffect(() => {
		let active = true;

		setLoading(true);
		setError("");

		publicAPI<PublicArtifactDetailResponse>(
			`/public/artifacts/${encodeURIComponent(id)}?page=${page}&limit=${limit}`,
		)
			.then((data) => {
				if (active) {
					setItem(data.item);
					setTotal(Number(data.files_total ?? 0));
					setError("");
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
	}, [id, page, limit]);

	if (loading && !item) {
		return <p className="public-muted">読み込み中…</p>;
	}

	if (error && !item) {
		return (
			<>
				<p className="public-error">{error || "作品が見つかりません。"}</p>
				<Link className="public-text-link" to="/">
					← 作品一覧
				</Link>
			</>
		);
	}

	if (!item) {
		return <p className="public-muted">作品が見つかりません。</p>;
	}

	return (
		<>
			<PageMeta
				title={item.title}
				description={item.description || `${item.title}の作品情報と公開ファイル。uploader.jp保全プロジェクトで保全しています。`}
				type="article"
				url={`/artifacts/${item.id}`}
			/>
			<Link className="public-text-link" to="/">
				← 作品一覧
			</Link>

			<h1 className="public-detail-title">{item.title}</h1>

			<dl className="public-metadata">
				<dt>作者</dt>
				<dd>{item.creator_name || "不明"}</dd>
				<dt>元ページ</dt>
				<dd>
					<a href={item.source_url} target="_blank" rel="noreferrer">
						元ページを開く ↗
					</a>
				</dd>
			</dl>

			{item.description && (
				<p className="public-detail-description">{item.description}</p>
			)}

			<h2 className="public-section-title">ファイル</h2>

			{error ? (
				<p className="public-error">{error}</p>
			) : loading ? (
				<p className="public-muted">ファイルを読み込み中…</p>
			) : item.files.length === 0 ? (
				<p className="public-muted">公開できるファイルはありません。</p>
			) : (
				<div className="public-file-list">
					{item.files.map((file) => (
						<PublicFileRow key={file.id} file={file} artifactId={item.id} />
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

			<p className="public-footnote">
				ダウンロードできるのは、公開条件と保存状態を確認できたファイルだけです。
			</p>
		</>
	);
}
