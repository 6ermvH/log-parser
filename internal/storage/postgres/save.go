package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/6ermvH/log-parser/internal/domain"
)

type portKey struct {
	nodeID  int64
	portNum int
}

const statusOK = "ok"

func (r *Repository) SaveDomainLog(ctx context.Context, logID uuid.UUID, dlog domain.Log) (err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback(ctx) //nolint:errcheck // rollback error is not actionable; original err is reported
		}
	}()

	if _, err = tx.Exec(ctx, `UPDATE logs SET status = $1 WHERE id = $2`, statusOK, logID); err != nil {
		return fmt.Errorf("update log status: %w", err)
	}

	nodeIDByGUID, err := insertNodes(ctx, tx, logID, dlog.Nodes)
	if err != nil {
		return err
	}

	portIDByKey, err := insertPorts(ctx, tx, nodeIDByGUID, dlog.Nodes)
	if err != nil {
		return err
	}

	if err = insertNodesInfo(ctx, tx, nodeIDByGUID, dlog.Nodes); err != nil {
		return err
	}

	if err = insertConnections(ctx, tx, logID, nodeIDByGUID, portIDByKey, dlog.Connections); err != nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

func insertNodes(ctx context.Context, tx pgxTx, logID uuid.UUID, nodes []domain.Node) (map[string]int64, error) {
	result := make(map[string]int64, len(nodes))

	const q = `
		INSERT INTO nodes (log_id, node_guid, node_type, node_desc, system_image_guid, port_guid)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id
	`

	for _, n := range nodes {
		var nodeID int64
		if err := tx.QueryRow(ctx, q,
			logID, n.GUID, n.Type, n.Desc, n.SystemImageGUID, n.PortGUID,
		).Scan(&nodeID); err != nil {
			return nil, fmt.Errorf("insert node %s: %w", n.GUID, err)
		}

		result[n.GUID] = nodeID
	}

	return result, nil
}

func insertPorts(ctx context.Context, tx pgxTx, nodeIDByGUID map[string]int64, nodes []domain.Node) (map[portKey]int64, error) {
	result := make(map[portKey]int64)

	const q = `
		INSERT INTO ports (node_id, port_num, port_guid, port_state, port_phy_state,
		                   link_speed_actv, link_width_actv, lid, raw)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id
	`

	for _, n := range nodes {
		nodeID := nodeIDByGUID[n.GUID]

		for _, p := range n.Ports {
			rawJSON, err := json.Marshal(p.Raw)
			if err != nil {
				return nil, fmt.Errorf("marshal port raw: %w", err)
			}

			var portID int64
			if err := tx.QueryRow(ctx, q,
				nodeID, p.Num, p.GUID, p.State, p.PhyState,
				p.LinkSpeedActv, p.LinkWidthActv, p.LID, rawJSON,
			).Scan(&portID); err != nil {
				return nil, fmt.Errorf("insert port %s:%d: %w", n.GUID, p.Num, err)
			}

			result[portKey{nodeID, p.Num}] = portID
		}
	}

	return result, nil
}

func insertNodesInfo(ctx context.Context, tx pgxTx, nodeIDByGUID map[string]int64, nodes []domain.Node) error {
	const q = `
		INSERT INTO nodes_info (node_id, switch_info, system_info, sharp_info)
		VALUES ($1, $2, $3, $4)
	`

	for _, n := range nodes {
		if !hasAnyInfo(n.Info) {
			continue
		}

		nodeID := nodeIDByGUID[n.GUID]

		sw, err := jsonOrNull(n.Info.SwitchInfo)
		if err != nil {
			return fmt.Errorf("marshal switch_info: %w", err)
		}

		sys, err := jsonOrNull(n.Info.SystemInfo)
		if err != nil {
			return fmt.Errorf("marshal system_info: %w", err)
		}

		sharp, err := jsonOrNull(n.Info.SharpInfo)
		if err != nil {
			return fmt.Errorf("marshal sharp_info: %w", err)
		}

		if _, err := tx.Exec(ctx, q, nodeID, sw, sys, sharp); err != nil {
			return fmt.Errorf("insert nodes_info %s: %w", n.GUID, err)
		}
	}

	return nil
}

func insertConnections(
	ctx context.Context,
	tx pgxTx,
	logID uuid.UUID,
	nodeIDByGUID map[string]int64,
	portIDByKey map[portKey]int64,
	conns []domain.Connection,
) error {
	const q = `
		INSERT INTO connections (log_id, port_a_id, port_b_id)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
	`

	for _, c := range conns {
		nodeAID, okA := nodeIDByGUID[c.NodeAGUID]
		nodeBID, okB := nodeIDByGUID[c.NodeBGUID]

		if !okA || !okB {
			continue
		}

		portAID, okPA := portIDByKey[portKey{nodeAID, c.PortANum}]
		portBID, okPB := portIDByKey[portKey{nodeBID, c.PortBNum}]

		if !okPA || !okPB || portAID == portBID {
			continue
		}

		if portAID > portBID {
			portAID, portBID = portBID, portAID
		}

		if _, err := tx.Exec(ctx, q, logID, portAID, portBID); err != nil {
			return fmt.Errorf("insert connection: %w", err)
		}
	}

	return nil
}

func hasAnyInfo(info *domain.NodeInfo) bool {
	return info != nil && (info.SwitchInfo != nil || info.SystemInfo != nil || info.SharpInfo != nil)
}

func jsonOrNull(m map[string]string) (any, error) {
	if m == nil {
		return nil, nil
	}

	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	return b, nil
}
