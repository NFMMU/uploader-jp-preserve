import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { publicAPI } from "@/api/public";
import { PageMeta } from "@/components/common/PageMeta";
import { formatBytes } from "@/lib/format";
import type { PublicFileDetail as PublicFileDetailType } from "@/types/public";

export function PublicFileDetail() {
 const { id = "" } = useParams();
 const [item, setItem] = useState<PublicFileDetailType | null>(null);
 const [error, setError] = useState("");
 const [loading, setLoading] = useState(true);
 useEffect(() => {
  let active = true;
  setLoading(true);
  publicAPI<{ item: PublicFileDetailType }>(`/public/files/${encodeURIComponent(id)}`)
   .then((data) => { if (active) { setItem(data.item); setError(""); } })
   .catch((e: Error) => { if (active) setError(e.message); })
   .finally(() => { if (active) setLoading(false); });
  return () => { active = false; };
 }, [id]);
 if (loading) return <p className="public-muted">読み込み中…</p>;
 if (error || !item) return <><p className="public-error">{error || "ファイルが見つかりません。"}</p><Link className="public-text-link" to="/">← 公開カタログ</Link></>;
 return <>
  <PageMeta title={item.original_filename} description={item.description || `${item.original_filename}のファイル情報。`} type="article" url={`/files/${item.id}`} />
  <Link className="public-text-link" to={`/artifacts/${item.artifact_id}`}>← {item.artifact_title}</Link>
  <h1 className="public-detail-title">{item.original_filename}</h1>
  <dl className="public-metadata"><dt>作品</dt><dd><Link to={`/artifacts/${item.artifact_id}`}>{item.artifact_title}</Link></dd><dt>ファイルサイズ</dt><dd>{formatBytes(item.size_bytes)}</dd>{item.media_type && <><dt>MIME タイプ</dt><dd>{item.media_type}</dd></>}</dl>
  {item.description ? <p className="public-detail-description">{item.description}</p> : <p className="public-muted">このファイルの説明はまだ登録されていません。</p>}
  <p className="public-footnote">ダウンロードは作品ページのファイル一覧から行えます。公開条件と保存状態を確認できたファイルのみ表示されます。</p>
 </>;
}
