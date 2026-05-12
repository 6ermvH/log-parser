CREATE INDEX idx_nodes_log_id_node_type ON nodes (log_id, node_type);

CREATE INDEX idx_logs_uploaded_at ON logs (uploaded_at DESC);

CREATE INDEX idx_logs_processing_stale ON logs (uploaded_at) WHERE status = 'processing';
