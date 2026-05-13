package parser

import (
	"archive/zip"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

var (
	ErrInputNotFound = errors.New("input file not found")
	ErrInputNotZip   = errors.New("input is not a valid zip archive")
)

func (p *Parser) Preflight(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: %s", ErrInputNotFound, path)
		}

		return fmt.Errorf("stat input: %w", err)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: not a regular file", ErrInputNotZip)
	}

	zr, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrInputNotZip, err.Error())
	}

	if cErr := zr.Close(); cErr != nil {
		return fmt.Errorf("close zip: %w", cErr)
	}

	return nil
}
