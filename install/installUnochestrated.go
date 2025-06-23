// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor
package install

import (
	"archive/tar"
	"compress/gzip"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/Masterminds/sprig"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/fatih/color"
	"github.com/kubearmor/kubearmor-client/utils"
	"golang.org/x/mod/semver"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

//go:embed templates/configTemplate.yaml
var kubeArmorConfig string

//go:embed templates/kubearmor.service
var kubearmorServiceFile string

//go:embed templates/composeTemplate.yaml
var kubearmorcomposeTemplate string

const (
	PodmanRuntime            = "podman"
	DockerRuntime            = "docker"
	SystemdRuntime           = "systemd"
	RemoteHost               = "docker.io"
	ImageName                = "kubearmor/kubearmor-systemd"
	DefaultComposeFile       = "./docker-compose.yaml"
	DefaultSystemdFileConfig = "kubearmor-systemd.json"
	ConfigPath               = "/opt/kubearmor/kubearmor.yaml"
	DockerHubURL             = "https://hub.docker.com/v2/repositories/kubearmor/kubearmor-systemd/tags/"
)

var DefaultSystemdFile string

type Config struct {
	HostVisibility            string `json:"hostVisibility"`
	EnableKubeArmorHostPolicy bool   `json:"enableKubeArmorHostPolicy"`
}

type KubeArmorConfig struct {
	KubeArmorTag                string
	KubeArmorInitImage          string
	KubeArmorImage              string
	ImagePullPolicy             string
	Hostname                    string
	KubeArmorVisibility         string `json:"visibility"`
	KubeArmorHostVisibility     string `json:"hostVisibility"`
	EnableKubeArmorHostPolicy   bool   `json:"enableKubeArmorHostPolicy"`
	EnableKubeArmorPolicy       bool   `json:"enableKubeArmorPolicy"`
	KubeArmorFilePosture        string `json:"defaultFilePosture"`
	KubeArmorNetworkPosture     string `json:"defaultNetworkPosture"`
	KubeArmorCapPosture         string `json:"defaultCapabilitiesPosture"`
	KubeArmorHostFilePosture    string `json:"hostDefaultFilePosture"`
	KubeArmorHostNetworkPosture string `json:"hostDefaultNetworkPosture"`
	KubeArmorHostCapPosture     string `json:"hostDefaultCapabilitiesPosture"`
	EnableKubeArmorVm           bool   `json:"enableKubeArmorVm"`
	KubeArmorAlertThrottling    bool   `json:"alertThrottling"`
	KubeArmorMaxAlertsPerSec    int    `json:"maxAlertPerSec"`
	KubeArmorThrottleSec        int    `json:"throttleSec"`
	ComposeCmd                  string
	ComposeVersion              string
	AgentName                   string
	PackageName                 string
	ServiceName                 string
	AgentDir                    string
	ConfigFilePath              string
	ServiceTemplateString       string
	KubeArmorPort               string
	ConfigTemplateString        string
	Mode                        utils.VMMode
	ORASClient                  *auth.Client
	PlainHTTP                   bool
	UserConfigPath              string
	SecureContainers            bool
}

type TagResponse struct {
	Results []struct {
		Name string `json:"name"`
	} `json:"results"`
	Next string `json:"next"`
}

func createDefaultConfigPath() (string, error) {
	configPath, err := utils.GetDefaultConfigPath()
	if err != nil {
		return "", err
	}

	_, err = os.Stat(configPath)
	// return all errors expect if given path does not exist
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	err = os.MkdirAll(configPath, os.ModeDir|os.ModePerm)
	if err != nil {
		return "", err
	}

	return configPath, nil
}

func (config *KubeArmorConfig) DeployKAdocker() error {
	_, err := config.ValidateEnv()
	if err != nil {
		return err
	}
	fmt.Printf("ℹ️\tInstalling KubeArmor as a docker container")
	configPath, err := createDefaultConfigPath()
	if err != nil {
		return err
	}
	// initialize sprig for templating
	sprigFuncs := sprig.GenericFuncMap()

	// write compose file
	composeFilePath, err := utils.CopyOrGenerateFile(config.UserConfigPath, configPath, "docker-compose.yaml", sprigFuncs, kubearmorcomposeTemplate, config)
	if err != nil {
		return err
	}

	args := []string{"-f", composeFilePath, "--profile", "kubearmor", "up", "-d"}
	// need these flags for diagnosis
	if semver.Compare(config.ComposeVersion, utils.MinDockerComposeWithWaitSupported) >= 0 {
		args = append(args, "--wait", "--wait-timeout", "60")
	}
	// run compose command
	_, err = utils.ExecComposeCommand(true, false, config.ComposeCmd, args...)
	if err != nil {
		// cleanup volumes
		_, volDelErr := utils.ExecDockerCommand(true, false, "docker", "volume", "rm", "kubearmor-init-vol")
		if volDelErr != nil {
			fmt.Println("Error while removing volumes:", volDelErr.Error())
		}

		return err
	}

	return nil
}

func fetchTags() ([]string, error) {
	tags := []string{}
	url := DockerHubURL

	for url != "" {
		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var tagResp TagResponse
		if err := json.NewDecoder(resp.Body).Decode(&tagResp); err != nil {
			return nil, err
		}

		for _, tag := range tagResp.Results {
			tags = append(tags, tag.Name)
		}
		url = tagResp.Next
	}

	return tags, nil
}

func findLatestTag(tags []string) string {
	type tagVersion struct {
		original string
		version  float64
	}

	var versions []tagVersion

	for _, tag := range tags {
		parts := strings.Split(tag, "_")
		if len(parts) == 0 {
			continue
		}

		versionStr := parts[0]
		versionParts := strings.Split(versionStr, ".")

		// Convert version to a float (e.g., 1.5.2 → 1.52)
		if len(versionParts) >= 2 {
			major, _ := strconv.Atoi(versionParts[0])
			minor, _ := strconv.Atoi(versionParts[1])
			patch := 0
			if len(versionParts) > 2 {
				patch, _ = strconv.Atoi(versionParts[2])
			}

			floatVersion, _ := strconv.ParseFloat(fmt.Sprintf("%d.%d%d", major, minor, patch), 64)
			versions = append(versions, tagVersion{original: versionStr, version: floatVersion})
		}
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i].version > versions[j].version
	})

	// Return the highest version tag
	if len(versions) > 0 {
		return versions[0].original
	}
	return ""
}

func (config *KubeArmorConfig) DeployKASystemd() error {
	// Download and install agents
	fmt.Printf("ℹ️\tInstalling KubeArmor as a systemd service\n")
	err := config.SystemdInstall()
	if err != nil {
		fmt.Printf("ℹ️\tInstallation failed!! Error: %s.\nCleaning up downloaded assets...\n", err.Error())
		utils.Deletedir(utils.DownloadDir)
		RemoveSystemd() // #nosec G104
		return err
	}

	// Start services

	err = utils.StartSystemdService(config.ServiceName)
	if err != nil {
		fmt.Printf("failed to start service %s: %s\n", config.ServiceName, err.Error())
		return err
	}

	fmt.Printf("🥳\tKubeArmor installed successfully.\n")

	fmt.Printf("Cleaning up downloaded assets...\n")
	utils.Deletedir(utils.DownloadDir)
	return nil
}

func DetectRuntimes() []string {
	runtimes := []string{}
	if _, err := exec.LookPath("podman"); err == nil {
		runtimes = append(runtimes, PodmanRuntime)
	}
	if _, err := exec.LookPath("docker"); err == nil {
		runtimes = append(runtimes, DockerRuntime)
	}
	return runtimes
}

func DetectArchitecture() (string, error) {
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		return "amd64", nil
	case "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("unsupported architecture: %s", arch)
	}
}

func (config *KubeArmorConfig) SystemdInstall() error {
	btfPresent, err := verifyBTF()
	// BTF not present, we need to fail
	if err != nil {
		return fmt.Errorf("failed to look for BTF info: %s", err.Error())
	} else if !btfPresent {
		return fmt.Errorf("\n⚠️\tBTF info not found.")
	}

	err = utils.StopSystemdService(config.ServiceName, true, false)
	if err != nil {
		fmt.Printf("⚠️\tFailed to stop existing systemd service %s: %s\n", config.ServiceName, err.Error())
	}

	fmt.Printf("ℹ️\tDownloading Agent - %s | Image - %s\n", config.AgentName, config.KubeArmorImage)
	packageMeta := utils.SplitLast(config.KubeArmorImage, ":")

	err = config.installAgent(config.AgentName, packageMeta[0], packageMeta[1])
	if err != nil {
		fmt.Println(err)
		return err
	}

	fmt.Printf("😄\t%s version %s downloaded successfully\n", config.AgentName, utils.SplitLast(packageMeta[1], "_")[0])

	err = config.placeServiceFiles()
	if err != nil {
		fmt.Println(err)
		return err
	}
	sprigFuncs := sprig.GenericFuncMap()
	// copy config file
	_, err = utils.CopyOrGenerateFile(config.UserConfigPath, config.AgentDir, config.ConfigFilePath, sprigFuncs, config.ConfigTemplateString, config)
	if err != nil {
		return err
	}

	fmt.Printf("😄\tKubearmor service files placed successfully\n")

	return nil
}

func verifyBTF() (bool, error) {
	btfPath := "/sys/kernel/btf/vmlinux"

	// Check if the file exists
	if _, err := os.Stat(btfPath); err == nil {
		// btf present
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

func SetKAConfig(installOptions *Options) (*KubeArmorConfig, error) {
	config := new(KubeArmorConfig)
	var err error
	if len(installOptions.KubeArmorTag) == 0 {
		tags, err := fetchTags()
		if err != nil {
			return config, fmt.Errorf("Error fetching tags:%s", err.Error())
		}
		latestTag := findLatestTag(tags)
		arch, err := DetectArchitecture()
		if err != nil {
			return nil, err
		}
		tagName := fmt.Sprintf("%s_%s-%s", latestTag, "linux", arch)
		config.KubeArmorTag = tagName
	}
	if installOptions.VmMode == utils.VMMode_Docker {
		if installOptions.InitImage == "" {
			if installOptions.KubeArmorTag != "" {
				config.KubeArmorInitImage = utils.DefaultKubeArmorInitImage + ":" + installOptions.KubeArmorTag
			} else {
				config.KubeArmorInitImage = utils.DefaultKubeArmorInitImage + ":" + utils.DefaultDockerTag
			}
		} else {
			config.KubeArmorInitImage = installOptions.InitImage
		}
		if installOptions.KubearmorImage == "" {
			if installOptions.KubeArmorTag != "" {
				config.KubeArmorImage = utils.DefaultKubeArmorImage + ":" + installOptions.KubeArmorTag
			} else {
				config.KubeArmorImage = utils.DefaultKubeArmorImage + ":" + utils.DefaultDockerTag
			}
		} else {
			config.KubeArmorImage = installOptions.KubearmorImage
		}
	}
	if installOptions.VmMode == utils.VMMode_Systemd {
		if installOptions.KubearmorImage == "" {
			if installOptions.KubeArmorTag != "" {
				config.KubeArmorImage = utils.DefaultKubeArmorSystemdImage + ":" + installOptions.KubeArmorTag
			} else {
				config.KubeArmorImage = utils.DefaultKubeArmorSystemdImage + ":" + config.KubeArmorTag
			}
		} else {
			config.KubeArmorImage = installOptions.KubearmorImage
		}
		config.AgentName = "kubearmor"
		config.ServiceName = "kubearmor.service"
		config.AgentDir = utils.KAconfigPath
		config.ServiceTemplateString = kubearmorServiceFile
		config.ConfigTemplateString = kubeArmorConfig
	}
	config.ImagePullPolicy = installOptions.ImagePullPolicy
	config.Hostname, err = os.Hostname()
	if err != nil {
		fmt.Println(color.YellowString("Failed to get hostname", err.Error()))
	}
	config.Mode = installOptions.VmMode
	config.KubeArmorPort = utils.KubeArmorPort
	// config.HostDefaultFilePosture = installOptions.HostDefaultFilePosture
	config.KubeArmorAlertThrottling = installOptions.AlertThrottling
	config.KubeArmorMaxAlertsPerSec = installOptions.MaxAlertPerSec
	config.KubeArmorThrottleSec = installOptions.ThrottleSec

	config.KubeArmorVisibility = installOptions.Visibility
	if config.KubeArmorVisibility == "" {
		config.KubeArmorVisibility = "process,network"
	}

	config.KubeArmorHostVisibility = installOptions.HostVisibility
	if config.KubeArmorHostVisibility == "" {
		config.KubeArmorHostVisibility = "process,network"
	}
	config.KubeArmorFilePosture = utils.GetDefaultPosture(installOptions.Audit, installOptions.Block, "file")
	config.KubeArmorNetworkPosture = utils.GetDefaultPosture(installOptions.Audit, installOptions.Block, "network")
	config.KubeArmorCapPosture = utils.GetDefaultPosture(installOptions.Audit, installOptions.Block, "capabilities")

	//======= Host Default Postures ========//
	config.KubeArmorHostFilePosture = utils.GetDefaultPosture(installOptions.HostAudit, installOptions.HostBlock, "file")
	config.KubeArmorHostNetworkPosture = utils.GetDefaultPosture(installOptions.HostAudit, installOptions.HostBlock, "network")
	config.KubeArmorHostCapPosture = utils.GetDefaultPosture(installOptions.HostAudit, installOptions.HostBlock, "capabilities")
	config.SecureContainers = installOptions.SecureContainers
	config.ConfigFilePath = "kubearmor.yaml"

	return config, nil
}

func (config *KubeArmorConfig) installAgent(agentName, agentRepo, agentTag string) error {
	fileName, err := config.downloadAgent(agentName, agentRepo, agentTag)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	err = extractAgent(fileName)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	return nil
}

func (config *KubeArmorConfig) placeServiceFiles() error {
	// initialize sprig for templating
	sprigFuncs := sprig.GenericFuncMap()
	_, err := utils.CopyOrGenerateFile("", utils.SystemdDir, config.ServiceName, sprigFuncs, config.ServiceTemplateString, interface{}(nil))
	if err != nil {
		return err
	}
	return nil
}

// extractAgent extracts agent tar
func extractAgent(fileName string) error {
	file, err := os.Open(filepath.Clean(fileName))
	if err != nil {
		fmt.Println("Error opening file:", fileName, err)
		return err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		fmt.Println("Error creating gzip reader:", err)
		return err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("Error reading tar header:", err)
			return err
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}
		rootDir := "/"
		// Sanitize the path to prevent directory traversal
		destPath := filepath.Clean(filepath.Join(rootDir, header.Name)) // #nosec G305
		// Ensure the file is within the intended root directory
		if strings.Contains(destPath, "..") {
			return fmt.Errorf("illegal file path: %s", destPath)
		}

		// Create parent directories
		err = os.MkdirAll(filepath.Dir(destPath), 0o755) // #nosec G301
		if err != nil {
			return err
		}

		file, err := os.Create(destPath)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(file, tarReader) // #nosec G110
		if err != nil {
			return err
		}

		// Preserve executable permission
		if header.Mode&0o111 != 0 {
			err := os.Chmod(destPath, 0o755) // #nosec G302
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// downloadAgent downloads agents as OCI artifiacts
func (config *KubeArmorConfig) downloadAgent(agentName, agentRepo, agentTag string) (string, error) {
	fs, err := file.New(utils.DownloadDir)
	if err != nil {
		return "", err
	}
	defer fs.Close()
	// 1. Connect to a remote repository
	ctx := context.Background()
	repo, err := remote.NewRepository(utils.DefaultDockerRegistry + "/" + agentRepo)
	if err != nil {
		return "", err
	}

	_, err = oras.Copy(ctx, repo, agentTag, fs, agentTag, oras.DefaultCopyOptions)
	if err != nil {
		return "", err
	}

	filepath := path.Join(utils.DownloadDir, agentName+"_"+agentTag+".tar.gz")
	return filepath, nil
}

func (installOptions *KubeArmorConfig) ValidateEnv() (string, error) {
	_, err := exec.LookPath("docker")
	if err != nil {
		return "", fmt.Errorf("Error while looking for docker. Err: %s. Please install docker %s+.", err.Error(), utils.MinDockerVersion)
	}

	serverVersionCmd := exec.Command("docker", "version", "-f", "{{.Server.Version}}")
	serverVersion, err := serverVersionCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return "", errors.New(string(exitErr.Stderr))
		}
		return "", err
	}

	serverVersionStr := strings.TrimSpace(string(serverVersion))
	if serverVersionStr != "" {
		if serverVersionStr[0] != 'v' {
			serverVersionStr = "v" + serverVersionStr
		}

		if semver.Compare(serverVersionStr, utils.MinDockerVersion) < 0 {
			return "", fmt.Errorf("docker version %s not supported", serverVersionStr)
		}
	}

	composeCmd, composeVersion, err := utils.GetComposeCommand()
	if err != nil {
		return "", fmt.Errorf("Error: %s. Please install docker-compose %s+", err.Error(), utils.MinDockerComposeVersion)
	}
	installOptions.ComposeCmd = composeCmd
	installOptions.ComposeVersion = composeVersion

	return fmt.Sprintf("Using %s version %s\n", composeCmd, composeVersion), nil
}

func CheckAndRemoveKAVmInstallation() bool {
	filePath := utils.SystemdDir + "kubearmor.service"
	if _, err := os.Stat(filePath); err == nil {
		// found service file means we have agents as systemd service
		fmt.Printf("ℹ️\tFound kubearmor in systemd mode\n")
		err = RemoveSystemd()
		if err != nil {
			return true
		}
		return true
	} else if !os.IsNotExist(err) {
		fmt.Printf("Error checking service file %s: %v", filePath, err)
	}
	var cfg KubeArmorConfig
	// check for docker containers
	_, err := cfg.ValidateEnv()
	if err == nil {
		contianerID, err := getContainerIDByName("kubearmor")
		if err != nil {
			fmt.Printf("ℹ️\t\nUnable to look for containers")
		}
		if contianerID != "" {
			fmt.Printf("ℹ️\tFound kubearmor in docker mode\n")
			err = stopAndDeleteContainerByID(contianerID)
			if err != nil {
				fmt.Printf("error delete kuberamor container %s", err.Error())
			}
			return true
		}

	}
	return false
}

func RemoveSystemd() error {
	err := utils.StopSystemdService("kubearmor.service", false, true)
	if err != nil {
		fmt.Printf("\nℹ️\terror stopping %s: %s", "kubearmor.service", err)
		return err
	}
	utils.Deletedir(utils.KAconfigPath)
	return nil
}

func getContainerIDByName(containerName string) (string, error) {
	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", err
	}

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return "", err
	}

	for _, c := range containers {
		for _, name := range c.Names {
			if strings.TrimPrefix(name, "/") == containerName {
				return c.ID, nil
			}
		}
	}
	return "", nil // not found
}

func stopAndDeleteContainerByID(containerID string) error {
	configPath, err := utils.GetDefaultConfigPath()
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	composeFilePath := filepath.Join(configPath, "docker-compose.yaml")
	composeExist := false
	_, err = os.Stat(composeFilePath)
	if err == nil {
		composeExist = true
	} else if os.IsNotExist(err) {
		// for handling cases when users might have deleted the docker compose file
		// but agent containers are left running
		composeExist = false
	} else {
		return err
	}
	if composeExist {
		composeCmd, composeVersion, err := utils.GetComposeCommand()
		if err != nil {
			return err
		}
		fmt.Printf("Using %s version %s\n", composeCmd, composeVersion)

		_, err = utils.ExecComposeCommand(true, false, composeCmd,
			"-f", composeFilePath, "--profile", "kubearmor", "down",
			"--volumes")
		if err != nil {
			return fmt.Errorf("error: %s", err.Error())
		}
	} else {
		fmt.Printf("compose not found but container found")
		ctx := context.Background()

		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			return err
		}

		if err := cli.ContainerStop(ctx, containerID, container.StopOptions{}); err != nil {
			return err
		}

		if err := cli.ContainerRemove(ctx, containerID, container.RemoveOptions{}); err != nil {
			return err
		}
	}

	return nil
}
