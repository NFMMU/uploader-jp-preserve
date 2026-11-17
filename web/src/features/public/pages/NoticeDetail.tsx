import { ArrowLeft, CalendarDays, Clock3, Tag } from "lucide-react";
import { useMemo } from "react";
import ReactMarkdown from "react-markdown";
import { Link, Navigate, useParams } from "react-router-dom";
import { PageMeta } from "@/components/common/PageMeta";
import { type Notice, parseNotice } from "./notice/parseNotice";
import { parseNoticeHeadings, remarkNoticeHeadingIds } from "./notice/markdownHeadings";
import noticesSource from "virtual:public-notices";

const notices = (noticesSource as Array<{ path: string; source: string }>)
	.map(({ path, source }) => parseNotice(path, source))
	.filter((notice): notice is Notice => notice !== null);

function formatDate(date: string) {
	return new Date(date + "T00:00:00").toLocaleDateString("ja-JP", {
		year: "numeric", month: "long", day: "numeric",
	});
}

function readingTime(markdown: string) {
	return Math.max(1, Math.ceil(markdown.replace(/\s/g, "").length / 500));
}

export function NoticeDetail() {
	const { id } = useParams();
	const notice = notices.find((item) => item.slug === id);
	const headings = useMemo(
		() => notice ? parseNoticeHeadings(notice.body) : [],
		[notice],
	);

	if (!notice) return <Navigate to="/notices" replace />;

	return (
		<>
			<PageMeta title={notice.title + " - お知らせ"} description={notice.summary} />
			<div className="notice-detail-page">
				<Link className="notice-back-link" to="/notices">
					<ArrowLeft size={16} /> お知らせ一覧へ戻る
				</Link>
				<article className="notice-article">
					<header className="notice-article-header">
						<div className="notice-article-meta">
							<time dateTime={notice.date}><CalendarDays size={15} /> {formatDate(notice.date)}</time>
							{notice.updatedAt && <span>更新：{formatDate(notice.updatedAt)}</span>}
							<span><Clock3 size={15} /> 約{readingTime(notice.body)}分で読めます</span>
						</div>
						<h1>{notice.title}</h1>
						{notice.tags.length > 0 && (
							<div className="notice-entry-tags notice-article-tags">
								{notice.tags.map((tag) => (
									<Link key={tag} to={"/notices?tag=" + encodeURIComponent(tag)}>
										<Tag size={13} /> {tag}
									</Link>
								))}
							</div>
						)}
					</header>
					{headings.length > 0 && (
						<nav className="notice-toc" aria-label="この記事の目次">
							<b>この記事の内容</b>
							<ul>
								{headings.map((heading) => (
									<li className={heading.level === 3 ? "nested" : ""} key={heading.id}>
										<a href={"#" + heading.id}>{heading.title}</a>
									</li>
								))}
							</ul>
						</nav>
					)}
					<div className="notice-article-body">
						<ReactMarkdown
							remarkPlugins={[remarkNoticeHeadingIds()]}
							components={{
								a: ({ href, children }) => (
									<a href={href} target={href?.startsWith("http") ? "_blank" : undefined}
										rel={href?.startsWith("http") ? "noreferrer" : undefined}>
										{children}
									</a>
								),
							}}
						>
							{notice.body}
						</ReactMarkdown>
					</div>
					<footer className="notice-article-footer">
						<Link to="/notices"><ArrowLeft size={15} /> お知らせ一覧へ戻る</Link>
					</footer>
				</article>
			</div>
		</>
	);
}
