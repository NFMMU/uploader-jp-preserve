import { useData } from "vike-react/useData";
import type { Data } from "./+data";

export default function Head() {
	const { metadata } = useData<Data>();
	return (
		<>
			<title>{metadata.title}</title>
			<meta name="theme-color" content="#f8fafc" />
			<meta name="description" content={metadata.description} />
			{metadata.noIndex && <meta name="robots" content="noindex, nofollow" />}
			<meta property="og:type" content={metadata.type} />
			<meta property="og:site_name" content={metadata.siteName} />
			<meta property="og:title" content={metadata.title} />
			<meta property="og:description" content={metadata.description} />
			<meta property="og:image" content={metadata.image} />
			<meta property="og:url" content={metadata.url} />
			<meta name="twitter:card" content="summary_large_image" />
			<meta name="twitter:site" content="@thetalemon" />
			<meta name="twitter:creator" content="@thetalemon" />
			<meta name="twitter:title" content={metadata.title} />
			<meta name="twitter:description" content={metadata.description} />
			<meta name="twitter:image" content={metadata.image} />
		</>
	);
}
