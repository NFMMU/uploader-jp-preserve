import {
	Archive,
	ArrowUpRight,
	Files,
	FolderOpen,
	Search,
	UserRound,
} from "lucide-react";
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
						{data.loading
							? "読み込み中…"
							: `${filtered.length} / ${items.length} 作品`}
					</span>
				</div>
				{query && (
					<div>
						<button
							className="small-btn"
							type="button"
							onClick={() => setQuery("")}
						>
							検索条件をクリア
						</button>
					</div>
				)}
			</section>

			{data.loading ? (
				<section className="panel empty">
					割り当てられた作品を読み込んでいます…
				</section>
			) : data.error ? (
				<section className="panel policy-warning">
					作品一覧を取得できませんでした: {data.error}
				</section>
			) : filtered.length === 0 ? (
				<section className="panel empty">
					<Archive size={28} />
					<b>
						{items.length
							? "一致する作品がありません"
							: "まだ作品が割り当てられていません"}
					</b>
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
							className="panel artifact-catalog-card"
							to={{
								pathname: `/admin/artifacts/${encodeURIComponent(artifact.id)}`,
								search: query.trim()
									? `?q=${encodeURIComponent(query.trim())}`
									: "",
							}}
							key={artifact.id}
						>
							<div className="artifact-catalog-card-head">
								<span className="artifact-catalog-icon">
									<FolderOpen size={19} />
								</span>
								<span className="artifact-catalog-count">
									<Files size={14} />{" "}
									{Number(artifact.file_count || 0).toLocaleString()} ファイル
								</span>
							</div>
							<div className="artifact-catalog-card-body">
								<b>{artifact.title || "名称未設定"}</b>
								<span>{artifact.creator_name || "作者不明"}</span>
								{artifact.description && <p>{artifact.description}</p>}
								<small>
									<code>{artifact.id}</code>
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
		</>
	);
}
