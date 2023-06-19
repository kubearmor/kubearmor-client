// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

// Package oci work with OCI registry for KubeArmor policies
package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	credentials "github.com/oras-project/oras-credentials-go"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
	// "github.com/rs/zerolog/log"
)

type OCIRegistry struct {
	// Image is OCI image. Must follow the OCI image standard.
	Image string

	// Files are absolute artifact paths.
	Files []string

	// Credentials encapsulates registry authentication details.
	Credentials struct {
		// Username for registry.
		Username string

		// Password for registry.
		Password string
	}
}

func New(img string, files []string, user string, passwd string) *OCIRegistry {
	return &OCIRegistry{
		Image: img,
		Files: files,
		Credentials: struct {
			Username string
			Password string
		}{
			Username: user,
			Password: passwd,
		},
	}
}

// Pull fetches OCI image with artifact files from the OCI registry.
func (o *OCIRegistry) Pull(output string) error {
	ctx := context.Background()
	tempDir, err, rmdir := MakeTemporaryDir("")
	if err != nil {
		return err
	}
	defer rmdir()

	// 0. Create a file store
	store, err := file.New(tempDir)
	if err != nil {
		return err
	}
	defer store.Close()

	reg, repoPath, tag, err := getRegRepoTag(o.Image)
	if err != nil {
		return err
	}
	// log.Debug().Msgf("detected registry %s, repository %s, tag %s", reg, repoPath, tag)

	// 1. Connect to a remote repository
	repo, err := remote.NewRepository(repoPath)
	if err != nil {
		return err
	}
	if v := os.Getenv(EnvOCIInsecure); v == "true" {
		repo.PlainHTTP = true
	}
	if o.Credentials.Username != "" {
		repo.Client = &auth.Client{
			Client: retry.DefaultClient,
			Cache:  auth.DefaultCache,
			Credential: auth.StaticCredential(reg, auth.Credential{
				Username: o.Credentials.Username,
				Password: o.Credentials.Password,
			}),
		}
	} else {
		// Get credentials from the docker credential store
		storeOpts := credentials.StoreOptions{}
		credStore, err := credentials.NewStoreFromDocker(storeOpts)
		if err != nil {
			return err
		}
		repo.Client = &auth.Client{
			Client:     retry.DefaultClient,
			Cache:      auth.DefaultCache,
			Credential: credentials.Credential(credStore),
		}
	}

	// 2. Copy from the remote repository to the file store
	manifestDescriptor, err := oras.Copy(ctx, repo, tag, store, tag, oras.DefaultCopyOptions)
	if err != nil {
		return err
	}
	// log.Debug().Msgf("manifest descriptor: %v", manifestDescriptor)

	// 3. Fetch from OCI layout store
	fetched, err := content.FetchAll(ctx, store, manifestDescriptor)
	if err != nil {
		return err
	}
	// log.Debug().Msgf("manifest content:\n%s", fetched)

	manifest := &v1.Manifest{}
	err = json.Unmarshal(fetched, manifest)
	if err != nil {
		return err
	}
	var files []string
	for _, layer := range manifest.Layers {
		if layer.MediaType != mediaType {
			continue
		}
		if title, ok := layer.Annotations[v1.AnnotationTitle]; ok {
			files = append(files, filepath.Join(tempDir, title))
		}
	}
	if output == "" {
		output, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	outputStat, err := os.Stat(output)
	if err != nil {
		return err
	}
	if !outputStat.IsDir() {
		return fmt.Errorf("%s is not a directory", output)
	}

	dsts, err := CopyFiles(files, output)
	if err != nil {
		return err
	}
	o.Files = dsts
	return nil
}

// Push pushes OCI image along with artifact files to the OCI
// registry.
func (o *OCIRegistry) Push() error {
	ctx := context.Background()
	tempDir, err, rmdir := MakeTemporaryDir("")
	if err != nil {
		return err
	}
	defer rmdir()

	reg, repoPath, tag, err := getRegRepoTag(o.Image)
	if err != nil {
		return err
	}
	// log.Debug().Msgf("detected registry %s, repository %s, tag %s", reg, repoPath, tag)

	// 0. Create a file store
	fs, err := file.New(tempDir)
	if err != nil {
		return err
	}
	defer fs.Close()

	// 1. Add files to a files store
	fileNames, err := CopyFiles(o.Files, tempDir)
	if err != nil {
		return err
	}
	fileDescriptors := make([]v1.Descriptor, 0, len(fileNames))
	for _, name := range fileNames {
		fileDescriptor, err := fs.Add(ctx, filepath.Base(name), mediaType, name)
		if err != nil {
			return err
		}
		fileDescriptors = append(fileDescriptors, fileDescriptor)
		// log.Debug().Msgf("file descriptor for %s: %v", name, fileDescriptor)
	}

	// 2. Pack the files and tag the packed manifest
	manifestDescriptor, err := oras.Pack(ctx, fs, artifactType, fileDescriptors, oras.PackOptions{
		PackImageManifest: true,
	})
	if err != nil {
		return err
	}
	// log.Debug().Msgf("manifest descriptor: %v", manifestDescriptor)
	if err = fs.Tag(ctx, manifestDescriptor, tag); err != nil {
		return err
	}

	// 3. Connect to remote registry
	repo, err := remote.NewRepository(repoPath)
	if err != nil {
		return err
	}
	if v := os.Getenv(EnvOCIInsecure); v == "true" {
		repo.PlainHTTP = true
	}
	if o.Credentials.Username != "" {
		repo.Client = &auth.Client{
			Client: retry.DefaultClient,
			Cache:  auth.DefaultCache,
			Credential: auth.StaticCredential(reg, auth.Credential{
				Username: o.Credentials.Username,
				Password: o.Credentials.Password,
			}),
		}
	} else {
		// Get credentials from the docker credential store
		storeOpts := credentials.StoreOptions{}
		credStore, err := credentials.NewStoreFromDocker(storeOpts)
		if err != nil {
			return err
		}
		repo.Client = &auth.Client{
			Client:     retry.DefaultClient,
			Cache:      auth.DefaultCache,
			Credential: credentials.Credential(credStore),
		}
	}

	// 4. Copy from the file store to the remote repository
	_, err = oras.Copy(ctx, fs, tag, repo, tag, oras.DefaultCopyOptions)
	if err != nil {
		return err
	}
	return nil
}
