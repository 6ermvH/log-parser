package parser

import (
	"bufio"
	"io"
	"strconv"
	"strings"

	"github.com/6ermvH/log-parser/internal/domain"
)

const (
	suffixDBCSV    = ".db_csv"
	suffixSharpAN  = ".sharp_an_info"
	guidPrefix     = "0x"
	sharpGUIDLabel = "SW_GUID="
	sharpDelimiter = "---"

	sectionNodes      = "NODES"
	sectionPorts      = "PORTS"
	sectionSwitches   = "SWITCHES"
	sectionSystemInfo = "SYSTEM_GENERAL_INFORMATION"

	colNodeGUID        = "NodeGUID"
	colNodeGUIDAlt     = "NodeGuid"
	colNodeType        = "NodeType"
	colNodeDesc        = "NodeDesc"
	colSystemImageGUID = "SystemImageGUID"
	colPortGUID        = "PortGUID"
	colPortGUIDAlt     = "PortGuid"
	colPortNum         = "PortNum"
	colPortState       = "PortState"
	colPortPhyState    = "PortPhyState"
	colLinkSpeedActv   = "LinkSpeedActv"
	colLinkWidthActv   = "LinkWidthActv"
	colLID             = "LID"

	nodeTypeCodeHost   = "1"
	nodeTypeCodeSwitch = "2"
	nodeTypeCodeRouter = "3"
)

type Aggregator struct {
	nodes map[string]*domain.Node
}

func NewAggregator() *Aggregator {
	return &Aggregator{nodes: make(map[string]*domain.Node)}
}

func (a *Aggregator) AnalyzeFile(name string, r io.Reader) error {
	switch {
	case strings.HasSuffix(name, suffixDBCSV):
		sm := newSectionMachine(a.handleEvent)

		return sm.Run(r)
	case strings.HasSuffix(name, suffixSharpAN):
		return a.analyzeSharp(r)
	}

	return nil
}

func (a *Aggregator) Result() domain.Log {
	nodes := make([]domain.Node, 0, len(a.nodes))
	for _, n := range a.nodes {
		nodes = append(nodes, *n)
	}

	return domain.Log{Nodes: nodes}
}

func (a *Aggregator) handleEvent(e sectionEvent) {
	switch e.Name {
	case sectionNodes:
		a.addNode(e.Columns, e.Row)
	case sectionPorts:
		a.addPort(e.Columns, e.Row)
	case sectionSwitches:
		a.upsertSwitchInfo(e.Columns, e.Row)
	case sectionSystemInfo:
		a.upsertSystemInfo(e.Columns, e.Row)
	}
}

func (a *Aggregator) addNode(cols, row []string) {
	m := assocColumns(cols, row)

	guid := m[colNodeGUID]
	if guid == "" {
		return
	}

	a.nodes[guid] = &domain.Node{
		GUID:            guid,
		Type:            mapNodeType(m[colNodeType]),
		Desc:            strings.Trim(m[colNodeDesc], `"`),
		SystemImageGUID: m[colSystemImageGUID],
		PortGUID:        m[colPortGUID],
	}
}

func (a *Aggregator) addPort(cols, row []string) {
	m := assocColumns(cols, row)

	guid := firstNonEmpty(m[colNodeGUID], m[colNodeGUIDAlt])

	node, ok := a.nodes[guid]
	if !ok {
		return
	}

	node.Ports = append(node.Ports, domain.Port{
		Num:           parseIntOrZero(m[colPortNum]),
		GUID:          firstNonEmpty(m[colPortGUID], m[colPortGUIDAlt]),
		State:         parseIntOrZero(m[colPortState]),
		PhyState:      parseIntOrZero(m[colPortPhyState]),
		LinkSpeedActv: parseIntOrZero(m[colLinkSpeedActv]),
		LinkWidthActv: parseIntOrZero(m[colLinkWidthActv]),
		LID:           parseIntOrZero(m[colLID]),
		Raw:           m,
	})
}

func (a *Aggregator) upsertSwitchInfo(cols, row []string) {
	m := assocColumns(cols, row)

	guid := firstNonEmpty(m[colNodeGUID], m[colNodeGUIDAlt])

	node, ok := a.nodes[guid]
	if !ok {
		return
	}

	if node.Info == nil {
		node.Info = &domain.NodeInfo{}
	}

	node.Info.SwitchInfo = m
}

func (a *Aggregator) upsertSystemInfo(cols, row []string) {
	m := assocColumns(cols, row)

	guid := firstNonEmpty(m[colNodeGUID], m[colNodeGUIDAlt])

	node, ok := a.nodes[guid]
	if !ok {
		return
	}

	if node.Info == nil {
		node.Info = &domain.NodeInfo{}
	}

	node.Info.SystemInfo = m
}

func (a *Aggregator) analyzeSharp(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, scannerBufferInitial), scannerBufferMax)

	var (
		currentGUID string
		current     map[string]string
	)

	commit := func() {
		if currentGUID == "" || current == nil {
			return
		}

		node, ok := a.nodes[currentGUID]
		if !ok {
			return
		}

		if node.Info == nil {
			node.Info = &domain.NodeInfo{}
		}

		node.Info.SharpInfo = current
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, sharpDelimiter) {
			continue
		}

		if strings.HasPrefix(line, sharpGUIDLabel) {
			commit()

			raw := strings.TrimPrefix(line, sharpGUIDLabel)
			if !strings.HasPrefix(raw, guidPrefix) {
				raw = guidPrefix + raw
			}

			currentGUID = raw
			current = make(map[string]string)

			continue
		}

		if current == nil {
			continue
		}

		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}

		current[strings.TrimSpace(line[:idx])] = strings.TrimSpace(line[idx+1:])
	}

	commit()

	return scanner.Err()
}

func assocColumns(keys, vals []string) map[string]string {
	n := min(len(keys), len(vals))

	m := make(map[string]string, n)
	for i := range n {
		m[keys[i]] = vals[i]
	}

	return m
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}

	return ""
}

func parseIntOrZero(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}

	return n
}

func mapNodeType(code string) string {
	switch code {
	case nodeTypeCodeHost:
		return domain.NodeTypeHost
	case nodeTypeCodeSwitch:
		return domain.NodeTypeSwitch
	case nodeTypeCodeRouter:
		return domain.NodeTypeRouter
	default:
		return domain.NodeTypeUnknown
	}
}
