-- Store GetUploader sources as host-independent paths and merge uploader-level artifacts.
BEGIN;

-- Canonicalize individual source references. Keep /download/{id} on file/request
-- records, while artifact records always point at the uploader root.
UPDATE file_objects
SET source_url = regexp_replace(
  regexp_replace(source_url, '^https://(ux|uu)\\.getuploader\\.com', '', 'i'),
  '/(index|list)/.*$', ''
)
WHERE source_url ~* '^https://(ux|uu)\\.getuploader\\.com/';

UPDATE public_requests
SET source_url = regexp_replace(
  regexp_replace(source_url, '^https://(ux|uu)\\.getuploader\\.com', '', 'i'),
  '/(index|list)/.*$', ''
)
WHERE source_url ~* '^https://(ux|uu)\\.getuploader\\.com/';

UPDATE artifacts
SET source_url = regexp_replace(
  regexp_replace(source_url, '^https://(ux|uu)\\.getuploader\\.com', '', 'i'),
  '/(download|index|list)/.*$', ''
)
WHERE source_url ~* '^https://(ux|uu)\\.getuploader\\.com/';

-- Merge artifacts that represent the same GetUploader account. Preserve the
-- oldest artifact ID and move every dependent record before deleting duplicates.
CREATE TEMP TABLE getuploader_artifact_merge ON COMMIT DROP AS
WITH canonical AS (
  SELECT id,
         CASE
           WHEN source_url LIKE '/%' THEN regexp_replace(source_url, '/(download|index|list)/.*$', '')
           ELSE source_url
         END AS uploader_key,
         first_value(id) OVER (
           PARTITION BY CASE
             WHEN source_url LIKE '/%' THEN regexp_replace(source_url, '/(download|index|list)/.*$', '')
             ELSE source_url
           END
           ORDER BY created_at, id
         ) AS keep_id
  FROM artifacts
)
SELECT id AS duplicate_id, keep_id
FROM canonical
WHERE id <> keep_id
  AND uploader_key LIKE '/%';

UPDATE file_objects f
SET artifact_id = m.keep_id
FROM getuploader_artifact_merge m
WHERE f.artifact_id = m.duplicate_id;

UPDATE public_requests r
SET artifact_id = m.keep_id
FROM getuploader_artifact_merge m
WHERE r.artifact_id = m.duplicate_id;

INSERT INTO author_artifact (user_id, artifact_id, created_at)
SELECT aa.user_id, m.keep_id, aa.created_at
FROM author_artifact aa
JOIN getuploader_artifact_merge m ON m.duplicate_id = aa.artifact_id
ON CONFLICT (artifact_id, user_id) DO NOTHING;

DELETE FROM author_artifact aa
USING getuploader_artifact_merge m
WHERE aa.artifact_id = m.duplicate_id;

DELETE FROM artifacts a
USING getuploader_artifact_merge m
WHERE a.id = m.duplicate_id;

-- Registry keys are unique. Remove aliases that collapse to the same canonical
-- individual URL before rewriting both stored columns.
WITH canonical AS (
  SELECT normalized_url,
         regexp_replace(
           regexp_replace(normalized_url, '^https://(ux|uu)\\.getuploader\\.com', '', 'i'),
           '/(index|list)/.*$', ''
         ) AS canonical_url,
         row_number() OVER (
           PARTITION BY regexp_replace(
             regexp_replace(normalized_url, '^https://(ux|uu)\\.getuploader\\.com', '', 'i'),
             '/(index|list)/.*$', ''
           )
           ORDER BY CASE WHEN origin = 'catalogued' THEN 0 ELSE 1 END, created_at
         ) AS position
  FROM source_url_registry
  WHERE normalized_url ~* '^https://(ux|uu)\\.getuploader\\.com/'
)
DELETE FROM source_url_registry r
USING canonical c
WHERE r.normalized_url = c.normalized_url AND c.position > 1;

UPDATE source_url_registry
SET normalized_url = regexp_replace(
      regexp_replace(normalized_url, '^https://(ux|uu)\\.getuploader\\.com', '', 'i'),
      '/(index|list)/.*$', ''
    ),
    source_url = regexp_replace(
      regexp_replace(source_url, '^https://(ux|uu)\\.getuploader\\.com', '', 'i'),
      '/(index|list)/.*$', ''
    ),
    updated_at = now()
WHERE normalized_url ~* '^https://(ux|uu)\\.getuploader\\.com/';

COMMIT;
