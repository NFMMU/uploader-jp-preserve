import type { PageContextServer } from "vike/types";

const SITE_NAME = "uploader.jp 保全プロジェクト";
const SITE_URL = "https://uploaderjp.udonya.moe";
const DEFAULT_DESCRIPTION =
	"uploader.jpのサービス終了に伴う創作関連アセットの消失を防ぐことを目的としたプロジェクトです。";
const DEFAULT_IMAGE = `${SITE_URL}/ogp/default.png`;

type ArtifactResponse = {
	item?: {
		id?: string;
		title?: string;
		description?: string;
		source_url?: string;
	};
};

function escapeMetadata(value: unknown, fallback: string): string {
	if (typeof value !== "string") return fallback;
	const normalized = value.replace(/[\u0000-\u001f\u007f]/g, " ").trim();
	return normalized || fallback;
}

export async function data(pageContext: PageContextServer) {
	const pathname = pageContext.urlPathname;
	const metadata = {
		siteName: SITE_NAME,
		siteUrl: SITE_URL,
		title: SITE_NAME,
		description: DEFAULT_DESCRIPTION,
		image: DEFAULT_IMAGE,
		type: "website" as "website" | "article",
		url: new URL(pathname, SITE_URL).href,
		noIndex: pathname.startsWith("/admin"),
	};

	const staticPages: Record<string, { title: string; description: string }> = {
		"/requests": {
			title: "申請",
			description: "公開停止・削除などの申請を行えます。",
		},
		"/preservation-requests": {
			title: "保全の相談",
			description: "創作関連アセットの保全について相談できます。",
		},
		"/redistribution-support": {
			title: "再配布の相談",
			description: "ファイルの再配布や権利者への問い合わせを相談できます。",
		},
		"/publisher-support": {
			title: "配布・再配布の支援",
			description: "配布元・権利者の方向けの配布支援ページです。",
		},
		"/relocation-info": {
			title: "移設先情報の提供",
			description: "uploader.jp上の作品の移設先情報を提供できます。",
		},
		"/participate": {
			title: "保存ノード参加",
			description: "プロジェクトの分散保存ノードへの参加案内です。",
		},
		"/terms": {
			title: "利用規約",
			description: "uploader.jp保全プロジェクトの利用規約です。",
		},
		"/privacy": {
			title: "プライバシー・ポリシー",
			description: "uploader.jp保全プロジェクトのプライバシー・ポリシーです。",
		},
		"/disclaimer": {
			title: "免責事項",
			description: "uploader.jp保全プロジェクトの免責事項です。",
		},
	};

	const staticPage = staticPages[pathname];
	if (staticPage) {
		metadata.title = `${staticPage.title} - ${SITE_NAME}`;
		metadata.description = staticPage.description;
	}

	const artifactMatch = pathname.match(/^\/artifacts\/([^/]+)\/?$/);
	if (artifactMatch) {
		metadata.type = "article";
		try {
			const apiOrigin = (process.env.API_ORIGIN || "http://api:8080").replace(
				/\/$/,
				"",
			);
			const response = await fetch(
				`${apiOrigin}/api/v1/public/artifacts/${encodeURIComponent(decodeURIComponent(artifactMatch[1]))}?page=1&limit=1`,
				{
					headers: { Accept: "application/json" },
					signal: AbortSignal.timeout(5000),
				},
			);
			if (response.ok) {
				const body = (await response.json()) as ArtifactResponse;
				if (body.item) {
					const artifactTitle = escapeMetadata(body.item.title, "作品詳細");
					metadata.title = `${artifactTitle} - ${SITE_NAME}`;
					metadata.description = escapeMetadata(
						body.item.description,
						`${artifactTitle}の作品情報と公開ファイル。uploader.jp保全プロジェクトで保全しています。`,
					).slice(0, 300);
					metadata.url = `${SITE_URL}/artifacts/${encodeURIComponent(body.item.id || artifactMatch[1])}`;
				}
			}
		} catch (error) {
			// SSR metadata should still render with safe defaults if the API is down.
			console.error("Failed to fetch artifact metadata for SSR:", error);
		}
	}

	metadata.image =
  `${SITE_URL}/og.png?title=${encodeURIComponent(metadata.title)}` +
  `&description=${encodeURIComponent(metadata.description)}`;

	return { metadata };
}
