// SPDX-License-Identifier: Apache-2.0
// Copyright 2023 Authors of KubeArmor

// Package registry contains scanner for image info
package registry

import (
	"context"
	_ "embed" // need for embedding

	"encoding/json"
	"fmt"

	"os"
	"path/filepath"

	// image "github.com/kubearmor/kubearmor-client/recommend/image"
	// kg "github.com/kubearmor/KubeArmor/KubeArmor/log"
	// log "github.com/sirupsen/logrus"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	credentials "oras.land/oras-go/v2/registry/remote/credentials"
	"oras.land/oras-go/v2/registry/remote/retry"
)

const karmorTempDirPattern = "karmor"

// Scanner represents a utility for scanning Docker registries

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

// New creates and initializes a new instance of the Scanner
func (o *OCIRegistry) Pull(output string) (files []string, directories []string, err error) {
	ctx := context.Background()
	tempDir, err, rmdir := MakeTemporaryDir("")
	fmt.Printf("tmpdir: %v", tempDir)
	if err != nil {
		return nil, nil, err
	}
	defer rmdir()

	// 0. Create a file store
	store, err := file.New(tempDir)
	if err != nil {
		return nil, nil, err
	}
	defer store.Close()

	reg, repoPath, tag, err := getRegRepoTag(o.Image)
	repoPath = "docker.io/library/ubuntu"
	fmt.Printf("This is the repoPath %v \n",repoPath)
	fmt.Println(reg)
	if err != nil {
		return nil, nil, err
	}

	// 1. Connect to a remote repository
	repo, err := remote.NewRepository(repoPath)
	if err != nil {
		return nil, nil, err
	}
	if v := os.Getenv(EnvOCIInsecure); v == "true" {
		repo.PlainHTTP = true
	}
	if o.Credentials.Username != "" {
		fmt.Printf("Using static credentials: %s\n", o.Credentials.Username)
		repo.Client = &auth.Client{
			Client: retry.DefaultClient,
			Cache:  auth.DefaultCache,
			Credential: auth.StaticCredential(reg, auth.Credential{
				Username: o.Credentials.Username,
				Password: o.Credentials.Password,
			}),
		}
	} else {
		// Get credentials from the Docker credential store
		fmt.Println("hello")
		storeOpts := credentials.StoreOptions{} // Adjust as per the deprecation notice
		credStore, err := credentials.NewStoreFromDocker(storeOpts)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create credential store: %w", err)
		}

		// Retrieve the credentials and print them to debug
		creds, err := credStore.Get(ctx, reg)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get credentials for %s: %w", reg, err)
		}

		fmt.Printf("Retrieved credentials from Docker store: %v\n", creds)

		repo.Client = &auth.Client{
			Client:     retry.DefaultClient,
			Cache:      auth.DefaultCache,
			Credential: credentials.Credential(credStore),
		}
	}

	// 2. Copy from the remote repository to the file store
	fmt.Println("reached second")
	fmt.Printf("the tag is %v and the another target is %v \n", tag ,tag  )
	fmt.Printf("the repo is %v \n", *repo)
	fmt.Printf("the store is %v \n", store)
	fmt.Printf("the options is %v \n", oras.DefaultCopyOptions)
	copyOptions := oras.DefaultCopyOptions
	

	fmt.Printf("max data bytes :%v", copyOptions.MaxMetadataBytes )
	manifestDescriptor, err := oras.Copy(ctx, repo, tag, store, tag, copyOptions)
	if err != nil {
		fmt.Println("reached second error")
		return nil, nil, err
	}
	fmt.Println("reached third")
	// 3. Fetch from OCI layout store
	fetched, err := content.FetchAll(ctx, store, manifestDescriptor)
	if err != nil {
		return nil, nil, err
	}

	manifest := &v1.Manifest{}
	err = json.Unmarshal(fetched, manifest)
	if err != nil {
		return nil, nil, err
	}
	fmt.Println("reached four")
	// 4. Iterate over layers and extract files
	var layerFiles []string
	for _, layer := range manifest.Layers {
		if layer.MediaType != mediaType {
			continue
		}
		if title, ok := layer.Annotations[v1.AnnotationTitle]; ok {
			layerFiles = append(layerFiles, filepath.Join(tempDir, title))
		}
	}

	if output == "" {
		output, err = os.Getwd()
		if err != nil {
			return nil, nil, err
		}
	}
	outputStat, err := os.Stat(output)
	if err != nil {
		return nil, nil, err
	}
	if !outputStat.IsDir() {
		return nil, nil, fmt.Errorf("%s is not a directory", output)
	}

	// 5. Copy files to the output directory
	dsts, err := CopyFiles(layerFiles, output)
	if err != nil {
		return nil, nil, err
	}
	o.Files = dsts

	// 6. Walk through the directory to categorize files and directories
	err = filepath.Walk(output, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			directories = append(directories, path)
		} else {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	return files, directories, nil
}

// // Analyze performs analysis and caching of image information using the Scanner
// func (r *Scanner) Analyze(img *image.Info) {
// 	if val, ok := r.cache[img.Name]; ok {
// 		log.WithFields(log.Fields{
// 			"image": img.Name,
// 		}).Infof("Image already scanned in this session, using cached informations for image")
// 		img.Arch = val.Arch
// 		img.DirList = val.DirList
// 		img.FileList = val.FileList
// 		img.Distro = val.Distro
// 		img.Labels = val.Labels
// 		img.OS = val.OS
// 		img.RepoTags = val.RepoTags
// 		return
// 	}
// 	tmpDir, err := os.MkdirTemp("", karmorTempDirPattern)
