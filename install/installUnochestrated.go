// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor
package install

import (
	"fmt"
    "io"
    "net/http"
    "os"
    "os/exec"
    "strings"
    "github.com/compose-spec/compose-go/v2/loader"
    "gopkg.in/yaml.v2"
)
const (
    PodmanRuntime     = "podman"
    DockerRuntime     = "docker"
	ComposeFileURL = "https://raw.githubusercontent.com/itsCheithanya/kubearmor-client/install-nonk8s/docker-compose.yaml"
    DefaultComposeFile = "./docker-compose.yaml"
)

// ensureComposeFile checks if the Compose file exists locally, if not, it downloads it.
func EnsureComposeFile() (string, error) {
    if _, err := os.Stat(DefaultComposeFile); os.IsNotExist(err) {
        fmt.Println("Compose file not found locally. Downloading from KubeArmor repository...")
        resp, err := http.Get(ComposeFileURL)
        if err != nil {
            return "", fmt.Errorf("failed to download Compose file: %w", err)
        }
        defer resp.Body.Close()


        if resp.StatusCode != http.StatusOK {
            return "", fmt.Errorf("failed to fetch Compose file: received status code %d", resp.StatusCode)
        }


        out, err := os.Create(DefaultComposeFile)
        if err != nil {
            return "", fmt.Errorf("failed to create local Compose file: %w", err)
        }
        defer out.Close()


        _, err = io.Copy(out, resp.Body)
        if err != nil {
            return "", fmt.Errorf("failed to save Compose file locally: %w", err)
        }
        fmt.Println("Compose file downloaded successfully.")
    }
    return DefaultComposeFile, nil
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


func SelectRuntime(secureRuntime string, availableRuntimes []string) string {
    if secureRuntime != "" {
        for _, rt := range availableRuntimes {
            if rt == secureRuntime {
                return rt
            }
        }
        fmt.Printf("Specified runtime '%s' is not available.\n", secureRuntime)
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

func ParseAndValidateComposeFile(filePath, runtime string) error {
    data, err := os.ReadFile(filePath)
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

    if err := os.WriteFile(filePath, updatedData, 0644); err != nil {
        return fmt.Errorf("failed to write updated compose file: %w", err)
    }
	 
    fmt.Println("Compose file validated successfully without modification.")
    return nil
}

func RunCompose(env, filePath string) error {
    var cmd *exec.Cmd
    switch env {
    case PodmanRuntime:
		if _, err := exec.LookPath("podman-compose"); err != nil {
			return fmt.Errorf("podman-compose not present please install")
		}
		cmd = exec.Command("podman-compose", "-f", filePath, "down") //clean the podman pods before starting
		cmd = exec.Command("podman-compose", "-f", filePath, "up", "-d")
    case DockerRuntime:
		if _, err := exec.LookPath("docker-compose"); err != nil {
			return fmt.Errorf("docker-compose not present please install")
		}
		cmd = exec.Command("docker-compose", "-f", filePath, "down") //clean the docker pods before starting
		cmd = exec.Command("docker-compose", "-f", filePath, "up", "-d")
    default:
        return fmt.Errorf("unsupported environment: %s", env)
    }

    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    return cmd.Run()
}