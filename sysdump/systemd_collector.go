// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package sysdump

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/kubearmor/kubearmor-client/probe"
)

type SystemdCollector struct {
	options Options
}

func NewSystemdCollector(o Options) *SystemdCollector {
	return &SystemdCollector{options: o}
}

func (sc *SystemdCollector) Collect(d string) error {
	var errs errgroup.Group

	errs.Go(func() error { return sc.collectSystemInfo(d) })
	errs.Go(func() error {
		return writeCommandOutput(path.Join(d, "kubearmor-status.txt"), "systemctl", "status", "kubearmor")
	})
	errs.Go(func() error { return sc.collectKubeArmorVersion(d) })
	errs.Go(func() error { return sc.collectJournalctlLogs(d) })
	errs.Go(func() error { return sc.collectResourceUsage(d) })
	errs.Go(func() error { return sc.collectConfigFiles(d) })
	errs.Go(func() error { return sc.collectAppArmorProfiles(d) })
	errs.Go(func() error { return sc.collectProbeData(d) })

	return errs.Wait()
}

func (sc *SystemdCollector) collectSystemInfo(d string) error {
	sysInfo := make(map[string]string)

	if hostname, err := os.Hostname(); err == nil {
		sysInfo["hostname"] = hostname
	}
	if kernelVersion, ok := executeCommandSilent("uname", "-r"); ok {
		sysInfo["kernel_version"] = strings.TrimSpace(kernelVersion)
	}
	if osRelease, err := os.ReadFile("/etc/os-release"); err == nil {
		sysInfo["os_release"] = string(osRelease)
	}
	if uptime, ok := executeCommandSilent("uptime", "-p"); ok {
		sysInfo["uptime"] = strings.TrimSpace(uptime)
	}

	data, _ := json.MarshalIndent(sysInfo, "", "  ")
	return writeToFile(path.Join(d, "system-info.json"), string(data))
}

func (sc *SystemdCollector) collectKubeArmorVersion(d string) error {
	version := ""

	if serviceContent, err := os.ReadFile("/usr/lib/systemd/system/kubearmor.service"); err == nil {
		content := string(serviceContent)
		if strings.Contains(content, "Image=") {
			for _, line := range strings.Split(content, "\n") {
				if strings.Contains(line, "Image=") {
					version = strings.TrimSpace(strings.TrimPrefix(line, "Image="))
					break
				}
			}
		}
	}

	if version == "" {
		if versionContent, err := os.ReadFile("/opt/kubearmor/.version"); err == nil {
			version = strings.TrimSpace(string(versionContent))
		}
	}

	if version == "" {
		if versionOutput, ok := executeCommandSilent("kubearmor", "--version"); ok {
			version = strings.TrimSpace(versionOutput)
		}
	}

	if version == "" {
		version = "unknown"
	}

	return writeToFile(path.Join(d, "kubearmor-version.txt"), version)
}

func (sc *SystemdCollector) collectJournalctlLogs(d string) error {
	dir := path.Join(d, "kubearmor-logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	fmt.Println("Collecting KubeArmor logs from journalctl...")
	writeCommandOutput(path.Join(dir, "all-logs.txt"), "journalctl", "-u", "kubearmor", "-n", "5000", "--no-pager")
	writeCommandOutput(path.Join(dir, "errors-only.txt"), "journalctl", "-u", "kubearmor", "-p", "err", "--no-pager")
	writeCommandOutput(path.Join(dir, "last-hour-logs.txt"), "journalctl", "-u", "kubearmor", "--since", "1 hour ago", "--no-pager")
	writeCommandOutput(path.Join(dir, "logs-json.json"), "journalctl", "-u", "kubearmor", "-n", "1000", "-o", "json")

	return nil
}

func (sc *SystemdCollector) collectResourceUsage(d string) error {
	resInfo := make(map[string]interface{})

	pidOutput, ok := executeCommandSilent("systemctl", "show", "kubearmor", "--property=MainPID", "--value")
	if !ok {
		resInfo["status"] = "failed to retrieve service PID"
		data, _ := json.MarshalIndent(resInfo, "", "  ")
		return writeToFile(path.Join(d, "kubearmor-resources.json"), string(data))
	}

	pidStr := strings.TrimSpace(pidOutput)
	if pidStr == "0" || pidStr == "" {
		resInfo["status"] = "KubeArmor service not running"
		data, _ := json.MarshalIndent(resInfo, "", "  ")
		return writeToFile(path.Join(d, "kubearmor-resources.json"), string(data))
	}

	pidNum, err := strconv.Atoi(pidStr)
	if err != nil {
		resInfo["error"] = fmt.Sprintf("failed to parse PID %s: %v", pidStr, err)
		data, _ := json.MarshalIndent(resInfo, "", "  ")
		return writeToFile(path.Join(d, "kubearmor-resources.json"), string(data))
	}

	resInfo["pid"] = pidNum

	if psOutput, ok := executeCommandSilent("ps", "-p", pidStr, "-o", "pid=,user=,%cpu=,%mem=,rss=,vsz=,etime=,comm="); ok {
		resInfo["ps_output"] = strings.TrimSpace(psOutput)
	}

	if statm, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pidNum)); err == nil {
		fields := strings.Fields(string(statm))
		if len(fields) > 23 {
			vsize, _ := strconv.ParseInt(fields[22], 10, 64)
			rss, _ := strconv.ParseInt(fields[23], 10, 64)
			resInfo["vsize_bytes"] = vsize
			resInfo["rss_bytes"] = rss
		}
	}

	if status, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pidNum)); err == nil {
		resInfo["proc_status"] = string(status)
	}

	if limits, err := os.ReadFile(fmt.Sprintf("/proc/%d/limits", pidNum)); err == nil {
		resInfo["limits"] = string(limits)
	}

	data, _ := json.MarshalIndent(resInfo, "", "  ")
	return writeToFile(path.Join(d, "kubearmor-resources.json"), string(data))
}

func (sc *SystemdCollector) collectConfigFiles(d string) error {
	configDir := path.Join(d, "kubearmor-config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}

	configFiles := []string{
		"/opt/kubearmor/kubearmor.yaml",
		"/opt/kubearmor/kubearmor.conf",
		"/etc/kubearmor/kubearmor.yaml",
		"/etc/kubearmor/kubearmor.conf",
		"/usr/lib/systemd/system/kubearmor.service",
	}

	for _, configFile := range configFiles {
		fmt.Printf("Checking config file: %s\n", configFile)
		copyFileIfExists(configFile, configDir)
	}

	policyDir := "/opt/kubearmor/policies"
	if _, err := os.Stat(policyDir); err == nil {
		destPolicyDir := path.Join(configDir, "policies")
		if err := os.MkdirAll(destPolicyDir, 0o755); err == nil {
			files, _ := os.ReadDir(policyDir)
			for _, file := range files {
				src := path.Join(policyDir, file.Name())
				copyFileIfExists(src, destPolicyDir)
			}
		}
	}

	return nil
}

func (sc *SystemdCollector) collectAppArmorProfiles(d string) error {
	appArmorDir := path.Join(d, "apparmor-profiles")
	if err := os.MkdirAll(appArmorDir, 0o755); err != nil {
		return err
	}

	fmt.Println("Collecting AppArmor profiles...")
	tarPath := path.Join(appArmorDir, "profiles.tar.gz")
	if _, ok := executeCommandSilent("tar", "czf", tarPath, "/etc/apparmor.d/"); !ok {
		if _, err := os.Stat("/etc/apparmor.d"); err == nil {
			files, _ := os.ReadDir("/etc/apparmor.d")
			for _, file := range files {
				src := path.Join("/etc/apparmor.d", file.Name())
				copyFileIfExists(src, appArmorDir)
			}
		}
	}

	writeCommandOutput(path.Join(appArmorDir, "apparmor-status.txt"), "apparmor_status")
	return nil
}

func (sc *SystemdCollector) collectProbeData(d string) error {
	fmt.Println("Collecting KubeArmor probe data...")
	probePath := path.Join(d, "karmor-probe.json")

	if probeData, err := os.ReadFile("/tmp/karmorProbeData.cfg"); err == nil {
		return writeToFile(probePath, string(probeData))
	}

	reader, writer, err := os.Pipe()
	if err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		defer writer.Close()
		probe.PrintProbeResultCmd(nil, probe.Options{
			Namespace: "",
			Full:      false,
			Output:    "json",
			GRPC:      os.Getenv("KUBEARMOR_SERVICE"),
			Writer:    writer,
		})
	}()

	<-ctx.Done()
	out, _ := io.ReadAll(reader)
	if len(out) > 0 {
		return writeToFile(probePath, string(out))
	}

	return nil
}
