// SPDX-License-Identifier: Apache-2.0
// Copyright 2023 Authors of KubeArmor

// Package registry contains scanner for image info
package registry

import (
	"archive/tar"
	"bufio"
	"context"
	_ "embed" // need for embedding
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	image "github.com/kubearmor/kubearmor-client/recommend/image"
	"github.com/moby/term"

	dockerTypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	kg "github.com/kubearmor/KubeArmor/KubeArmor/log"
	"github.com/kubearmor/kubearmor-client/hacks"
	log "github.com/sirupsen/logrus"
)

const karmorTempDirPattern = "karmor"

// Scanner represents a utility for scanning Docker registries
type Scanner struct {
	authConfiguration authConfigurations
	cli               *client.Client // docker client
	cache             map[string]image.Info
}

// authConfigurations contains the configuration information's
type authConfigurations struct {
	configPath string // stores path of docker config.json
	authCreds  []string
}

func getAuthStr(u, p string) string {
	if u == "" || p == "" {
		return ""
	}

	encodedJSON, err := json.Marshal(registry.AuthConfig{
		Username: u,
		Password: p,
	})
	if err != nil {
		log.WithError(err).Fatal("failed to marshal credentials")
	}

	return base64.URLEncoding.EncodeToString(encodedJSON)
}

func (r *Scanner) loadDockerAuthConfigs() {
	r.authConfiguration.authCreds = append(r.authConfiguration.authCreds, fmt.Sprintf("%s:%s", os.Getenv("DOCKER_USERNAME"), os.Getenv("DOCKER_PASSWORD")))
	if r.authConfiguration.configPath != "" {
		data, err := os.ReadFile(filepath.Clean(r.authConfiguration.configPath))
		if err != nil {
			return
		}

		confsWrapper := struct {
			Auths map[string]registry.AuthConfig `json:"auths"`
		}{}
		err = json.Unmarshal(data, &confsWrapper)
		if err != nil {
			return
		}

		for _, conf := range confsWrapper.Auths {
			if len(conf.Auth) == 0 {
				continue
			}
			data, _ := base64.StdEncoding.DecodeString(conf.Auth)
			userPass := strings.SplitN(string(data), ":", 2)
			r.authConfiguration.authCreds = append(r.authConfiguration.authCreds, getAuthStr(userPass[0], userPass[1]))
		}
	}
}

// New creates and initializes a new instance of the Scanner
func New(dockerConfigPath string) *Scanner {
	var err error
	scanner := Scanner{
		authConfiguration: authConfigurations{
			configPath: dockerConfigPath,
		},
		cache: make(map[string]image.Info),
	}

	if err != nil {
		log.WithError(err).Error("could not create temp dir")
	}

	scanner.cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.WithError(err).Fatal("could not create new docker client")
	}
	scanner.loadDockerAuthConfigs()

	return &scanner
}

// Analyze performs analysis and caching of image information using the Scanner
func (r *Scanner) Analyze(img *image.Info) {
	if val, ok := r.cache[img.Name]; ok {
		log.WithFields(log.Fields{
			"image": img.Name,
		}).Infof("Image already scanned in this session, using cached informations for image")
		img.Arch = val.Arch
		img.DirList = val.DirList
		img.FileList = val.FileList
		img.Distro = val.Distro
		img.Labels = val.Labels
		img.OS = val.OS
		img.RepoTags = val.RepoTags
		return
	}
	tmpDir, err := os.MkdirTemp("", karmorTempDirPattern)
	if err != nil {
		log.WithError(err).Error("could not create temp dir")
	}
	defer func() {
		err = os.RemoveAll(tmpDir)
		if err != nil {
			log.WithError(err).Error("failed to remove cache files")
		}
	}()
	img.TempDir = tmpDir
	err = r.pullImage(img.Name)
	if err != nil {
		log.Warn("Failed to pull image. Dumping generic policies.")
		img.OS = "linux"
		img.RepoTags = append(img.RepoTags, img.Name)
	} else {
		tarname := saveImageToTar(img.Name, r.cli, tmpDir)
		img.FileList, img.DirList = extractTar(tarname, tmpDir)
		img.GetImageInfo()
	}

	r.cache[img.Name] = *img
}

// The randomizer used in this function is not used for any cryptographic
// operation and hence safe to use.
func randString(n int) string {
	letterRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))] // #nosec
	}
	return string(b)
}

func (r *Scanner) pullImage(imageName string) (err error) {
	log.WithFields(log.Fields{
		"image": imageName,
	}).Info("pulling image")

	var out io.ReadCloser

	for _, cred := range r.authConfiguration.authCreds {
		out, err = r.cli.ImagePull(context.Background(), imageName,
			dockerTypes.PullOptions{
				RegistryAuth: cred,
			})
		if err == nil {
			break
		}
	}
	if err != nil {
		return err
	}
	defer func() {
		if err := out.Close(); err != nil {
			kg.Warnf("Error closing io stream %s\n", err)
		}
	}()
	termFd, isTerm := term.GetFdInfo(os.Stderr)
	err = jsonmessage.DisplayJSONMessagesStream(out, os.Stderr, termFd, isTerm, nil)
	if err != nil {
		log.WithError(err).Error("could not display json")
	}

	return
}

// Sanitize archive file pathing from "G305: Zip Slip vulnerability"
func sanitizeArchivePath(d, t string) (v string, err error) {
	v = filepath.Join(d, t)
	if strings.HasPrefix(v, filepath.Clean(d)) {
		return v, nil
	}

	return "", fmt.Errorf("%s: %s", "content filepath is tainted", t)
}

func extractTar(tarname string, tempDir string) ([]string, []string) {
	var fl []string
	var dl []string

	f, err := os.Open(filepath.Clean(tarname))
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"tar": tarname,
		}).Fatal("os create failed")
	}
	defer hacks.CloseCheckErr(f, tarname)
	if isTarFile(f) {
		_, err := f.Seek(0, 0)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"tar": tarname,
			}).Fatal("Failed to seek to the beginning of the file")
		}
		tr := tar.NewReader(bufio.NewReader(f))
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break // End of archive
			}
			if err != nil {
				log.WithError(err).Error("tar next failed")
				return nil, nil
			}

			tgt, err := sanitizeArchivePath(tempDir, hdr.Name)
			if err != nil {
				log.WithError(err).WithFields(log.Fields{
					"file": hdr.Name,
				}).Error("ignoring file since it could not be sanitized")
				continue
			}

			switch hdr.Typeflag {
			case tar.TypeDir:
				if _, err := os.Stat(tgt); err != nil {
					if err := os.MkdirAll(tgt, 0o750); err != nil {
						log.WithError(err).WithFields(log.Fields{
							"target": tgt,
						}).Fatal("tar mkdirall")
					}
				}
				dl = append(dl, tgt)
			case tar.TypeReg:
				f, err := os.OpenFile(filepath.Clean(tgt), os.O_CREATE|os.O_RDWR, os.FileMode(hdr.Mode)) //#nosec G115 // hdr.mode bits are trusted here
				if err != nil {
					log.WithError(err).WithFields(log.Fields{
						"target": tgt,
					}).Error("tar open file")
				} else {
					// copy over contents
					if _, err := io.CopyN(f, tr, 2e+9 /*2GB*/); err != io.EOF {
						log.WithError(err).WithFields(log.Fields{
							"target": tgt,
						}).Fatal("tar io.Copy()")
					}
				}
				hacks.CloseCheckErr(f, tgt)
				if strings.HasSuffix(tgt, "layer.tar") {
					ifl, idl := extractTar(tgt, tempDir)
					fl = append(fl, ifl...)
					dl = append(dl, idl...)
				} else if strings.HasPrefix(hdr.Name, "blobs/") {
					ifl, idl := extractTar(tgt, tempDir)
					fl = append(fl, ifl...)
					dl = append(dl, idl...)

				} else {
					fl = append(fl, tgt)
				}
			}
		}
	} else {
		log.WithFields(log.Fields{
			"file": tarname,
		}).Error("Not a valid tar file")
	}
	return fl, dl
}

func isTarFile(f *os.File) bool {
	tr := tar.NewReader(bufio.NewReader(f))
	_, err := tr.Next()
	return err == nil
}

func saveImageToTar(imageName string, cli *client.Client, tempDir string) string {
	imgdata, err := cli.ImageSave(context.Background(), []string{imageName})
	if err != nil {
		log.WithError(err).Fatal("could not save image")
	}
	defer func() {
		if err := imgdata.Close(); err != nil {
			kg.Warnf("Error closing io stream %s\n", err)
		}
	}()

	tarname := filepath.Join(tempDir, randString(8)+".tar")

	f, err := os.Create(filepath.Clean(tarname))
	if err != nil {
		log.WithError(err).Fatal("os create failed")
	}

	if _, err := io.CopyN(bufio.NewWriter(f), imgdata, 5e+9 /*5GB*/); err != io.EOF {
		log.WithError(err).WithFields(log.Fields{
			"tar": tarname,
		}).Fatal("io.CopyN() failed")
	}
	hacks.CloseCheckErr(f, tarname)
	log.WithFields(log.Fields{
		"tar": tarname,
	}).Info("dumped image to tar")
	return tarname
}
