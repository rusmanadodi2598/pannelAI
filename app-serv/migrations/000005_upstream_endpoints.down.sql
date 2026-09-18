DROP INDEX IF EXISTS idx_upstream_keys_status;
DROP INDEX IF EXISTS idx_upstream_keys_endpoint_priority;
DROP TABLE IF EXISTS upstream_keys;
DROP INDEX IF EXISTS idx_upstream_endpoints_status;
DROP INDEX IF EXISTS idx_upstream_endpoints_provider_priority;
DROP INDEX IF EXISTS idx_upstream_endpoints_provider_label;
DROP TABLE IF EXISTS upstream_endpoints;