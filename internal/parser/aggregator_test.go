//go:build !integration

package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/6ermvH/log-parser/internal/domain"
)

func TestAggregator_NodesAndPorts(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"START_NODES",
		"NodeDesc,NumPorts,NodeType,SystemImageGUID,NodeGUID,PortGUID",
		`"HOST_1",1,1,0xhost1,0xhost1,0xhost1`,
		`"SWITCH_1",65,2,0xswitch1,0xswitch1,0xswitch1`,
		"END_NODES",
		"",
		"START_PORTS",
		"NodeGuid,PortGuid,PortNum,LID,LinkSpeedActv,LinkWidthActv,PortPhyState,PortState",
		"0xhost1,0xhost1,1,1,2048,2,5,4",
		"0xswitch1,0xswitch1,1,0,2048,2,5,4",
		"END_PORTS",
	}, "\n")

	agg := NewAggregator()
	require.NoError(t, agg.AnalyzeFile("fabric.db_csv", strings.NewReader(input)))

	log := agg.Result()
	require.Len(t, log.Nodes, 2)

	byGUID := map[string]domain.Node{}
	for _, n := range log.Nodes {
		byGUID[n.GUID] = n
	}

	host, ok := byGUID["0xhost1"]
	require.True(t, ok, "host node missing")
	assert.Equal(t, domain.NodeTypeHost, host.Type)
	assert.Equal(t, "HOST_1", host.Desc)
	require.Len(t, host.Ports, 1)
	assert.Equal(t, 4, host.Ports[0].State)

	sw, ok := byGUID["0xswitch1"]
	require.True(t, ok, "switch node missing")
	assert.Equal(t, domain.NodeTypeSwitch, sw.Type)
}

func TestAggregator_SwitchAndSystemInfo(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"START_NODES",
		"NodeDesc,NodeType,NodeGUID",
		`"SWITCH_1",2,0xswitch1`,
		"END_NODES",
		"",
		"START_SWITCHES",
		"NodeGUID,LinearFDBCap,PortStateChange",
		"0xswitch1,49152,0",
		"END_SWITCHES",
		"",
		"START_SYSTEM_GENERAL_INFORMATION",
		"NodeGuid,SerialNumber,ProductName",
		`0xswitch1,SOS123,"Gorilla"`,
		"END_SYSTEM_GENERAL_INFORMATION",
	}, "\n")

	agg := NewAggregator()
	require.NoError(t, agg.AnalyzeFile("fabric.db_csv", strings.NewReader(input)))

	nodes := agg.Result().Nodes
	require.Len(t, nodes, 1)

	n := nodes[0]
	require.NotNil(t, n.Info)
	assert.Equal(t, "49152", n.Info.SwitchInfo["LinearFDBCap"])
	assert.Equal(t, "SOS123", n.Info.SystemInfo["SerialNumber"])
	assert.Equal(t, "Gorilla", n.Info.SystemInfo["ProductName"])
}

func TestAggregator_SharpInfo(t *testing.T) {
	t.Parallel()

	csvInput := strings.Join([]string{
		"START_NODES",
		"NodeDesc,NodeType,NodeGUID",
		`"SWITCH_4",2,0xswitch4`,
		"END_NODES",
	}, "\n")

	sharpInput := strings.Join([]string{
		"---",
		"SW_GUID=switch4",
		"---",
		"endianness = 0",
		"enable_endianness_per_job = 1",
		"reproducibility_disable = 0",
	}, "\n")

	agg := NewAggregator()
	require.NoError(t, agg.AnalyzeFile("fabric.db_csv", strings.NewReader(csvInput)))
	require.NoError(t, agg.AnalyzeFile("fabric.sharp_an_info", strings.NewReader(sharpInput)))

	nodes := agg.Result().Nodes
	require.Len(t, nodes, 1)
	require.NotNil(t, nodes[0].Info)
	require.NotNil(t, nodes[0].Info.SharpInfo)

	assert.Equal(t, "0", nodes[0].Info.SharpInfo["endianness"])
	assert.Equal(t, "1", nodes[0].Info.SharpInfo["enable_endianness_per_job"])
}

func TestAggregator_UnknownNodeTypeMapsToUnknown(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"START_NODES",
		"NodeDesc,NodeType,NodeGUID",
		`"WEIRD",99,0xweird`,
		"END_NODES",
	}, "\n")

	agg := NewAggregator()
	require.NoError(t, agg.AnalyzeFile("fabric.db_csv", strings.NewReader(input)))

	nodes := agg.Result().Nodes
	require.Len(t, nodes, 1)
	assert.Equal(t, domain.NodeTypeUnknown, nodes[0].Type)
}
