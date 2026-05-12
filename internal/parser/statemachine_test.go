package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSectionMachine_BasicFlow(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"START_NODES",
		"NodeDesc,NodeType,NodeGUID",
		`"HOST_1",1,0xhost1`,
		`"SWITCH_1",2,0xswitch1`,
		"END_NODES",
		"",
		"START_PORTS",
		"NodeGuid,PortNum",
		"0xhost1,1",
		"0xswitch1,0",
		"END_PORTS",
	}, "\n")

	var events []sectionEvent

	sm := newSectionMachine(func(e sectionEvent) {
		events = append(events, e)
	})

	require.NoError(t, sm.Run(strings.NewReader(input)))
	require.Len(t, events, 4)

	wantSections := []string{"NODES", "NODES", "PORTS", "PORTS"}
	for i, e := range events {
		assert.Equal(t, wantSections[i], e.Name, "event %d name", i)
	}

	assert.Equal(t, "HOST_1", events[0].Row[0])
}

func TestSectionMachine_IgnoresGarbageBetweenSections(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"random noise",
		"more noise",
		"START_X",
		"col1,col2",
		"a,b",
		"END_X",
		"trailing noise",
	}, "\n")

	var events []sectionEvent

	sm := newSectionMachine(func(e sectionEvent) {
		events = append(events, e)
	})

	require.NoError(t, sm.Run(strings.NewReader(input)))
	require.Len(t, events, 1)
}

func TestSectionMachine_MalformedCSVPropagates(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"START_X",
		"col1,col2",
		`"unclosed`,
		"END_X",
	}, "\n")

	sm := newSectionMachine(func(_ sectionEvent) {})

	require.Error(t, sm.Run(strings.NewReader(input)))
}
