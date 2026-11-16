import { Archive, ArrowUpRight, Search, UserRound } from "lucide-react";
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
				String(artifact.id || ""),
			]
				.join(" ")
				.toLocaleLowerCase()
				.includes(q),
		);
	}, [items, query]);

	return (
		<>
			<Heading
				eyebrow="MY WORKSPACE"
				title="割り当てられた作品"
				desc="担当する作品を選択すると、作品情報の編集やファイルの管理を行えます。"
			/>
			<section className="panel form-stack" style={{ padding: 18, gap: 14 }}>
				<div className="filter-bar" style={{ margin: 0 }}>
					<div className="search-box" style={{ flex: 1 }}>
						<Search size={17} />
						<input
							value={query}
							onChange={(event) => setQuery(event.target.value)}
							placeholder="作品名・作者・説明・IDで検索"
							aria-label="割り当て作品を検索"
						/>
					</div>
					<span className="result-count" aria-live="polite">
						{data.loading ? "読み込み中…" : `${filtered.length} / ${items.length} 作品`}
					</span>
				</div>
				{query && (
					<div>
						<button className="small-btn" type="button" onClick={() => setQuery("")}>
							検索条件をクリア
						</button>
					</div>
				)}
			</section>

			{data.loading ? (
				<section className="panel empty">割り当てられた作品を読み込んでいます…</section>
			) : data.error ? (
				<section className="panel policy-warning">
					作品一覧を取得できませんでした: {data.error}
				</section>
			) : filtered.length === 0 ? (
				<section className="panel empty">
					<Archive size={28} />
					<b>{items.length ? "一致する作品がありません" : "まだ作品が割り当てられていません"}</b>
					<span>
						{items.length
							? "検索語を変えるか、検索条件をクリアしてください。"
							: "管理者に作品の割り当てを依頼してください。"}
					</span>
				</section>
			) : (
				<div className="content-grid">
					{filtered.map((artifact) => (
						<NavLink
							className="panel quick-card"
							to={`/admin/artifacts/${encodeURIComponent(artifact.id)}`}
							key={artifact.id}
							aria-label={`${artifact.title || "名称未設定"}のファイル一覧を開く`}
						>
							<div className="quick-icon blue">
								<Archive size={20} />
							</div>
							<div className="quick-card-content">
								<b>{artifact.title || "名称未設定"}</b>
								<small><UserRound size={13} /> {artifact.creator_name || "作者不明"}</small>
								{artifact.description && <small>{artifact.description}</small>}
								<small className="muted"><code>{artifact.id}</code></small>
							</div>
							<ArrowUpRight size={18} />
						</NavLink>
					))}
				</div>
			)}
		</>
	);
}
