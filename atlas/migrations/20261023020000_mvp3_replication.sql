-- MVP-3: source-node claim index for verified-placement replication tasks.
CREATE INDEX tasks_source_node_status_idx ON tasks (source_node_id, status);
