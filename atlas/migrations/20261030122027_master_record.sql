ALTER TABLE public.artifacts
RENAME TO m_artifacts;

ALTER TABLE public.users
RENAME TO m_users;

ALTER TABLE public.file_objects
RENAME TO m_file_objects;

ALTER TABLE public.nodes
RENAME TO m_nodes;

ALTER TABLE public.audit_logs
RENAME TO t_audit_logs;

ALTER TABLE public.download_jobs
RENAME TO t_download_jobs;

ALTER TABLE public.download_audit_logs
RENAME TO t_download_audit_logs;

ALTER TABLE public.node_enrollment_requests
RENAME TO t_node_enrollment_requests;

ALTER TABLE public.placements
RENAME TO t_placements;

ALTER TABLE public.policies
RENAME TO t_policies;

ALTER TABLE public.public_requests
RENAME TO t_public_requests;

ALTER TABLE public.request_conversation_links
RENAME TO t_request_conversation_links;

ALTER TABLE public.source_url_registry
RENAME TO t_source_url_registry;

ALTER TABLE public.tasks
RENAME TO t_tasks;