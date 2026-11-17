import noticesSource from "virtual:public-notices";
import { Bell, CalendarDays, Pin, Tag } from "lucide-react";
import { useMemo } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { Heading } from "@/components/common/Heading";
import { PageMeta } from "@/components/common/PageMeta";
import { type Notice, parseNotice } from "./notice/parseNotice";

const notices = (noticesSource as Array<{ path: string; source: string }>)
	.map(({ path, source }) => parseNotice(path, source))
	.filter((notice): notice is Notice => notice !== null)
	.sort(
		(a, b) =>
			Number(b.pinned) - Number(a.pinned) || b.date.localeCompare(a.date),
	);

function formatDate(date: string) {
	return new Date(date + "T00:00:00").toLocaleDateString("ja-JP", {
		year: "numeric",
		month: "long",
		day: "numeric",
	});
}

export function PublicNotices() {
	const [searchParams, setSearchParams] = useSearchParams();
	const selectedTag = searchParams.get("tag") || "";
	const tags = useMemo(
		() =>
			[...new Set(notices.flatMap((notice) => notice.tags))].sort((a, b) =>
				a.localeCompare(b, "ja"),
			),
		[],
	);
	const visible = notices.filter(
		(notice) => !selectedTag || notice.tags.includes(selectedTag),
	);

	function selectTag(tag: string) {
		const next = new URLSearchParams(searchParams);
		if (tag) next.set("tag", tag);
		else next.delete("tag");
		setSearchParams(next);
	}

	return (
		<>
			<PageMeta
				title="お知らせ | uploader.jp保全プロジェクト"
				description="uploader.jp保全プロジェクトからのお知らせやサービスに関する情報をお届けします。"
			/>
			<Heading
				eyebrow="INFORMATION"
				title="お知らせ"
				desc="プロジェクトの活動状況、サービスの更新、メンテナンスなどの情報をお届けします。"
			/>
			<section className="public-notices" aria-label="お知らせ一覧">
				{tags.length > 0 && (
					<div className="notice-tag-filters">
						<button
							className={!selectedTag ? "active" : ""}
							onClick={() => selectTag("")}
							type="button"
						>
							すべて
						</button>
						{tags.map((tag) => (
							<button
								className={selectedTag === tag ? "active" : ""}
								key={tag}
								onClick={() => selectTag(tag)}
								type="button"
							>
								<Tag size={13} /> {tag}
							</button>
						))}
					</div>
				)}
				<div className="notice-list-header">
					<span>
						{selectedTag ? "「" + selectedTag + "」の投稿" : "すべてのお知らせ"}
					</span>
					<span>{visible.length} 件</span>
				</div>
				{visible.length ? (
					<div className="notice-list">
						{visible.map((notice) => (
							<article className="notice-list-item" key={notice.slug}>
								<div className="notice-list-main">
									<div className="notice-entry-meta">
										<time dateTime={notice.date}>
											<CalendarDays size={14} /> {formatDate(notice.date)}
										</time>
										{notice.pinned && (
											<span className="notice-pinned">
												<Pin size={13} /> 重要
											</span>
										)}
									</div>
									<h2>
										<Link to={"/notices/" + encodeURIComponent(notice.slug)}>
											{notice.title}
										</Link>
									</h2>
									<p className="notice-summary">{notice.summary}</p>
									{notice.tags.length > 0 && (
										<div className="notice-entry-tags">
											{notice.tags.map((tag) => (
												<Link
													key={tag}
													to={"/notices?tag=" + encodeURIComponent(tag)}
												>
													{tag}
												</Link>
											))}
										</div>
									)}
								</div>
								<Link
									className="notice-read-more"
									to={"/notices/" + encodeURIComponent(notice.slug)}
								>
									続きを読む <span aria-hidden="true">→</span>
								</Link>
							</article>
						))}
					</div>
				) : (
					<div className="admin-notice-empty">
						<Bell size={22} />
						<p>
							{selectedTag
								? "このタグが付いたお知らせはありません。"
								: "現在公開中のお知らせはありません。"}
						</p>
					</div>
				)}
			</section>
		</>
	);
}
