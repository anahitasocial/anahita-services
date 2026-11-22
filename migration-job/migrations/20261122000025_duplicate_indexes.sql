-- +goose Up
-- Indexes that were each the same as another on the same columns (roadmap
-- row 170, phase 4): every write to nodes and edges kept both up to date,
-- and no query could choose one over its twin. The twin of each stays:
-- the primary key for idx_nodes_id, and the KEY of the same column for the
-- rest. Measured on a copy of banu.io's data, 2026-11-22.
DROP INDEX idx_edges_node_a_id ON edges;
DROP INDEX idx_edges_node_b_id ON edges;
DROP INDEX idx_edges_type ON edges;
DROP INDEX idx_nodes_id ON nodes;
DROP INDEX idx_nodes_owner_id ON nodes;
DROP INDEX idx_nodes_type ON nodes;
DROP INDEX idx_nodes_parent_id ON nodes;

-- +goose Down
CREATE INDEX idx_edges_node_a_id ON edges (node_a_id);
CREATE INDEX idx_edges_node_b_id ON edges (node_b_id);
CREATE INDEX idx_edges_type ON edges (type);
CREATE INDEX idx_nodes_id ON nodes (id);
CREATE INDEX idx_nodes_owner_id ON nodes (owner_id);
CREATE INDEX idx_nodes_type ON nodes (type);
CREATE INDEX idx_nodes_parent_id ON nodes (parent_id);
