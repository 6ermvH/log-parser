package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type NodeRow struct {
	ID              int64
	LogID           uuid.UUID
	GUID            string
	Type            string
	Desc            string
	SystemImageGUID string
	PortGUID        string
}

type PortRow struct {
	ID            int64
	NodeID        int64
	Num           int
	GUID          string
	State         int
	PhyState      int
	LinkSpeedActv int
	LinkWidthActv int
	LID           int
	Raw           map[string]string
}

type NodeInfoRow struct {
	SwitchInfo map[string]string
	SystemInfo map[string]string
	SharpInfo  map[string]string
}

func (r *Repository) ListNodes(ctx context.Context, logID uuid.UUID) ([]NodeRow, error) {
	const q = `
		SELECT id, log_id, node_guid, node_type, node_desc, system_image_guid, port_guid
		FROM nodes
		WHERE log_id = $1
		ORDER BY id
	`

	rows, err := r.pool.Query(ctx, q, logID)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	defer rows.Close()

	var out []NodeRow

	for rows.Next() {
		var n NodeRow
		if err := rows.Scan(&n.ID, &n.LogID, &n.GUID, &n.Type, &n.Desc, &n.SystemImageGUID, &n.PortGUID); err != nil {
			return nil, fmt.Errorf("scan node: %w", err)
		}

		out = append(out, n)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate nodes: %w", err)
	}

	return out, nil
}

func (r *Repository) ListPortsByLog(ctx context.Context, logID uuid.UUID) ([]PortRow, error) {
	const q = `
		SELECT p.id, p.node_id, p.port_num, p.port_guid, p.port_state, p.port_phy_state,
		       p.link_speed_actv, p.link_width_actv, p.lid, p.raw
		FROM ports p
		JOIN nodes n ON n.id = p.node_id
		WHERE n.log_id = $1
		ORDER BY p.node_id, p.port_num
	`

	return scanPorts(ctx, r, q, logID)
}

func (r *Repository) ListPortsByNode(ctx context.Context, nodeID int64) ([]PortRow, error) {
	const q = `
		SELECT id, node_id, port_num, port_guid, port_state, port_phy_state,
		       link_speed_actv, link_width_actv, lid, raw
		FROM ports
		WHERE node_id = $1
		ORDER BY port_num
	`

	return scanPorts(ctx, r, q, nodeID)
}

func (r *Repository) GetNode(ctx context.Context, id int64) (NodeRow, error) {
	const q = `
		SELECT id, log_id, node_guid, node_type, node_desc, system_image_guid, port_guid
		FROM nodes
		WHERE id = $1
	`

	var n NodeRow
	if err := r.pool.QueryRow(ctx, q, id).Scan(
		&n.ID, &n.LogID, &n.GUID, &n.Type, &n.Desc, &n.SystemImageGUID, &n.PortGUID,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return NodeRow{}, ErrNotFound
		}

		return NodeRow{}, fmt.Errorf("get node: %w", err)
	}

	return n, nil
}

func (r *Repository) GetNodeInfo(ctx context.Context, nodeID int64) (NodeInfoRow, bool, error) {
	const q = `
		SELECT switch_info, system_info, sharp_info
		FROM nodes_info
		WHERE node_id = $1
	`

	var swRaw, sysRaw, sharpRaw []byte
	if err := r.pool.QueryRow(ctx, q, nodeID).Scan(&swRaw, &sysRaw, &sharpRaw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return NodeInfoRow{}, false, nil
		}

		return NodeInfoRow{}, false, fmt.Errorf("get node_info: %w", err)
	}

	info := NodeInfoRow{
		SwitchInfo: unmarshalMap(swRaw),
		SystemInfo: unmarshalMap(sysRaw),
		SharpInfo:  unmarshalMap(sharpRaw),
	}

	return info, true, nil
}

func (r *Repository) NodeExists(ctx context.Context, id int64) (bool, error) {
	const q = `SELECT EXISTS (SELECT 1 FROM nodes WHERE id = $1)`

	var exists bool
	if err := r.pool.QueryRow(ctx, q, id).Scan(&exists); err != nil {
		return false, fmt.Errorf("node exists: %w", err)
	}

	return exists, nil
}

func scanPorts(ctx context.Context, r *Repository, q string, arg any) ([]PortRow, error) {
	rows, err := r.pool.Query(ctx, q, arg)
	if err != nil {
		return nil, fmt.Errorf("query ports: %w", err)
	}
	defer rows.Close()

	var out []PortRow

	for rows.Next() {
		var (
			p      PortRow
			rawRaw []byte
		)

		if err := rows.Scan(
			&p.ID, &p.NodeID, &p.Num, &p.GUID, &p.State, &p.PhyState,
			&p.LinkSpeedActv, &p.LinkWidthActv, &p.LID, &rawRaw,
		); err != nil {
			return nil, fmt.Errorf("scan port: %w", err)
		}

		p.Raw = unmarshalMap(rawRaw)

		out = append(out, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ports: %w", err)
	}

	return out, nil
}

func unmarshalMap(b []byte) map[string]string {
	if len(b) == 0 {
		return nil
	}

	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}

	return m
}
