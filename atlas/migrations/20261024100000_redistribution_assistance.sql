ALTER TABLE public_requests
  ADD COLUMN rights_holder_contact varchar(1000),
  ADD COLUMN intake_file_path text NOT NULL DEFAULT '',
  ADD COLUMN intake_file_name varchar(500),
  ADD COLUMN intake_file_size bigint,
  ADD COLUMN intake_file_sha256 varchar(64),
  ADD COLUMN intake_file_uploaded_at timestamptz;

ALTER TABLE public_requests DROP CONSTRAINT public_requests_type_check;
ALTER TABLE public_requests ADD CONSTRAINT public_requests_type_check
  CHECK (request_type IN ('takedown', 'deletion', 'external_publication', 'preservation', 'redistribution_assistance', 'publisher_assistance'));
