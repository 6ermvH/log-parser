package parser

import (
	"archive/zip"
	"fmt"

	"github.com/6ermvH/log-parser/internal/domain"
)

type Parser struct{}

func New() *Parser {
	return &Parser{}
}

func (p *Parser) Parse(path string) (log domain.Log, err error) {
	zr, openErr := zip.OpenReader(path)
	if openErr != nil {
		return domain.Log{}, fmt.Errorf("open zip: %w", openErr)
	}

	defer func() {
		if cerr := zr.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close zip: %w", cerr)
		}
	}()

	agg := NewAggregator()

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}

		if entryErr := analyzeEntry(agg, f); entryErr != nil {
			return domain.Log{}, entryErr
		}
	}

	return agg.Result(), nil
}

func analyzeEntry(agg *Aggregator, f *zip.File) (err error) {
	rc, openErr := f.Open()
	if openErr != nil {
		return fmt.Errorf("open %s: %w", f.Name, openErr)
	}

	defer func() {
		if cerr := rc.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close %s: %w", f.Name, cerr)
		}
	}()

	if aErr := agg.AnalyzeFile(f.Name, rc); aErr != nil {
		return fmt.Errorf("analyze %s: %w", f.Name, aErr)
	}

	return nil
}
