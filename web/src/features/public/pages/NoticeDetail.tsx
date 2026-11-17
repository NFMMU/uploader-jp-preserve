import { ArrowLeft, CalendarDays, Clock3, Tag } from "lucide-react";
import { useMemo } from "react";
import ReactMarkdown from "react-markdown";
import { Link, Navigate, useParams } from "react-router-dom";
import { PageMeta } from "@/components/common/PageMeta";
import { type Notice, parseNotice } from "./notice/parseNotice";

const sources = import.meta.glob("../../../content/notices/*.md", {
	eager: true,
	query: "?raw",
	import: "default",
}) as Record<string, string>;

const notices = Object.entries(sources)
	.map(([path, source]) => parseNotice(path, source))
	.filter((notice): notice is Notice => notice !== null && notice.published);

function formatDate(date: string) {
	return new Date(date + "T00:00:00").toLocaleDateString("ja-JP", {
		year: "numeric",
		month: "long",
		day: "numeric",
	});
}

function readingTime(markdown: string) {
	return Math.max(1, Math.ceil(markdown.replace(/\s/g, "").length / 500));
}

function headingId(value: string) {
	return (
		value
			.toLocaleLowerCase()
			.replace(/[^\p{Letter}\p{Number}\s-]/gu, "")
			.replace(/[\s-]+/g, "-") || "section"
	);
}

export function NoticeDetail() {
	const { id } = useParams();
	const notice = notices.find((item) => item.slug === id);
	const headings = useMemo(() => {
		if (!notice) return [];
		return notice.body.split(/\r?\n/).flatMap((line) => {
			const match = line.match(/^(##|###)\s+(.+?)\s*#*\s*$/);
			return match
				? [{ level: match[1].length, title: match[2], id: headingId(match[2]) }]
				: [];
		});
	}, [notice]);

	if (!notice) return <Navigate to="/notices" replace />;

	return (
		<>
			<PageMeta
				title={notice.title + " | お知らせ | uploader.jp保全プロジェクト"}
				description={notice.summary}
			/>
			<div className="notice-detail-page">
				<Link className="notice-back-link" to="/notices">
					<ArrowLeft size={16} /> お知らせ一覧へ戻る
				</Link>
				<article className="notice-article">
					<header className="notice-article-header">
						<div className="notice-article-meta">
							<time dateTime={notice.date}>
								<CalendarDays size={15} /> {formatDate(notice.date)}
							</time>
							{notice.updatedAt && (
								<span>更新：{formatDate(notice.updatedAt)}</span>
							)}
							<span>
								<Clock3 size={15} /> 約{readingTime(notice.body)}分で読めます
							</span>
						</div>
						<h1>{notice.title}</h1>
						{notice.tags.length > 0 && (
							<div className="notice-entry-tags notice-article-tags">
								{notice.tags.map((tag) => (
									<Link
										key={tag}
										to={"/notices?tag=" + encodeURIComponent(tag)}
									>
										<Tag size={13} /> {tag}
									</Link>
								))}
							</div>
						)}
					</header>
					{headings.length > 2 && (
						<nav className="notice-toc" aria-label="この記事の目次">
							<b>この記事の内容</b>
							<ul>
								{headings.map((heading, index) => (
									<li
										className={heading.level === 3 ? "nested" : ""}
										key={heading.id + "-" + index}
									>
										<a href={"#" + heading.id}>{heading.title}</a>
									</li>
								))}
							</ul>
						</nav>
					)}
					<div className="notice-article-body">
						<ReactMarkdown
							components={{
								h2: ({ children }) => (
									<h2 id={headingId(String(children ?? ""))}>{children}</h2>
								),
								h3: ({ children }) => (
									<h3 id={headingId(String(children ?? ""))}>{children}</h3>
								),
								a: ({ href, children }) => (
									<a
										href={href}
										target={href?.startsWith("http") ? "_blank" : undefined}
										rel={href?.startsWith("http") ? "noreferrer" : undefined}
									>
										{children}
									</a>
								),
							}}
						>
							{notice.body}
						</ReactMarkdown>
					</div>
					<footer className="notice-article-footer">
						<Link to="/notices">
							<ArrowLeft size={15} /> お知らせ一覧へ戻る
						</Link>
					</footer>
				</article>
			</div>
		</>
	);
}
