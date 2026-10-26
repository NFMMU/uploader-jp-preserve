import { publicAPI } from "../../../api/public";
import type { PublicArtifact } from "../../../types/public";
import { PublicFileRow } from "./PublicFileRow";
import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";

export function PublicArtifactDetail() {
  const { id = "" } = useParams();
  const [item, setItem] = useState<PublicArtifact | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    setLoading(true);
    publicAPI<{ item: PublicArtifact }>(
      `/public/artifacts/${encodeURIComponent(id)}`,
    )
      .then((data) => {
        if (active) {
          setItem(data.item);
          setError("");
        }
      })
      .catch((e) => {
        if (active) setError(e.message);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [id]);
  if (loading) return <p className="public-muted">読み込み中…</p>;
  if (error || !item)
    return (
      <>
        <p className="public-error">{error || "作品が見つかりません。"}</p>
        <Link className="public-text-link" to="/">
          ← 作品一覧
        </Link>
      </>
    );
  return (
    <>
      <Link className="public-text-link" to="/">
        ← 作品一覧
      </Link>
      <h1 className="public-detail-title">{item.title}</h1>
      <dl className="public-metadata">
        <dt>作者</dt>
        <dd>{item.creator_name || "不明"}</dd>
        <dt>元ページ</dt>
        <dd>
          <a href={item.source_url} target="_blank" rel="noreferrer">
            元ページを開く ↗
          </a>
        </dd>
      </dl>
      {item.description && (
        <p className="public-detail-description">{item.description}</p>
      )}
      <h2 className="public-section-title">ファイル</h2>
      {item.files.length === 0 ? (
        <p className="public-muted">公開できるファイルはありません。</p>
      ) : (
        <div className="public-file-list">
          {item.files.map((file) => (
            <PublicFileRow key={file.id} file={file} artifactId={item.id} />
          ))}
        </div>
      )}
      <p className="public-footnote">
        ダウンロードできるのは、公開条件と保存状態を確認できたファイルだけです。
      </p>
    </>
  );
}
