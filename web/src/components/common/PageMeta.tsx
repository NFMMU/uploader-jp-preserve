type PageMetaProps = {
	title: string;
	description: string;
	image?: string;
	url?: string;
	type?: "website" | "article";
	overwriteTitle?: boolean;
};

const SITE_NAME = "uploader.jp 保全プロジェクト";
const SITE_URL = "https://uploaderjp.udonya.moe";
const DEFAULT_IMAGE = `${SITE_URL}/ogp/default.png`;

export function PageMeta({
	title,
	description,
	image = DEFAULT_IMAGE,
	url,
	type = "website",
	overwriteTitle = false,
}: PageMetaProps) {
	const fullTitle = overwriteTitle ? `${title}` : `${title} - ${SITE_NAME}`;
	const pageUrl = url ? new URL(url, SITE_URL).href : undefined;

	return (
		<>
			<title>{fullTitle}</title>
			<meta name="description" content={description} />

			{/* Open Graph */}
			<meta property="og:type" content={type} />
			<meta property="og:site_name" content={SITE_NAME} />
			<meta property="og:title" content={fullTitle} />
			<meta property="og:description" content={description} />
			<meta property="og:image" content={image} />
			{pageUrl && <meta property="og:url" content={pageUrl} />}

			{/* X / Twitter Card */}
			<meta name="twitter:card" content="summary_large_image" />
			<meta name="twitter:title" content={fullTitle} />
			<meta name="twitter:description" content={description} />
			<meta name="twitter:image" content={image} />
		</>
	);
}
