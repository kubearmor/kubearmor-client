// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor
package install

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/coreos/go-systemd/v22/dbus"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"gopkg.in/yaml.v2"
	"io"
	"net/http"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"text/template"
)

//go:embed configTemplate.yaml
var configTemplate string

//go:embed composeTemplate.yaml
var composeTemplate string

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

var (
	DefaultSystemdFile string
)

type Config struct {
	HostVisibility            string `json:"hostVisibility"`
	EnableKubeArmorHostPolicy bool   `json:"enableKubeArmorHostPolicy"`
	EnableKubeArmorVm         bool   `json:"enableKubeArmorVm"`
	AlertThrottling           bool   `json:"alertThrottling"`
	MaxAlertPerSec            int    `json:"maxAlertPerSec"`
	ThrottleSec               int    `json:"throttleSec"`
}

type ComposeConfig struct {
	Visibility                     string `json:"visibility"`
	HostVisibility                 string `json:"hostVisibility"`
	EnableKubeArmorHostPolicy      bool   `json:"enableKubeArmorHostPolicy"`
	EnableKubeArmorPolicy          bool   `json:"enableKubeArmorPolicy"`
	DefaultFilePosture             string `json:"defaultFilePosture"`
	DefaultNetworkPosture          string `json:"defaultNetworkPosture"`
	DefaultCapabilitiesPosture     string `json:"defaultCapabilitiesPosture"`
	HostDefaultFilePosture         string `json:"hostDefaultFilePosture"`
	HostDefaultNetworkPosture      string `json:"hostDefaultNetworkPosture"`
	HostDefaultCapabilitiesPosture string `json:"hostDefaultCapabilitiesPosture"`
}

type TagResponse struct {
	Results []struct {
		Name string `json:"name"`
	} `json:"results"`
	Next string `json:"next"`
}

func GenerateKubeArmorConfig(cfg Config) error {
	tmpl, err := template.New("config").Parse(configTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var output bytes.Buffer
	if err := tmpl.Execute(&output, cfg); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// Write to YAML file
	if err := os.WriteFile(ConfigPath, output.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func UpdateConfigCommandInComposeFile(cfg ComposeConfig) error {

	tmpl, err := template.New("composeConfig").Parse(composeTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var output bytes.Buffer
	if err := tmpl.Execute(&output, cfg); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// save to compose file in current directory
	if err := os.WriteFile(DefaultComposeFile, output.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
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

//  EnsureSystemdPackage checks if the systemd package exists locally, if not, it downloads it using ORAS.
func EnsureSystemdPackage(version string) error {
	if _, err := os.Stat(DefaultSystemdFile); os.IsNotExist(err) {
		fmt.Println("Systemd package not found locally. Downloading from KubeArmor repository using ORAS...")

		tags, err := fetchTags()
		if err != nil {
			return fmt.Errorf("Error fetching tags:", err)
		}
		latestTag := findLatestTag(tags)

		arch, err := DetectArchitecture()
		if err != nil {
			return fmt.Errorf("failed to determine system architecture: %w", err)
		}
		fmt.Println("Latest tag and machine type:", latestTag, arch)
		var tagName string
		if version == "latest" {
			tagName = fmt.Sprintf("%s_%s-%s", latestTag, "linux", arch)
		} else {
			tagName = fmt.Sprintf("%s_%s-%s", version, "linux", arch)
		}
		fmt.Println("Fetching Kubearmor version:", tagName)

		// Create a remote registry client
		reg, err :=
			remote.NewRegistry(RemoteHost)
		if err != nil {
			return fmt.Errorf("failed to create registry client: %w", err)
		}

		// Get the repository reference
		ctx := context.Background()
		src, err := reg.Repository(ctx, ImageName)
		if err != nil {
			return fmt.Errorf("failed to reference repository: %w", err)
		}

		// Fetch the manifest by tag
		desc, rc, err := src.FetchReference(ctx, tagName)
		if err != nil {
			return fmt.Errorf("failed to fetch manifest: %w", err)
		}
		defer rc.Close()

		// Read the manifest content
		manifestContent, err := content.ReadAll(rc, desc)
		if err != nil {
			return fmt.Errorf("failed to read manifest content: %w", err)
		}

		// Parse the manifest content
		manifest := &v1.Manifest{}

		if err := json.Unmarshal(manifestContent, &manifest); err != nil {
			return fmt.Errorf("failed to parse manifest content: %w", err)
		}

		// Fetch each blob (layer) from the manifest
		for _, layer := range manifest.Layers {
			fmt.Printf("Fetching layer: Digest=%s, Size=%d, MediaType=%s\n", layer.Digest, layer.Size, layer.MediaType)

			blobContent, err := content.FetchAll(ctx, src, layer)
			if err != nil {
				return fmt.Errorf("failed to fetch layer content: %w", err)
			}

			// Save the blob content to a local file (optional)
			if title, ok := layer.Annotations["org.opencontainers.image.title"]; ok {
				DefaultSystemdFile = title
			}

			err = os.WriteFile(DefaultSystemdFile, blobContent, 0644)
			if err != nil {
				return fmt.Errorf("failed to save layer content to file: %w", err)
			}
			fmt.Printf("Layer content saved to: %s\n", DefaultSystemdFile)
		}

		// Save the manifest content as the main package
		err = os.WriteFile(DefaultSystemdFileConfig, manifestContent, 0644)
		if err != nil {
			return fmt.Errorf("failed to save systemd package to file: %w", err)
		}

		fmt.Println("Systemd package and layers downloaded successfully.")
	}

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

func SelectRuntime(availableRuntimes []string, secureRuntime ...string) string {
	var runtime string

	if len(secureRuntime) > 1 {
		fmt.Println("Only one secureRuntime argument is allowed.")
		os.Exit(1)
	}

	if len(secureRuntime) == 1 && secureRuntime[0] != "" {
		runtime = secureRuntime[0]
		for _, rt := range availableRuntimes {
			if rt == runtime {
				return runtime
			}
		}
		fmt.Printf("Specified runtime '%s' is not available.\n", runtime)
		os.Exit(1)
	}

	if len(availableRuntimes) == 1 {
		return availableRuntimes[0]
	}

	fmt.Println("😄 Available runtimes:", strings.Join(availableRuntimes, ", "))
	fmt.Println("😄 Please select a runtime:")
	for i, rt := range availableRuntimes {
		fmt.Printf("%d: %s\n", i+1, rt)
	}

	var choice int
	fmt.Scanln(&choice)
	if choice < 1 || choice > len(availableRuntimes) {
		fmt.Println("Invalid choice.")
		os.Exit(1)
	}
	return availableRuntimes[choice-1]
}

func ParseAndValidateComposeFile(runtime string) error {
	data, err := os.ReadFile(DefaultComposeFile)
	if err != nil {
		return fmt.Errorf("failed to read compose file: %w", err)
	}

	if _, err := loader.ParseYAML(data); err != nil {
		return fmt.Errorf("failed to parse compose file: %w", err)
	}

	var criSocket string
	switch runtime {
	case PodmanRuntime:
		criSocket = "unix:///run/podman/podman.sock"
	case DockerRuntime:
		criSocket = "unix:///var/run/docker.sock"
	default:
		return fmt.Errorf("unsupported runtime: %s", runtime)
	}

	var composeMap map[string]interface{}
	if err := yaml.Unmarshal(data, &composeMap); err != nil {
		return fmt.Errorf("failed to unmarshal compose file: %w", err)
	}

	services, ok := composeMap["services"].(map[interface{}]interface{})
	if !ok {
		return fmt.Errorf("'services' section missing or invalid")
	}

	kubearmor, ok := services["kubearmor"].(map[interface{}]interface{})
	if !ok {
		return fmt.Errorf("'kubearmor' service missing")
	}

	command, ok := kubearmor["command"].([]interface{})
	if !ok {
		return fmt.Errorf("'command' for 'kubearmor' is missing or invalid")
	}

	found := false
	for j, cmd := range command {
		if strCmd, ok := cmd.(string); ok && strings.HasPrefix(strCmd, "-criSocket=") {
			command[j] = fmt.Sprintf("-criSocket=%s", criSocket)
			found = true
			break
		}
	}

	if !found {
		command = append(command, fmt.Sprintf("-criSocket=%s", criSocket))
	}
	kubearmor["command"] = command
	services["kubearmor"] = kubearmor
	composeMap["services"] = services

	updatedData, err := yaml.Marshal(composeMap)
	if err != nil {
		return fmt.Errorf("failed to marshal compose file: %w", err)
	}

	if err := os.WriteFile(DefaultComposeFile, updatedData, 0644); err != nil {
		return fmt.Errorf("failed to write updated compose file: %w", err)
	}

	return nil
}

func Run(env string) error {
	var cmd *exec.Cmd
	switch env {
	case PodmanRuntime:
		if _, err := exec.LookPath("podman-compose"); err != nil {
			return fmt.Errorf("podman-compose not present please install")
		}
		cmd = exec.Command("podman-compose", "-f", DefaultComposeFile, "down")
		//clean the podman pods before starting
		cmd = exec.Command("podman-compose", "-f", DefaultComposeFile, "up", "-d")
	case DockerRuntime:
		if _, err := exec.LookPath("docker-compose"); err != nil {
			return fmt.Errorf("docker-compose not present please install")
		}
		cmd = exec.Command("docker-compose", "-f", DefaultComposeFile, "down")
		//clean the docker pods before starting
		cmd = exec.Command("docker-compose", "-f", DefaultComposeFile, "up", "-d")
	case SystemdRuntime:
		fmt.Println("Extracting tarball from path...")

		fmt.Printf("Extracting tarball. Path=%s\n", DefaultSystemdFile)

		if err := unpackTarball(DefaultSystemdFile, "/"); err != nil {
			return fmt.Errorf("failed to unpack tarball: %w", err)
		}

		// Reload systemd daemon and start KubeArmor service
		conn, err := dbus.NewSystemConnectionContext(context.Background())
		if err != nil {
			return fmt.Errorf("failed to connect to systemd: %w", err)
		}
		defer conn.Close()

		if err := conn.ReloadContext(context.Background()); err != nil {
			return fmt.Errorf("failed to reload systemd daemon: %w", err)
		}

		if _, err := conn.StartUnitContext(context.Background(), "kubearmor.service", "replace", nil); err != nil {
			return fmt.Errorf("failed to start kubearmor service: %w", err)
		}
		// Return early so we don't try to use a nil cmd.
		return nil
	default:
		return fmt.Errorf("unsupported environment: %s", env)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Uninstall(env string) error {
	var cmd *exec.Cmd
	switch env {
	case PodmanRuntime:
		cmd = exec.Command("podman-compose", "-f", DefaultComposeFile, "down")
	case DockerRuntime:
		cmd = exec.Command("docker-compose", "-f", DefaultComposeFile, "down")
	case SystemdRuntime:
		conn, err := dbus.NewSystemConnectionContext(context.Background())
		if err != nil {
			return fmt.Errorf("failed to connect to systemd: %w", err)
		}
		defer conn.Close()

		unitName := "kubearmor.service"
		if _, err := conn.StopUnitContext(context.Background(), unitName, "replace", nil); err != nil {
			return fmt.Errorf("failed to stop kubearmor service: %w", err)
		}
		if _, err := conn.DisableUnitFilesContext(context.Background(), []string{unitName}, false); err != nil {
			return fmt.Errorf("failed to disable kubearmor service: %w", err)
		}
		if err := conn.ResetFailedUnitContext(context.Background(), unitName); err != nil {
			return fmt.Errorf("failed to reset failed state for kubearmor service: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported environment: %s", env)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func unpackTarball(tarballPath, destDir string) error {
	file, err := os.Open(tarballPath)

	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tarReader := tar.NewReader(gzr)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()

		case tar.TypeSymlink:
			// If the symlink already exists, remove it.
			if _, err := os.Lstat(target); err == nil {
				if err := os.Remove(target); err != nil {
					return fmt.Errorf("failed to remove existing file %s: %w", target, err)
				}
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("failed to create symlink %s -> %s: %w", target, header.Linkname, err)
			}

		default:
			return fmt.Errorf("unknown type: %v in %s", header.Typeflag, header.Name)
		}
	}
	return nil
}

func KubearmorPresentAsSystemd() (bool, error) {
	conn, err := dbus.NewSystemConnectionContext(context.Background())
	if err != nil {
		return false, fmt.Errorf("failed to connect to systemd: %w", err)
	}
	defer conn.Close()

	props, err := conn.GetUnitPropertiesContext(context.Background(), "kubearmor.service")
	if err != nil {
		return false, fmt.Errorf("failed to get properties for kubearmor.service: %w", err)
	}

	activeState, ok := props["ActiveState"].(string)
	if !ok {
		return false, fmt.Errorf("ActiveState property not found or invalid")
	}

	return activeState == "active", nil
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
