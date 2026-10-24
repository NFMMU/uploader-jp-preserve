ALTER TABLE file_objects ADD COLUMN source_url text;

ALTER TABLE public_requests
  ADD COLUMN title varchar(500),
  ADD COLUMN creator_name varchar(255),
  ADD COLUMN source_url text;

ALTER TABLE public_requests DROP CONSTRAINT public_requests_type_check;
ALTER TABLE public_requests ADD CONSTRAINT public_requests_type_check
  CHECK (request_type IN ('takedown', 'deletion', 'external_publication', 'preservation'));
