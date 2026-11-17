import { Bell, CalendarDays, Pin } from "lucide-react";
import { useMemo, useState } from "react";
import ReactMarkdown from "react-markdown";
import { Heading } from "@/components/common/Heading";

type Notice = {
	slug: string;
	title: string;
	date: string;
	category: string;
	pinned: boolean;
	body: string;
};

const sources = import.meta.glob("../../content/admin-notices/*.md", {
	eager: true,
	query: "?raw",
	import: "default",
}) as Record<string, string>;

function parseNotice(path: string, source: string): Notice | null {
	const match = source.match(/^---\s*\n([\s\S]*?)\n---\s*\n?/);
	if (!match) return null;
	const metadata = match[1];
	const field = (name: string, fallback = "") =>
		metadata.match(new RegExp(`^${name}:\s*(.*)$`, "m"))?.[1]?.trim() || fallback;
	const body = source.slice(match[0].length).trim();
	const slug = path.split("/").pop()?.replace(/\.md$/, "") || path;
	const title = field("title") || body.match(/^#\s+(.+)$/m)?.[1] || slug;
	return {
		slug,
		title,
		date: field("date", "1970-01-01"),
		category: field("category", "お知らせ"),
		pinned: field("pinned", "false") === "true",
		body: body.replace(/^#\s+.+\n?/, "").trim(),
	};
}

const notices = Object.entries(sources)
	.map(([path, source]) => parseNotice(path, source))
	.filter((notice): notice is Notice => notice !== null)
	.sort((a, b) => Number(b.pinned) - Number(a.pinned) || b.date.localeCompare(a.date));

export function AdminNotices() {
	const [category, setCategory] = useState("すべて");
	const categories = useMemo(
		() => ["すべて", ...new Set(notices.map((notice) => notice.category))],
		[],
	);
	const visible = notices.filter(
		(notice) => category === "すべて" || notice.category === category,
	);

	return (
		<>
			<Heading
				eyebrow="INFORMATION"
				title="管理者からのお知らせ"
				desc="運営からの重要なお知らせ、メンテナンス情報、更新内容を掲載します。"
			/>
			<div className="admin-notices">
				<div className="notice-toolbar" aria-label="お知らせのカテゴリ">
					{categories.map((item) => (
						<button
							className={category === item ? "active" : ""}
							key={item}
							onClick={() => setCategory(item)}
							type="button"
						>
							{item}
						</button>
					))}
					<span className="notice-count">{visible.length} 件</span>
				</div>
				{visible.length ? (
					<div className="notice-list">
						{visible.map((notice) => (
							<article className="admin-notice-card" key={notice.slug}>
								<div className="admin-notice-meta">
									<span className="admin-notice-category">{notice.category}</span>
									<time dateTime={notice.date}>
										<CalendarDays size={14} />
										{new Date(`${notice.date}T00:00:00`).toLocaleDateString("ja-JP", {
											year: "numeric",
											month: "long",
											day: "numeric",
										})}
									</time>
									{notice.pinned && (
										<span className="admin-notice-pinned"><Pin size={13} /> 固定</span>
									)}
								</div>
								<h2>{notice.title}</h2>
								<div className="admin-notice-body">
									<ReactMarkdown>{notice.body}</ReactMarkdown>
								</div>
							</article>
						))}
					</div>
				) : (
					<div className="admin-notices-empty">
						<Bell size={22} />
						<p>このカテゴリのお知らせはありません。</p>
					</div>
				)}
				<p className="admin-notices-footnote">
					お知らせは運営リポジトリで管理されています。内容に関する問い合わせは運営までご連絡ください。
				</p>
			</div>
		</>
	);
}
