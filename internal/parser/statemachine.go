package parser

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

const (
	markerStart = "START_"
	markerEnd   = "END_"

	scannerBufferInitial = 64 * 1024
	scannerBufferMax     = 1024 * 1024
)

type sectionState int

const (
	stateOutside sectionState = iota
	stateHeader
	stateBody
)

type sectionEvent struct {
	Name    string
	Columns []string
	Row     []string
}

type sectionMachine struct {
	state   sectionState
	section string
	columns []string
	emit    func(sectionEvent)
}

func newSectionMachine(emit func(sectionEvent)) *sectionMachine {
	return &sectionMachine{emit: emit}
}

func (m *sectionMachine) Run(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, scannerBufferInitial), scannerBufferMax)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if err := m.step(line); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if m.state != stateOutside {
		return fmt.Errorf("section %s: unclosed at end of stream", m.section)
	}

	return nil
}

func (m *sectionMachine) step(line string) error {
	switch m.state {
	case stateOutside:
		if name, ok := strings.CutPrefix(line, markerStart); ok {
			m.section = name
			m.state = stateHeader
		}
	case stateHeader:
		cols, err := parseCSVLine(line)
		if err != nil {
			return fmt.Errorf("section %s: header: %w", m.section, err)
		}

		m.columns = cols
		m.state = stateBody
	case stateBody:
		if line == markerEnd+m.section {
			m.state = stateOutside
			m.section = ""
			m.columns = nil

			return nil
		}

		row, err := parseCSVLine(line)
		if err != nil {
			return fmt.Errorf("section %s: row: %w", m.section, err)
		}

		if len(row) != len(m.columns) {
			return fmt.Errorf("section %s: row column count mismatch: expected %d, got %d",
				m.section, len(m.columns), len(row))
		}

		m.emit(sectionEvent{Name: m.section, Columns: m.columns, Row: row})
	}

	return nil
}

func parseCSVLine(line string) ([]string, error) {
	r := csv.NewReader(strings.NewReader(line))

	return r.Read()
}
