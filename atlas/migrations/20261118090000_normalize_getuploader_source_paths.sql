-- Canonicalize GetUploader URLs, merge uploader Artifacts, and retain independent registries.
BEGIN;

-- File and request rows retain the individual download ID; listing paths collapse to the root.
WITH cleaned AS (
  SELECT id, regexp_replace(source_url, '^https://(ux|uu)[.]getuploader[.]com(:443)?', '', 'i') AS path
  FROM file_objects
  WHERE source_url ~* '^https://(ux|uu)[.]getuploader[.]com(:443)?/' OR source_url LIKE '/%'
),
normalized AS (
  SELECT id, CASE
    WHEN rtrim(split_part(path, '?', 1), '/') ~* '^/[^/]+/download/[^/]+(/.*)?$'
      THEN regexp_replace(rtrim(split_part(path, '?', 1), '/'), '^(/[^/]+/)[^/]+/([^/]+).*$', '\\1download/\\2') || coalesce(substring(path from '\\?.*$'), '')
    ELSE '/' || split_part(trim(both '/' FROM split_part(path, '?', 1)), '/', 1)
  END AS source_url
  FROM cleaned
)
UPDATE file_objects f SET source_url = n.source_url FROM normalized n WHERE f.id = n.id;

WITH cleaned AS (
  SELECT id, regexp_replace(source_url, '^https://(ux|uu)[.]getuploader[.]com(:443)?', '', 'i') AS path
  FROM public_requests
  WHERE source_url ~* '^https://(ux|uu)[.]getuploader[.]com(:443)?/' OR source_url LIKE '/%'
),
normalized AS (
  SELECT id, CASE
    WHEN rtrim(split_part(path, '?', 1), '/') ~* '^/[^/]+/download/[^/]+(/.*)?$'
      THEN regexp_replace(rtrim(split_part(path, '?', 1), '/'), '^(/[^/]+/)[^/]+/([^/]+).*$', '\\1download/\\2') || coalesce(substring(path from '\\?.*$'), '')
    ELSE '/' || split_part(trim(both '/' FROM split_part(path, '?', 1)), '/', 1)
  END AS source_url
  FROM cleaned
)
UPDATE public_requests r SET source_url = n.source_url FROM normalized n WHERE r.id = n.id;

-- Artifacts must contain only the uploader root, including historical /name/pageN values.
WITH cleaned AS (
  SELECT id, regexp_replace(source_url, '^https://(ux|uu)[.]getuploader[.]com(:443)?', '', 'i') AS path
  FROM artifacts
  WHERE source_url ~* '^https://(ux|uu)[.]getuploader[.]com(:443)?/' OR source_url LIKE '/%'
),
normalized AS (
  SELECT id, '/' || split_part(trim(both '/' FROM split_part(path, '?', 1)), '/', 1) AS source_url
  FROM cleaned
)
UPDATE artifacts a SET source_url = n.source_url FROM normalized n WHERE a.id = n.id;

-- Backfill historical preservation-request relationships.
UPDATE public_requests r
SET artifact_id = f.artifact_id
FROM file_objects f
WHERE r.file_object_id = f.id AND r.artifact_id IS NULL AND r.request_type = 'preservation';

UPDATE public_requests r
SET artifact_id = a.id
FROM artifacts a
WHERE r.artifact_id IS NULL
  AND r.request_type = 'preservation'
  AND r.source_url LIKE '/%'
  AND a.source_url = '/' || split_part(trim(both '/' FROM split_part(r.source_url, '?', 1)), '/', 1);

-- Preserve the oldest Artifact ID per uploader and repoint all dependent rows.
CREATE TEMP TABLE getuploader_artifact_merge ON COMMIT DROP AS
WITH canonical AS (
  SELECT id, first_value(id) OVER (PARTITION BY source_url ORDER BY created_at, id) AS keep_id
  FROM artifacts
  WHERE source_url LIKE '/%'
)
SELECT id AS duplicate_id, keep_id FROM canonical WHERE id <> keep_id;

UPDATE file_objects f SET artifact_id = m.keep_id
FROM getuploader_artifact_merge m WHERE f.artifact_id = m.duplicate_id;

UPDATE public_requests r SET artifact_id = m.keep_id
FROM getuploader_artifact_merge m WHERE r.artifact_id = m.duplicate_id;

INSERT INTO author_artifact (user_id, artifact_id, created_at)
SELECT aa.user_id, m.keep_id, aa.created_at
FROM author_artifact aa JOIN getuploader_artifact_merge m ON m.duplicate_id = aa.artifact_id
ON CONFLICT (artifact_id, user_id) DO NOTHING;

DELETE FROM author_artifact aa USING getuploader_artifact_merge m WHERE aa.artifact_id = m.duplicate_id;
DELETE FROM artifacts a USING getuploader_artifact_merge m WHERE a.id = m.duplicate_id;

-- Only GetUploader absolute URLs and our stored path keys participate in registry merging.
WITH mapped AS (
  SELECT normalized_url, origin, public_request_id, created_at,
         CASE WHEN normalized_url ~* '^https://(ux|uu)[.]getuploader[.]com(:443)?/'
           THEN regexp_replace(normalized_url, '^https://(ux|uu)[.]getuploader[.]com(:443)?', '', 'i')
           ELSE normalized_url END AS path_url
  FROM source_url_registry
  WHERE normalized_url ~* '^https://(ux|uu)[.]getuploader[.]com(:443)?/' OR normalized_url LIKE '/%'
),
canonical AS (
  SELECT normalized_url,
         CASE WHEN rtrim(split_part(path_url, '?', 1), '/') ~* '^/[^/]+/download/[^/]+(/.*)?$'
           THEN regexp_replace(rtrim(split_part(path_url, '?', 1), '/'), '^(/[^/]+/)[^/]+/([^/]+).*$', '\\1download/\\2') || coalesce(substring(path_url from '\\?.*$'), '')
           ELSE '/' || split_part(trim(both '/' FROM split_part(path_url, '?', 1)), '/', 1) END AS canonical_url,
         row_number() OVER (
           PARTITION BY CASE WHEN rtrim(split_part(path_url, '?', 1), '/') ~* '^/[^/]+/download/[^/]+(/.*)?$'
             THEN regexp_replace(rtrim(split_part(path_url, '?', 1), '/'), '^(/[^/]+/)[^/]+/([^/]+).*$', '\\1download/\\2') || coalesce(substring(path_url from '\\?.*$'), '')
             ELSE '/' || split_part(trim(both '/' FROM split_part(path_url, '?', 1)), '/', 1) END
           ORDER BY (public_request_id IS NOT NULL) DESC,
                    CASE WHEN origin = 'catalogued' THEN 0 ELSE 1 END, created_at
         ) AS position
  FROM mapped
)
DELETE FROM source_url_registry r
USING canonical c
WHERE r.normalized_url = c.normalized_url AND c.position > 1;

-- Rewrite only matching registry records; query strings remain on individual download URLs.
WITH mapped AS (
  SELECT normalized_url, source_url,
         CASE WHEN normalized_url ~* '^https://(ux|uu)[.]getuploader[.]com(:443)?/'
           THEN regexp_replace(normalized_url, '^https://(ux|uu)[.]getuploader[.]com(:443)?', '', 'i')
           ELSE normalized_url END AS path_url,
         CASE WHEN source_url ~* '^https://(ux|uu)[.]getuploader[.]com(:443)?/'
           THEN regexp_replace(source_url, '^https://(ux|uu)[.]getuploader[.]com(:443)?', '', 'i')
           ELSE source_url END AS source_path
  FROM source_url_registry
  WHERE normalized_url ~* '^https://(ux|uu)[.]getuploader[.]com(:443)?/' OR normalized_url LIKE '/%'
),
normalized AS (
  SELECT normalized_url,
         CASE WHEN rtrim(split_part(path_url, '?', 1), '/') ~* '^/[^/]+/download/[^/]+(/.*)?$'
           THEN regexp_replace(rtrim(split_part(path_url, '?', 1), '/'), '^(/[^/]+/)[^/]+/([^/]+).*$', '\\1download/\\2') || coalesce(substring(path_url from '\\?.*$'), '')
           ELSE '/' || split_part(trim(both '/' FROM split_part(path_url, '?', 1)), '/', 1) END AS canonical_url,
         CASE
           WHEN rtrim(split_part(source_path, '?', 1), '/') ~* '^/[^/]+/download/[^/]+(/.*)?$'
             THEN regexp_replace(rtrim(split_part(source_path, '?', 1), '/'), '^(/[^/]+/)[^/]+/([^/]+).*$', '\\1download/\\2') || coalesce(substring(source_path from '\\?.*$'), '')
           WHEN source_path LIKE '/%' THEN '/' || split_part(trim(both '/' FROM split_part(source_path, '?', 1)), '/', 1)
           ELSE source_path
         END AS canonical_source_url
  FROM mapped
)
UPDATE source_url_registry r
SET normalized_url = n.canonical_url, source_url = n.canonical_source_url, updated_at = now()
FROM normalized n
WHERE r.normalized_url = n.normalized_url;

CREATE UNIQUE INDEX IF NOT EXISTS artifacts_getuploader_source_path_uidx
  ON artifacts (source_url) WHERE source_url LIKE '/%';

COMMIT;
