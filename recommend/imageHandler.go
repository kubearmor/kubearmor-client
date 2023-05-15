// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	"archive/tar"
	"bufio"
	"context"
	_ "embed" // need for embedding
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/clarketm/json"
	"sigs.k8s.io/yaml"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/fatih/color"
	kg "github.com/kubearmor/KubeArmor/KubeArmor/log"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/moby/term"
	log "github.com/sirupsen/logrus"
)

var cli *client.Client      // docker client
var tempDir string          // temporary directory used by karmor to save image etc
var dockerConfigPath string // stores path of docker config.json

// ImageInfo contains image information
type ImageInfo struct {
	Name       string
	RepoTags   []string
	Arch       string
	Distro     string
	OS         string
	FileList   []string
	DirList    []string
	Namespace  string
	Deployment string
	Labels     LabelMap
}

// AuthConfigurations contains the configuration information's
type AuthConfigurations struct {
	Configs map[string]types.AuthConfig `json:"configs"`
}

// getConf reads the docker config.json file and returns a map
func getConf() map[string]types.AuthConfig {
	var confs map[string]types.AuthConfig
	if dockerConfigPath != "" {
		data, err := os.ReadFile(filepath.Clean(dockerConfigPath))
		if err != nil {
			return confs
		}
		confs, err = parseDockerConfig(data)
		if err != nil {
			return confs
		}
	}
	return confs
}

func getAuthStr(u, p string) string {
	if u == "" || p == "" {
		return ""
	}

	encodedJSON, err := json.Marshal(types.AuthConfig{
		Username: u,
		Password: p,
	})
	if err != nil {
		log.WithError(err).Fatal("failed to marshal credentials")
	}

	return base64.URLEncoding.EncodeToString(encodedJSON)
}

func init() {
	var err error

	rand.Seed(time.Now().UnixNano()) // random seed init for random string generator

	cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.WithError(err).Fatal("could not create new docker client")
	}
}

// parseDockerConfig parses the docker config.json to generate a map
func parseDockerConfig(byteData []byte) (map[string]types.AuthConfig, error) {

	confsWrapper := struct {
		Auths map[string]types.AuthConfig `json:"auths"`
	}{}
	if err := json.Unmarshal(byteData, &confsWrapper); err == nil {
		if len(confsWrapper.Auths) > 0 {
			return confsWrapper.Auths, nil
		}
	}

	var confs map[string]types.AuthConfig
	if err := json.Unmarshal(byteData, &confs); err != nil {
		return nil, err
	}
	return confs, nil
}

func pullImage(imageName string) error {
	var out io.ReadCloser
	var err error
	var confData []string
	confData = append(confData, fmt.Sprintf("%s:%s", os.Getenv("DOCKER_USERNAME"), os.Getenv("DOCKER_PASSWORD")))
	if dockerConfigPath != "" {
		confs := getConf()
		if len(confs) > 0 {
			for _, conf := range confs {
				data, _ := base64.StdEncoding.DecodeString(conf.Auth)
				confData = append(confData, string(data))
			}
		}
	}
	for _, data := range confData {
		userpass := strings.SplitN(string(data), ":", 2)
		out, err = cli.ImagePull(context.Background(), imageName, types.ImagePullOptions{RegistryAuth: getAuthStr(userpass[0], userpass[1])})
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

	return nil
}

// The randomizer used in this function is not used for any cryptographic
// operation and hence safe to use.
func randString(n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))] // #nosec
	}
	return string(b)
}

func closeCheckErr(f *os.File, fname string) {
	err := f.Close()
	if err != nil {
		log.WithFields(log.Fields{
			"file": fname,
		}).Error("close file failed")
	}
}

// Sanitize archive file pathing from "G305: Zip Slip vulnerability"
func sanitizeArchivePath(d, t string) (v string, err error) {
	v = filepath.Join(d, t)
	if strings.HasPrefix(v, filepath.Clean(d)) {
		return v, nil
	}

	return "", fmt.Errorf("%s: %s", "content filepath is tainted", t)
}

func extractTar(tarname string) ([]string, []string) {
	var fl []string
	var dl []string

	f, err := os.Open(filepath.Clean(tarname))
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"tar": tarname,
		}).Fatal("os create failed")
	}
	defer closeCheckErr(f, tarname)

	tr := tar.NewReader(bufio.NewReader(f))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			log.WithError(err).Fatal("tar next failed")
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
				if err := os.MkdirAll(tgt, 0750); err != nil {
					log.WithError(err).WithFields(log.Fields{
						"target": tgt,
					}).Fatal("tar mkdirall")
				}
			}
			dl = append(dl, tgt)
		case tar.TypeReg:
			f, err := os.OpenFile(filepath.Clean(tgt), os.O_CREATE|os.O_RDWR, os.FileMode(hdr.Mode))
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
			closeCheckErr(f, tgt)
			if strings.HasSuffix(tgt, "layer.tar") { // deflate container image layer
				ifl, idl := extractTar(tgt)
				fl = append(fl, ifl...)
				dl = append(dl, idl...)
			} else {
				fl = append(fl, tgt)
			}
		}
	}
	return fl, dl
}

func saveImageToTar(imageName string) string {
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
	closeCheckErr(f, tarname)
	log.WithFields(log.Fields{
		"tar": tarname,
	}).Info("dumped image to tar")
	return tarname
}

func checkForSpec(spec string, fl []string) []string {
	var matches []string
	if !strings.HasSuffix(spec, "*") {
		spec = fmt.Sprintf("%s$", spec)
	}

	re := regexp.MustCompile(spec)
	for _, name := range fl {
		if re.Match([]byte(name)) {
			matches = append(matches, name)
		}
	}
	return matches
}

func getFileBytes(fname string) ([]byte, error) {
	f, err := os.Open(filepath.Clean(fname))
	if err != nil {
		log.WithFields(log.Fields{
			"file": fname,
		}).Fatal("open file failed")
	}
	defer closeCheckErr(f, fname)
	return io.ReadAll(f)
}

func (img *ImageInfo) readManifest(manifest string) {
	// read manifest file
	barr, err := getFileBytes(manifest)
	if err != nil {
		log.WithError(err).Fatal("manifest read failed")
	}
	var manres []map[string]interface{}
	err = json.Unmarshal(barr, &manres)
	if err != nil {
		log.WithError(err).Fatal("manifest json unmarshal failed")
	}
	if len(manres) < 1 {
		log.WithFields(log.Fields{
			"len":     len(manres),
			"results": manres,
		}).Fatal("expecting atleast one config in manifest!")
	}

	var man map[string]interface{}
	for _, man = range manres {
		if man["RepoTags"] != nil {
			break
		}
	}

	// read config file
	config := filepath.Join(tempDir, man["Config"].(string))
	barr, err = getFileBytes(config)
	if err != nil {
		log.WithFields(log.Fields{
			"config": config,
		}).Fatal("config read failed")
	}
	var cfgres map[string]interface{}
	err = json.Unmarshal(barr, &cfgres)
	if err != nil {
		log.WithError(err).Fatal("config json unmarshal failed")
	}
	img.Arch = cfgres["architecture"].(string)
	img.OS = cfgres["os"].(string)

	if man["RepoTags"] == nil {
		// If the image name contains sha256 digest,
		// then manifest["RepoTags"] will be `nil`.
		img.RepoTags = append(img.RepoTags, shortenImageNameWithSha256(img.Name))
	} else {
		for _, tag := range man["RepoTags"].([]interface{}) {
			img.RepoTags = append(img.RepoTags, tag.(string))
		}
	}
}

func (img *ImageInfo) getPolicyDir() string {
	var policyDir string

	if img.Deployment == "" {
		// policy recommendation for container images
		if img.Namespace == "" {
			policyDir = mkPathFromTag(img.RepoTags[0])
		} else {
			policyDir = fmt.Sprintf("%s-%s", img.Namespace, mkPathFromTag(img.RepoTags[0]))
		}
	} else {
		// policy recommendation based on k8s manifest
		policyDir = fmt.Sprintf("%s-%s", img.Namespace, img.Deployment)
	}
	return filepath.Join(options.OutDir, policyDir)
}

func (img *ImageInfo) getPolicyFile(spec string) string {
	var policyFile string

	if img.Deployment != "" {
		// policy recommendation based on k8s manifest
		policyFile = fmt.Sprintf("%s-%s.yaml", mkPathFromTag(img.RepoTags[0]), spec)
	} else {
		policyFile = fmt.Sprintf("%s.yaml", spec)
	}

	return filepath.Join(img.getPolicyDir(), policyFile)
}

func (img *ImageInfo) getPolicyName(spec string) string {
	var policyName string

	if img.Deployment == "" {
		// policy recommendation for container images
		policyName = fmt.Sprintf("%s-%s", mkPathFromTag(img.RepoTags[0]), spec)
	} else {
		// policy recommendation based on k8s manifest
		policyName = fmt.Sprintf("%s-%s-%s", img.Deployment, mkPathFromTag(img.RepoTags[0]), spec)
	}
	return policyName
}

type distroRule struct {
	Name  string `json:"name" yaml:"name"`
	Match []struct {
		Path string `json:"path" yaml:"path"`
	} `json:"match" yaml:"match"`
}

//go:embed yaml/distro.yaml
var distroYAML []byte

var distroRules []distroRule

func init() {
	distroJSON, err := yaml.YAMLToJSON(distroYAML)
	if err != nil {
		color.Red("failed to convert distro rules yaml to json")
		log.WithError(err).Fatal("failed to convert distro rules yaml to json")
	}

	var jsonRaw map[string]json.RawMessage
	err = json.Unmarshal(distroJSON, &jsonRaw)
	if err != nil {
		color.Red("failed to unmarshal distro rules json")
		log.WithError(err).Fatal("failed to unmarshal distro rules json")
	}

	err = json.Unmarshal(jsonRaw["distroRules"], &distroRules)
	if err != nil {
		color.Red("failed to unmarshal distro rules")
		log.WithError(err).Fatal("failed to unmarshal distro rules")
	}
}

func (img *ImageInfo) getDistro() {
	for _, d := range distroRules {
		match := true
		for _, m := range d.Match {
			matches := checkForSpec(filepath.Clean(tempDir+m.Path), img.FileList)
			if len(matches) == 0 {
				match = false
				break
			}
		}
		if len(d.Match) > 0 && match {
			color.Green("Distribution %s", d.Name)
			img.Distro = d.Name
			return
		}
	}
}

func (img *ImageInfo) getImageInfo() {
	matches := checkForSpec(filepath.Join(tempDir, "manifest.json"), img.FileList)
	if len(matches) != 1 {
		log.WithFields(log.Fields{
			"len":     len(matches),
			"matches": matches,
		}).Fatal("expecting one manifest.json!")
	}
	img.readManifest(matches[0])

	img.getDistro()
}

func getImageDetails(img ImageInfo) error {

	// step 1: save the image to a tar file
	tarname := saveImageToTar(img.Name)

	// step 2: retrieve information from tar
	img.FileList, img.DirList = extractTar(tarname)

	// step 3: getImageInfo
	img.getImageInfo()

	if len(img.RepoTags) == 0 {
		img.RepoTags = append(img.RepoTags, img.Name)
	}
	// step 4: get policy from image info
	img.getPolicyFromImageInfo()

	return nil
}

func imageHandler(namespace, deployment string, labels LabelMap, imageName string, c *k8s.Client) error {
	dockerConfigPath = options.Config
	img := ImageInfo{
		Name:       imageName,
		Namespace:  namespace,
		Deployment: deployment,
		Labels:     labels,
	}

	if len(options.Policy) == 0 {
		return fmt.Errorf("no policy specified, specify at least one policy to be recommended")
	}

	policiesToBeRecommendedSet := make(map[string]bool)
	for _, policy := range options.Policy {
		policiesToBeRecommendedSet[policy] = true
	}

	_, containsKubeArmorPolicy := policiesToBeRecommendedSet[KubeArmorPolicy]
	if containsKubeArmorPolicy {
		err := recommendKubeArmorPolicies(imageName, img)
		if err != nil {
			log.WithError(err).Error("failed to recommend kubearmor policies.")
			return err
		}
	}

	_, containsKyvernoPolicy := policiesToBeRecommendedSet[KyvernoPolicy]

	// Admission Controller Policies are not recommended based on an image
	if len(options.Images) == 0 && containsKyvernoPolicy {
		if len(img.RepoTags) == 0 {
			img.RepoTags = append(img.RepoTags, img.Name)
		}
		if !containsKubeArmorPolicy {
			if err := ReportStart(&img); err != nil {
				log.WithError(err).Error("report start failed")
				return err
			}
		}
		err := initClientConnection(c)
		if err != nil {
			log.WithError(err).Error("failed to initialize client connection.")
			return err
		}
		err = recommendAdmissionControllerPolicies(img)
		if err != nil {
			log.WithError(err).Error("failed to recommend admission controller policies.")
			return err
		}
	}

	if !containsKyvernoPolicy && !containsKubeArmorPolicy {
		return fmt.Errorf("policy type not supported: %v", options.Policy)
	}
	_ = ReportSectEnd(&img)

	return nil
}

func recommendKubeArmorPolicies(imageName string, img ImageInfo) error {
	log.WithFields(log.Fields{
		"image": imageName,
	}).Info("pulling image")
	err := pullImage(imageName)
	if err != nil {
		log.Warn("Failed to pull image. Dumping generic policies.")
		img.OS = "linux"
		img.RepoTags = append(img.RepoTags, img.Name)
		img.getPolicyFromImageInfo()
	} else {
		err = getImageDetails(img)
		if err != nil {
			return err
		}
	}
	return nil
}

// shortenImageNameWithSha256 truncates the sha256 digest in image name
func shortenImageNameWithSha256(name string) string {
	if strings.Contains(name, "@sha256:") {
		// shorten sha256 to first 8 chars
		return name[:len(name)-56]
	}
	return name
}
