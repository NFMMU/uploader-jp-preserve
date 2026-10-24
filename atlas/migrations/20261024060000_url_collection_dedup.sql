-- Reserve each canonical source URL once so manual and automated collection
-- requests cannot enqueue duplicate work concurrently.
CREATE TABLE source_url_registry (
  normalized_url text PRIMARY KEY,
  source_url text NOT NULL,
  origin varchar(32) NOT NULL CHECK (origin IN ('public_request', 'auto_collection', 'catalogued')),
  public_request_id uuid REFERENCES public_requests(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX source_url_registry_request_idx ON source_url_registry (public_request_id);
