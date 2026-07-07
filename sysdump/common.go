// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package sysdump

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/kubernetes/scheme"

	kg "github.com/kubearmor/KubeArmor/KubeArmor/log"
	"github.com/mholt/archiver/v3"
)

func writeToFile(p, v string) error {
	return os.WriteFile(p, []byte(v), 0o600)
}

func writeYaml(p string, o runtime.Object) error {
	var j printers.YAMLPrinter
	w, err := printers.NewTypeSetter(scheme.Scheme).WrapToPrinter(&j, nil)
	if err != nil {
		return err
	}
	var b bytes.Buffer
	if err := w.PrintObj(o, &b); err != nil {
		return err
	}
	return writeToFile(p, b.String())
}

func IsDirEmpty(name string) (bool, error) {
	files, err := os.ReadDir(name)
	if err != nil {
		return false, err
	}
	return len(files) == 0, nil
}

func archiveDump(d string, filename string) (string, error) {
	sysdumpFile := filename
	if filename == "" {
		sysdumpFile = "karmor-sysdump-" + strings.Replace(time.Now().Format(time.UnixDate), ":", "_", -1) + ".zip"
	}
	if err := archiver.Archive([]string{d}, sysdumpFile); err != nil {
		return "", fmt.Errorf("failed to create zip file: %w", err)
	}
	return sysdumpFile, nil
}

func executeCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func executeCommandSilent(name string, args ...string) (string, bool) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err == nil
}

func writeCommandOutput(p string, name string, args ...string) error {
	output, err := executeCommand(name, args...)
	if err != nil {
		kg.Warnf("Error running command %s: %v\n", name, err)
		return nil
	}
	return writeToFile(p, output)
}

func copyFileIfExists(src string, dstDir string) error {
	_, err := os.Stat(src)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	content, err := os.ReadFile(src)
	if err != nil {
		kg.Warnf("Error reading file %s: %v\n", src, err)
		return nil
	}

	filename := path.Base(src)
	dstPath := path.Join(dstDir, filename)
	return writeToFile(dstPath, string(content))
}
