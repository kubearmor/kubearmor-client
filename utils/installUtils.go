package utils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"golang.org/x/mod/semver"
)

func GetComposeCommand() (string, string, error) {
	var (
		err            error
		tryComposeCMDs = []string{"docker-compose", "docker compose"}
		minVersion     = MinDockerComposeVersion
		prevCommand    = ""
	)

	for _, command := range tryComposeCMDs {
		version, execErr := ExecComposeCommand(false, false, command, "version", "--short")
		if execErr != nil {
			if err != nil {
				err = fmt.Errorf("%s. while executing %s: %s", err.Error(), command, execErr.Error())
			} else {
				err = fmt.Errorf("while executing %s: %s", command, execErr.Error())
			}

			continue
		}

		composeCmd, finalVersion := compareVersionsAndGetComposeCommand(version, command, minVersion, prevCommand)
		if composeCmd != "" {
			return composeCmd, finalVersion, nil
		}

		// use command with latest version
		prevCommand = command
	}

	if err != nil {
		return "", "", fmt.Errorf("docker requirements not met: %s", err.Error())
	}

	return "", "", fmt.Errorf("docker requirements not met")
}

func ExecComposeCommand(setStdOut, dryRun bool, tryCmd string, args ...string) (string, error) {
	if !strings.Contains(tryCmd, "docker") {
		return "", fmt.Errorf("Command %s not supported", tryCmd)
	}

	composeCmd := new(exec.Cmd)

	cmd := strings.Split(tryCmd, " ")
	if len(cmd) == 1 {

		composeCmd = exec.Command(cmd[0]) // #nosec G204
		if dryRun {
			composeCmd.Args = append(composeCmd.Args, "--dry-run")
		}
		composeCmd.Args = append(composeCmd.Args, args...)

	} else if len(cmd) > 1 {

		// need this to handle docker compose command
		composeCmd = exec.Command(cmd[0], cmd[1]) // #nosec G204
		if dryRun {
			composeCmd.Args = append(composeCmd.Args, "--dry-run")
		}
		composeCmd.Args = append(composeCmd.Args, args...)

	} else {
		return "", fmt.Errorf("unknown compose command")
	}
	if setStdOut {
		composeCmd.Stdout = os.Stdout
		composeCmd.Stderr = os.Stderr

		err := composeCmd.Run()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
				return "", errors.New(string(exitErr.Stderr))
			}
			return "", err
		}

		return "", nil
	}

	stdout, err := composeCmd.CombinedOutput()
	if err != nil {
		return string(stdout), err
	}

	return string(stdout), nil
}

func compareVersionsAndGetComposeCommand(v1, v1Cmd, v2, v2Cmd string) (string, string) {
	v1Clean := strings.TrimSpace(string(v1))
	v2Clean := strings.TrimSpace(string(v2))

	if v1Clean != "" && v2Clean != "" {
		if v1Clean[0] != 'v' {
			v1Clean = "v" + v1Clean
		}

		if v2Clean[0] != 'v' {
			v2Clean = "v" + v2Clean
		}

		if semver.Compare(v1Clean, v2Clean) >= 0 && semver.Compare(v1Clean, MinDockerComposeVersion) >= 0 {
			return v1Cmd, v1Clean
		} else if semver.Compare(v1Clean, v2Clean) <= 0 && semver.Compare(v2Clean, MinDockerComposeVersion) >= 0 {
			return v2Cmd, v2Clean
		} else {
			return "", ""
		}

	} else if v1Clean != "" {
		if v1Clean[0] != 'v' {
			v1Clean = "v" + v1Clean
		}

		if semver.Compare(v1Clean, MinDockerComposeVersion) >= 0 {
			return v1Cmd, v1Clean
		} else {
			return "", ""
		}
	} else if v2Clean != "" {
		if v2Clean[0] != 'v' {
			v2Clean = "v" + v2Clean
		}

		if semver.Compare(v2Clean, MinDockerComposeVersion) >= 0 {
			return v2Cmd, v2Clean
		} else {
			return "", ""
		}
	}

	return "", ""
}

// copyOrGenerateFile copies a a config file from userConfigDir to the given path or writes file with the given template at the given path
func CopyOrGenerateFile(userConfigDir, dirPath, filePath string, tempFuncs template.FuncMap, templateString string, templateArgs interface{}) (string, error) {
	dataFile := &bytes.Buffer{}

	// if user specified a config path - read if the given file
	// exists in it and skip template generation
	if userConfigDir != "" {
		userConfigFilePath := filepath.Join(userConfigDir, filePath)
		if _, err := os.Stat(userConfigFilePath); err != nil {
			return "", fmt.Errorf("error while opening user specified file: %s", err.Error())
		}

		userFileBytes, err := os.ReadFile(userConfigFilePath) // #nosec G304
		if err != nil {
			return "", err
		} else if len(userFileBytes) == 0 {
			return "", fmt.Errorf("empty config file given at %s", userConfigFilePath)
		}

		dataFile = bytes.NewBuffer(userFileBytes)

	} else if tempFuncs != nil {
		// generate the file with the template
		templateFile, err := template.New(filePath).Funcs(tempFuncs).Parse(templateString)
		if err != nil {
			return "", err
		}

		err = templateFile.Execute(dataFile, templateArgs)
		if err != nil {
			return "", err
		}
	}

	if dataFile == nil || len(dataFile.Bytes()) == 0 {
		return "", fmt.Errorf("Failed to read config file for %s: Empty file", filePath)
	}

	fullFilePath := filepath.Join(dirPath, filePath)
	fullFileDir := filepath.Dir(fullFilePath)

	// create needed directories at the path to write
	err := os.MkdirAll(fullFileDir, os.ModeDir|os.ModePerm)
	if err != nil && !os.IsExist(err) {
		return "", err
	}

	// ignoring G304 - fullFilePath contains the path to configDir - hard coding
	// paths won't be efficient
	// ignoring G302 - if containers are run by the root user, members of the
	// docker group should be able to read the files
	// overwrite files if need
	resultFile, err := os.OpenFile(fullFilePath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644) // #nosec G304 G302
	if err != nil {
		return "", err
	}
	defer resultFile.Close()

	_, err = dataFile.WriteTo(resultFile)
	if err != nil {
		return "", err
	}

	return fullFilePath, nil
}

// GetDefaultConfigPath returns home dir along with an error
func GetDefaultConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	if homeDir == "" {
		return "", fmt.Errorf("Home directory not found")
	}

	configPath := filepath.Join(homeDir, DefaultConfigPathDirName)

	return configPath, nil
}

func ExecDockerCommand(setStdOut, dryRun bool, tryCmd string, args ...string) (string, error) {
	dockerCmd := exec.Command(tryCmd) // #nosec G204
	if dryRun {
		dockerCmd.Args = append(dockerCmd.Args, "--dry-run")
	}

	dockerCmd.Args = append(dockerCmd.Args, args...)

	if setStdOut {
		dockerCmd.Stdout = os.Stdout
		dockerCmd.Stderr = os.Stderr

		err := dockerCmd.Run()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
				return "", errors.New(string(exitErr.Stderr))
			}

			return "", err
		}

		return "", nil
	}

	stdout, err := dockerCmd.CombinedOutput()
	if err != nil {
		return string(stdout), err
	}

	return string(stdout), nil
}

func StopSystemdService(serviceName string, skipDeleteDisable, force bool) error {
	ctx := context.Background()
	conn, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to systemd: %v", err)
	}
	defer conn.Close()

	stopChan := make(chan string)

	property, err := conn.GetUnitPropertyContext(ctx, serviceName, "ActiveState")
	if err != nil {
		return fmt.Errorf("Failed to check service status: %s", err.Error())
	}

	// service not active, return
	if property.Value.Value() != "active" && !force {
		return nil
	}

	if _, err := conn.StopUnitContext(ctx, serviceName, "replace", stopChan); err != nil {
		if !strings.Contains(err.Error(), "not loaded") {
			return fmt.Errorf("Failed to stop existing %s service: %v\n", serviceName, err)
		}
	} else {
		fmt.Printf("ℹ️\tStopping existing %s...\n", serviceName)
		<-stopChan
		fmt.Printf("ℹ️\t%s stopped successfully.\n", serviceName)
	}

	if !skipDeleteDisable {
		if _, err := conn.DisableUnitFilesContext(ctx, []string{serviceName}, false); err != nil {
			if !strings.Contains(err.Error(), "does not exist") {
				fmt.Printf("Failed to disable %s : %v", serviceName, err)
				return err
			}
		} else {
			fmt.Printf("Disabled %s \n", serviceName)
		}

		svcfilePath := SystemdDir + serviceName
		if err := os.Remove(svcfilePath); err != nil {
			if !os.IsNotExist(err) {
				fmt.Printf("Failed to delete %s file: %v", serviceName, err)
				return err
			}
		}

		// reload systemd config, equivalent to systemctl daemon-reload
		if err := conn.ReloadContext(ctx); err != nil {
			return fmt.Errorf("failed to reload systemd configuration: %v", err)
		}
	}

	return nil
}

// splitLast splits at the last index of separator
func SplitLast(fullString, seperator string) []string {
	colonIdx := strings.LastIndex(fullString, seperator)

	// bound check
	if colonIdx <= 0 || colonIdx == (len(fullString)-1) {
		return []string{fullString}
	}

	return []string{fullString[:colonIdx], fullString[colonIdx+1:]}
}

func Deletedir(dirName string) {
	//	Clean Up
	err := os.RemoveAll(dirName)
	if err != nil && !os.IsNotExist(err) {
		// Check if the error is due to the directory not existing
		fmt.Printf("error deleting %s : %v", dirName, err)
	}
}

func StartSystemdService(serviceName string) error {
	if serviceName == "" {
		return nil
	}
	ctx := context.Background()
	// Connect to systemd dbus
	conn, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to systemd: %v", err)
	}
	defer conn.Close()

	// reload systemd config, equivalent to systemctl daemon-reload
	if err := conn.ReloadContext(ctx); err != nil {
		return fmt.Errorf("failed to reload systemd configuration: %v", err)
	}

	// enable service
	_, _, err = conn.EnableUnitFilesContext(ctx, []string{serviceName}, false, true)
	if err != nil {
		return fmt.Errorf("failed to enable %s: %v", serviceName, err)
	}

	// Start the service
	ch := make(chan string)
	if _, err := conn.RestartUnitContext(ctx, serviceName, "replace", ch); err != nil {
		return fmt.Errorf("failed to start %s: %v", serviceName, err)
	}
	fmt.Printf("🔥\tStarted %s\n", serviceName)

	return nil
}

func StopAndDeleteContainer(containerName string) error {
	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	for _, obj := range containers {
		for _, name := range obj.Names {
			if strings.TrimPrefix(name, "/") == containerName {
				fmt.Printf("Found container '%s' (ID: %s). Stopping and removing...\n", containerName, obj.ID)

				// Stop container (graceful stop)
				if err := cli.ContainerStop(ctx, obj.ID, container.StopOptions{}); err != nil {
					return fmt.Errorf("failed to stop container: %w", err)
				}

				// Remove container
				if err := cli.ContainerRemove(ctx, obj.ID, container.RemoveOptions{}); err != nil {
					return fmt.Errorf("failed to remove container: %w", err)
				}

				fmt.Printf("Container '%s' successfully deleted.\n", containerName)
				return nil
			}
		}
	}
	return nil
}

func GetDefaultPosture(auditPostureVal, blockPostureVal, ruleType string) string {
	if auditPostureVal == "all" || (auditPostureVal == "" && blockPostureVal == "") {
		return "audit"
	} else if blockPostureVal == "all" {
		return "block"
	}

	if strings.Contains(auditPostureVal, ruleType) {
		return "audit"
	} else if strings.Contains(blockPostureVal, ruleType) {
		return "block"
	}

	// unrecognized or default
	return "audit"
}
