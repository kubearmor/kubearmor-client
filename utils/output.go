package utils

import (
	"os"
)

// CreateOutDir Function to create output directory.
func CreateOutDir(dir string) error {
	if dir == "" {
		return nil
	}

	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	return nil
}
