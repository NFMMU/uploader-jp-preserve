import { Archive, Search } from "lucide-react";
import { useMemo, useState } from "react";
import { NavLink } from "react-router-dom";
import { Heading } from "@/components/common/Heading";
import { useData } from "@/hooks/useData";
import type { Obj } from "@/types/common";

export function AuthorArtifacts({ api, refresh }: any) {
	const data = useData(api, "/catalog/artifacts?all=true", refresh);
	const [query, setQuery] = useState("");
	const items: Obj[] = data.data.items || [];
	const filtered = useMemo(() => {
		const q = query.trim().toLocaleLowerCase();
		if (!q) return items;
		return items.filter((artifact) =>
			[
				String(artifact.title || ""),
				String(artifact.creator_name || ""),
				String(artifact.description || ""),
			]
				.join(" ")
				.toLocaleLowerCase()
				.includes(q),
		);
	}, [items, query]);

	return (
		<>
			<Heading
				eyebrow="MY CATALOG"
				title="割り当てられた作品"
				desc="管理者から割り当てられた作品を確認し、作品内のファイルを操作できます。"
			/>
			<div className="filter-bar">
				<div className="search-box">
					<Search size={17} />
					<input
						value={query}
						onChange={(event) => setQuery(event.target.value)}
						placeholder="作品名・作者・説明で検索"
					/>
				</div>
				<span className="result-count">
					{data.loading ? "—" : filtered.length} /{" "}
					{data.data.total ?? items.length} 作品
				</span>
			</div>
			{data.loading ? (
				<section className="panel empty">作品を読み込み中…</section>
			) : data.error ? (
				<section className="panel policy-warning">
					作品一覧を取得できませんでした: {data.error}
				</section>
			) : filtered.length === 0 ? (
				<section className="panel empty">
					{items.length
						? "検索条件に一致する作品がありません。"
						: "現在、割り当てられている作品はありません。管理者に割り当てを依頼してください。"}
				</section>
			) : (
				<div className="content-grid">
					{filtered.map((artifact) => (
						<NavLink
							className="panel quick-card"
							to={`/admin/artifacts/${encodeURIComponent(artifact.id)}`}
							key={artifact.id}
						>
							<div className="quick-icon blue">
								<Archive size={20} />
							</div>
							<div className="quick-card-content">
								<b>{artifact.title || "名称未設定"}</b>
								<small>{artifact.creator_name || "作者不明"}</small>
								{artifact.description && <small>{artifact.description}</small>}
							</div>
							<span>→</span>
						</NavLink>
					))}
				</div>
			)}
		</>
	);
}
