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
			<meta name="twitter:site" content="@udonya_moe" />
			<meta name="twitter:creator" content="@udonya_moe" />
			<meta name="twitter:title" content={metadata.title} />
			<meta name="twitter:description" content={metadata.description} />
			<meta name="twitter:image" content={metadata.image} />
			<script
				dangerouslySetInnerHTML={{
					__html: `
            (function(w,d,s,l,i){
              w[l]=w[l]||[];
              w[l].push({'gtm.start':new Date().getTime(),event:'gtm.js'});
              var f=d.getElementsByTagName(s)[0],
                  j=d.createElement(s),
                  dl=l!='dataLayer'?'&l='+l:'';
              j.async=true;
              j.src='https://www.googletagmanager.com/gtm.js?id='+i+dl;
              f.parentNode.insertBefore(j,f);
            })(window,document,'script','dataLayer','GTM-NXK523G5');
          `,
				}}
			/>
			<noscript>
				<iframe
					src={`https://www.googletagmanager.com/ns.html?id=GTM-NXK523G5`}
					height="0"
					width="0"
					style={{ display: "none", visibility: "hidden" }}
					title="Google Tag Manager"
				/>
			</noscript>
		</>
	);
}
