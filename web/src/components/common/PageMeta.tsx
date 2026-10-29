import { useEffect } from "react";

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

/**
 * Initial HTML metadata is rendered by Vike's pages/+Head.tsx. This component
 * updates metadata during client-side React Router navigation.
 */
export function PageMeta({
  title,
  description,
  image = DEFAULT_IMAGE,
  url,
  type = "website",
  overwriteTitle = false,
}: PageMetaProps) {
  useEffect(() => {
    const fullTitle = overwriteTitle ? title : `${title} - ${SITE_NAME}`;
    const pageUrl = url ? new URL(url, SITE_URL).href : window.location.href;
    document.title = fullTitle;

    const setMeta = (selector: string, attribute: string, key: string, value: string) => {
      let element = document.head.querySelector<HTMLMetaElement>(selector);
      if (!element) {
        element = document.createElement("meta");
        element.setAttribute(attribute, key);
        document.head.appendChild(element);
      }
      element.content = value;
    };

    setMeta('meta[name="description"]', "name", "description", description);
    setMeta('meta[property="og:type"]', "property", "og:type", type);
    setMeta('meta[property="og:site_name"]', "property", "og:site_name", SITE_NAME);
    setMeta('meta[property="og:title"]', "property", "og:title", fullTitle);
    setMeta('meta[property="og:description"]', "property", "og:description", description);
    setMeta('meta[property="og:image"]', "property", "og:image", image);
    setMeta('meta[property="og:url"]', "property", "og:url", pageUrl);
    setMeta('meta[name="twitter:card"]', "name", "twitter:card", "summary_large_image");
    setMeta('meta[name="twitter:title"]', "name", "twitter:title", fullTitle);
    setMeta('meta[name="twitter:description"]', "name", "twitter:description", description);
    setMeta('meta[name="twitter:image"]', "name", "twitter:image", image);
  }, [title, description, image, url, type, overwriteTitle]);

  return null;
}
