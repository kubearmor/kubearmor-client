// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package oci

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/distribution/distribution/v3/reference"
	"github.com/rs/zerolog/log"
)

// getRegRepoTag return registry, repository path and tag out of image
// string. It returns default registry and tag if not found.
func getRegRepoTag(image string) (string, string, string, error) {
	var reg, repo, tag string
	matches := reference.ReferenceRegexp.FindStringSubmatch(image)
	if matches == nil {
		return "", "", "", ErrInvalidImage
	}
	tag = matches[2]
	repo = matches[1]
	// TODO(akshay): Improve: Fetch reg from repo
	repoSplit := strings.SplitN(repo, "/", 2)
	if strings.ContainsAny(repoSplit[0], ".:") {
		reg = repoSplit[0]
	}
	if reg == "" {
		reg = DefaultRegistry
		repo = strings.Join([]string{reg, repo}, "/")
	}
	if tag == "" {
		tag = DefaultTag
	}
	return reg, repo, tag, nil
}

// MakeTemporaryDir creates a temporary directory and returns the path
// of the new directory.
func MakeTemporaryDir(pfx string) (string, error, func()) {
	if pfx == "" {
		pfx = DefaultTempDirPrefix
	}

	dir, err := os.MkdirTemp(os.TempDir(), pfx)
	if err != nil {
		return "", err, nil
	}
	return dir, nil, func() {
		err := os.RemoveAll(dir)
		if err != nil {
			log.Warn().Msgf("unable to clean temp directory: %s", err)
		}
	}
}

// CopyFiles copies srcFiles to dst directory. It returns path of
// copied files.
func CopyFiles(srcFiles []string, dst string) ([]string, error) {
	copied := make([]string, 0, len(srcFiles))
	for _, file := range srcFiles {
		file, err := filepath.Abs(file)
		if err != nil {
			return nil, err
		}
		stat, err := os.Stat(file)
		if err != nil {
			return nil, err
		}
		if !stat.Mode().IsRegular() {
			return nil, fmt.Errorf("%s is not a regular file", file)
		}

		sp, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		defer sp.Close()

		new := filepath.Join(dst, filepath.Base(file))
		dp, err := os.Create(new)
		if err != nil {
			return nil, err
		}
		defer dp.Close()

		_, err = io.Copy(dp, sp)
		if err != nil {
			return nil, err
		}
		copied = append(copied, new)
	}
	return copied, nil
}
