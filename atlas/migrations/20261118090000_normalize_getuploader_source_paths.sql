-- Store GetUploader sources as host-independent paths and merge uploader-level artifacts.
BEGIN;

-- Canonicalize file and request references. Individual download IDs remain
-- intact; listing pagination paths and query parameters are discarded.
UPDATE file_objects
SET source_url = CASE
      WHEN regexp_replace(
        regexp_replace(source_url, '^https://(ux|uu)[.]getuploader[.]com', '', 'i'),
        '/(index|list)/.*$', ''
      ) LIKE '%/download/%'
      THEN regexp_replace(
        regexp_replace(
          regexp_replace(source_url, '^https://(ux|uu)[.]getuploader[.]com', '', 'i'),
          '/(index|list)/.*$', ''
        ),
        '/+$', ''
      )
      ELSE regexp_replace(
        split_part(
          regexp_replace(
            regexp_replace(source_url, '^https://(ux|uu)[.]getuploader[.]com', '', 'i'),
            '/(index|list)/.*$', ''
          ),
          '?', 1
        ),
        '/+$', ''
      )
    END
WHERE source_url ~* '^https://(ux|uu)[.]getuploader[.]com/'
   OR source_url LIKE '/%';

UPDATE public_requests
SET source_url = CASE
      WHEN regexp_replace(
        regexp_replace(source_url, '^https://(ux|uu)[.]getuploader[.]com', '', 'i'),
        '/(index|list)/.*$', ''
      ) LIKE '%/download/%'
      THEN regexp_replace(
        regexp_replace(
          regexp_replace(source_url, '^https://(ux|uu)[.]getuploader[.]com', '', 'i'),
          '/(index|list)/.*$', ''
        ),
        '/+$', ''
      )
      ELSE regexp_replace(
        split_part(
          regexp_replace(
            regexp_replace(source_url, '^https://(ux|uu)[.]getuploader[.]com', '', 'i'),
            '/(index|list)/.*$', ''
          ),
          '?', 1
        ),
        '/+$', ''
      )
    END
WHERE source_url ~* '^https://(ux|uu)[.]getuploader[.]com/'
   OR source_url LIKE '/%';

-- Artifacts always store the uploader root, never an individual download path.
UPDATE artifacts
SET source_url = regexp_replace(
  split_part(
    regexp_replace(
      regexp_replace(source_url, '^https://(ux|uu)[.]getuploader[.]com', '', 'i'),
      '/(download|index|list)/.*$', ''
    ),
    '?', 1
  ),
  '/+$', ''
)
WHERE source_url ~* '^https://(ux|uu)[.]getuploader[.]com/'
   OR source_url LIKE '/%';

-- Backfill artifact relationships for historical file-level preservation requests.
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
  AND a.source_url = regexp_replace(r.source_url, '/(download|index|list)/.*$', '');

-- Merge duplicate uploader artifacts, preserving the oldest artifact ID.
CREATE TEMP TABLE getuploader_artifact_merge ON COMMIT DROP AS
WITH canonical AS (
  SELECT
    id,
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

-- Convert both absolute GetUploader URLs and existing path keys to one
-- canonical source_url_registry key before resolving the unique-key collisions.
WITH mapped AS (
  SELECT
    normalized_url,
    origin,
    public_request_id,
    created_at,
    CASE
      WHEN normalized_url ~* '^https://(ux|uu)[.]getuploader[.]com/'
      THEN regexp_replace(normalized_url, '^https://(ux|uu)[.]getuploader[.]com', '', 'i')
      ELSE normalized_url
    END AS path_url
  FROM source_url_registry
),
canonical AS (
  SELECT
    normalized_url,
    row_number() OVER (
      PARTITION BY CASE
        WHEN regexp_replace(path_url, '/(index|list)/.*$', '') LIKE '%/download/%'
        THEN regexp_replace(
          regexp_replace(path_url, '/(index|list)/.*$', ''),
          '/+$', ''
        )
        ELSE regexp_replace(
          split_part(regexp_replace(path_url, '/(index|list)/.*$', ''), '?', 1),
          '/+$', ''
        )
      END
      ORDER BY
        (public_request_id IS NOT NULL) DESC,
        CASE WHEN origin = 'catalogued' THEN 0 ELSE 1 END,
        created_at
    ) AS position
  FROM mapped
)
DELETE FROM source_url_registry r
USING canonical c
WHERE r.normalized_url = c.normalized_url
  AND c.position > 1;

UPDATE source_url_registry
SET normalized_url = CASE
      WHEN regexp_replace(
        regexp_replace(normalized_url, '^https://(ux|uu)[.]getuploader[.]com', '', 'i'),
        '/(index|list)/.*$', ''
      ) LIKE '%/download/%'
      THEN regexp_replace(
        regexp_replace(
          regexp_replace(normalized_url, '^https://(ux|uu)[.]getuploader[.]com', '', 'i'),
          '/(index|list)/.*$', ''
        ),
        '/+$', ''
      )
      ELSE regexp_replace(
        split_part(
          regexp_replace(
            regexp_replace(normalized_url, '^https://(ux|uu)[.]getuploader[.]com', '', 'i'),
            '/(index|list)/.*$', ''
          ),
          '?', 1
        ),
        '/+$', ''
      )
    END,
    source_url = CASE
      WHEN regexp_replace(
        regexp_replace(source_url, '^https://(ux|uu)[.]getuploader[.]com', '', 'i'),
        '/(index|list)/.*$', ''
      ) LIKE '%/download/%'
      THEN regexp_replace(
        regexp_replace(
          regexp_replace(source_url, '^https://(ux|uu)[.]getuploader[.]com', '', 'i'),
          '/(index|list)/.*$', ''
        ),
        '/+$', ''
      )
      ELSE regexp_replace(
        split_part(
          regexp_replace(
            regexp_replace(source_url, '^https://(ux|uu)[.]getuploader[.]com', '', 'i'),
            '/(index|list)/.*$', ''
          ),
          '?', 1
        ),
        '/+$', ''
      )
    END,
    updated_at = now()
WHERE normalized_url ~* '^https://(ux|uu)[.]getuploader[.]com/'
   OR normalized_url LIKE '/%';

-- Enforce one artifact per canonical uploader path for future registrations.
CREATE UNIQUE INDEX IF NOT EXISTS artifacts_getuploader_source_path_uidx
  ON artifacts (source_url)
  WHERE source_url LIKE '/%';

COMMIT;
