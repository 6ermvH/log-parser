package service

import (
	"time"

	"github.com/google/uuid"
)

type LogMeta struct {
	ID           uuid.UUID
	Status       string
	UploadedAt   time.Time
	NodesCount   int
	PortsCount   int
	ErrorMessage string
}

type Topology struct {
	Nodes []TopologyNode
	Ports []Port
}

type TopologyNode struct {
	ID    int64
	LogID uuid.UUID
	GUID  string
	Type  string
	Desc  string
}

type Port struct {
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

type NodeDetails struct {
	ID              int64
	LogID           uuid.UUID
	GUID            string
	Type            string
	Desc            string
	SystemImageGUID string
	PortGUID        string
	SwitchInfo      map[string]string
	SystemInfo      map[string]string
	SharpInfo       map[string]string
}
