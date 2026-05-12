package parser

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/6ermvH/log-parser/internal/domain"
)

func TestParser_RealLogZip(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "testdata", "log.zip")

	log, err := New().Parse(path)
	require.NoError(t, err)
	require.Len(t, log.Nodes, 5)

	var hosts, switches int

	for _, n := range log.Nodes {
		switch n.Type {
		case domain.NodeTypeHost:
			hosts++
		case domain.NodeTypeSwitch:
			switches++
		}
	}

	assert.Equal(t, 1, hosts)
	assert.Equal(t, 4, switches)

	totalPorts := 0

	var switchOne *domain.Node

	for i := range log.Nodes {
		n := &log.Nodes[i]
		totalPorts += len(n.Ports)

		if n.GUID == "0xswitch1" {
			switchOne = n
		}
	}

	assert.Positive(t, totalPorts, "ports parsed")
	require.NotNil(t, switchOne, "switch1 not found")
	require.NotNil(t, switchOne.Info, "switch1 info missing")
	assert.NotNil(t, switchOne.Info.SwitchInfo)
	assert.NotNil(t, switchOne.Info.SystemInfo)
	assert.Equal(t, "Gorilla", switchOne.Info.SystemInfo["ProductName"])
}
