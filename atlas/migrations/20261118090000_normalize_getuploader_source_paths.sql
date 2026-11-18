-- Store GetUploader URLs as host-independent paths and merge uploader artifacts.
BEGIN;

-- File and request records retain individual download IDs. Other paths,
-- including listing pagination paths, collapse to the uploader root.
WITH cleaned AS (
  SELECT id,
         regexp_replace(source_url, '^https://(ux|uu)[.]getuploader[.]com', '', 'i') AS path
  FROM file_objects
  WHERE source_url ~* '^https://(ux|uu)[.]getuploader[.]com/'
     OR source_url LIKE '/%'
),
normalized AS (
  SELECT id,
         CASE
           WHEN split_part(path, '?', 1) ~ '^/[^/]+/download/[^/]+$'
             THEN regexp_replace(path, '/+$', '')
           ELSE '/' || split_part(trim(both '/' FROM split_part(path, '?', 1)), '/', 1)
         END AS source_url
  FROM cleaned
)
UPDATE file_objects f
SET source_url = n.source_url
FROM normalized n
WHERE f.id = n.id;

WITH cleaned AS (
  SELECT id,
         regexp_replace(source_url, '^https://(ux|uu)[.]getuploader[.]com', '', 'i') AS path
  FROM public_requests
  WHERE source_url ~* '^https://(ux|uu)[.]getuploader[.]com/'
     OR source_url LIKE '/%'
),
normalized AS (
  SELECT id,
         CASE
           WHEN split_part(path, '?', 1) ~ '^/[^/]+/download/[^/]+$'
             THEN regexp_replace(path, '/+$', '')
           ELSE '/' || split_part(trim(both '/' FROM split_part(path, '?', 1)), '/', 1)
         END AS source_url
  FROM cleaned
)
UPDATE public_requests r
SET source_url = n.source_url
FROM normalized n
WHERE r.id = n.id;

-- Artifacts always point to the first path segment (the uploader root).
WITH cleaned AS (
  SELECT id,
         regexp_replace(source_url, '^https://(ux|uu)[.]getuploader[.]com', '', 'i') AS path
  FROM artifacts
  WHERE source_url ~* '^https://(ux|uu)[.]getuploader[.]com/'
     OR source_url LIKE '/%'
),
normalized AS (
  SELECT id,
         '/' || split_part(trim(both '/' FROM split_part(path, '?', 1)), '/', 1) AS source_url
  FROM cleaned
)
UPDATE artifacts a
SET source_url = n.source_url
FROM normalized n
WHERE a.id = n.id;

-- Link historical requests to the Artifact owning their uploader root.
UPDATE public_requests r
SET artifact_id = f.artifact_id
FROM file_objects f
WHERE r.file_object_id = f.id
  AND r.artifact_id IS NULL
  AND r.request_type = 'preservation';

UPDATE public_requests r
SET artifact_id = a.id
FROM artifacts a
WHERE r.artifact_id IS NULL
  AND r.request_type = 'preservation'
  AND r.source_url LIKE '/%'
  AND a.source_url = '/' || split_part(
    trim(both '/' FROM split_part(r.source_url, '?', 1)), '/', 1
  );

-- Merge duplicate uploader Artifacts, preserving the oldest ID.
CREATE TEMP TABLE getuploader_artifact_merge ON COMMIT DROP AS
WITH canonical AS (
  SELECT id,
         first_value(id) OVER (
           PARTITION BY source_url
           ORDER BY created_at, id
         ) AS keep_id
  FROM artifacts
  WHERE source_url LIKE '/%'
)
SELECT id AS duplicate_id, keep_id
FROM canonical
WHERE id <> keep_id;

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

-- Deduplicate only GetUploader absolute URLs and paths. Other hosts must not
-- be folded together because their paths can be meaningful independent keys.
WITH mapped AS (
  SELECT normalized_url,
         origin,
         public_request_id,
         created_at,
         CASE
           WHEN normalized_url ~* '^https://(ux|uu)[.]getuploader[.]com/'
             THEN regexp_replace(normalized_url, '^https://(ux|uu)[.]getuploader[.]com', '', 'i')
           ELSE normalized_url
         END AS path_url
  FROM source_url_registry
  WHERE normalized_url ~* '^https://(ux|uu)[.]getuploader[.]com/'
     OR normalized_url LIKE '/%'
),
canonical AS (
  SELECT normalized_url,
         CASE
           WHEN split_part(path_url, '?', 1) ~ '^/[^/]+/download/[^/]+$'
             THEN regexp_replace(path_url, '/+$', '')
           ELSE '/' || split_part(trim(both '/' FROM split_part(path_url, '?', 1)), '/', 1)
         END AS canonical_url,
         row_number() OVER (
           PARTITION BY CASE
             WHEN split_part(path_url, '?', 1) ~ '^/[^/]+/download/[^/]+$'
               THEN regexp_replace(path_url, '/+$', '')
             ELSE '/' || split_part(trim(both '/' FROM split_part(path_url, '?', 1)), '/', 1)
           END
           ORDER BY (public_request_id IS NOT NULL) DESC,
                    CASE WHEN origin = 'catalogued' THEN 0 ELSE 1 END,
                    created_at
         ) AS position
  FROM mapped
)
DELETE FROM source_url_registry r
USING canonical c
WHERE r.normalized_url = c.normalized_url
  AND c.position > 1;

-- Rewrite registry keys and display URLs using the same storage rule.
WITH mapped AS (
  SELECT normalized_url,
         source_url,
         CASE
           WHEN normalized_url ~* '^https://(ux|uu)[.]getuploader[.]com/'
             THEN regexp_replace(normalized_url, '^https://(ux|uu)[.]getuploader[.]com', '', 'i')
           ELSE normalized_url
         END AS path_url,
         CASE
           WHEN source_url ~* '^https://(ux|uu)[.]getuploader[.]com/'
             THEN regexp_replace(source_url, '^https://(ux|uu)[.]getuploader[.]com', '', 'i')
           ELSE source_url
         END AS source_path
  FROM source_url_registry
  WHERE normalized_url ~* '^https://(ux|uu)[.]getuploader[.]com/'
     OR normalized_url LIKE '/%'
),
normalized AS (
  SELECT normalized_url,
         CASE
           WHEN split_part(path_url, '?', 1) ~ '^/[^/]+/download/[^/]+$'
             THEN regexp_replace(path_url, '/+$', '')
           ELSE '/' || split_part(trim(both '/' FROM split_part(path_url, '?', 1)), '/', 1)
         END AS canonical_url,
         CASE
           WHEN source_path ~ '^/[^/]+/download/[^/]+$'
             THEN regexp_replace(source_path, '/+$', '')
           WHEN source_path LIKE '/%'
             THEN '/' || split_part(trim(both '/' FROM split_part(source_path, '?', 1)), '/', 1)
           ELSE source_path
         END AS canonical_source_url
  FROM mapped
)
UPDATE source_url_registry r
SET normalized_url = n.canonical_url,
    source_url = n.canonical_source_url,
    updated_at = now()
FROM normalized n
WHERE r.normalized_url = n.normalized_url;

-- Enforce one Artifact per canonical uploader path for future registrations.
CREATE UNIQUE INDEX IF NOT EXISTS artifacts_getuploader_source_path_uidx
  ON artifacts (source_url)
  WHERE source_url LIKE '/%';

COMMIT;
