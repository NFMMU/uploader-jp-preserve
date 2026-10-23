-- MVP-2: node credentials and task leases.
-- Node credentials are stored only as SHA-256 hashes; the raw token is returned once at enrollment.
ALTER TABLE nodes ADD COLUMN api_token_hash varchar(64);
CREATE UNIQUE INDEX nodes_api_token_hash_key ON nodes (api_token_hash);

-- A lease token prevents an old worker attempt from reporting a result after a task has been reclaimed.
ALTER TABLE tasks ADD COLUMN lease_until timestamptz;
ALTER TABLE tasks ADD COLUMN lease_token uuid;
CREATE UNIQUE INDEX tasks_lease_token_key ON tasks (lease_token);
