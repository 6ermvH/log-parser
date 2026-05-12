package domain

const (
	NodeTypeHost    = "host"
	NodeTypeSwitch  = "switch"
	NodeTypeRouter  = "router"
	NodeTypeUnknown = "unknown"
)

type Log struct {
	Nodes       []Node
	Connections []Connection
}

type Node struct {
	GUID            string
	Type            string
	Desc            string
	SystemImageGUID string
	PortGUID        string
	Ports           []Port
	Info            *NodeInfo
}

type Port struct {
	Num           int
	GUID          string
	State         int
	PhyState      int
	LinkSpeedActv int
	LinkWidthActv int
	LID           int
	Raw           map[string]string
}

type NodeInfo struct {
	SwitchInfo map[string]string
	SystemInfo map[string]string
	SharpInfo  map[string]string
}

type Connection struct {
	NodeAGUID string
	PortANum  int
	NodeBGUID string
	PortBNum  int
}
