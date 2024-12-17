// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package vm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tp "github.com/kubearmor/KVMService/src/types"
	kg "github.com/kubearmor/KubeArmor/KubeArmor/log"
	"sigs.k8s.io/yaml"
)

func postHTTPRequest(eventData []byte, vmAction string, address string) (string, error) {
	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}

	request, err := http.NewRequest("POST", address+"/"+vmAction, bytes.NewBuffer(eventData))
	request.Header.Set("Content-type", "application/json")
	if err != nil {
		return "", err
	}

	resp, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			kg.Warnf("Error closing http stream %s\n", err)
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(respBody), err
}

// List - Lists all configured VMs
func List(address string) error {
	var endpoints []tp.KVMSEndpoint

	vmlist, err := postHTTPRequest(nil, "vmlist", address)
	if err != nil {
		fmt.Println("Failed to get vm list")
		return err
	}

	err = json.Unmarshal([]byte(vmlist), &endpoints)
	if err != nil {
		fmt.Println("Failed to parse vm list")
		return err
	}

	if len(endpoints) == 0 {
		fmt.Println("No VMs configured")
	} else {
		fmt.Println("-------------------------------------------")
		fmt.Printf(" %-3s| %-15s| %-10s| %s\n", "", "VM Name", "Identity", "Labels")
		fmt.Println("-------------------------------------------")
		for idx, vm := range endpoints {
			fmt.Printf(" %-3s| %-15s| %-10s| %s\n", strconv.Itoa(idx+1),
				vm.VMName, strconv.Itoa(int(vm.Identity)), strings.Join(vm.Labels, "; "))
		}
	}

	return nil
}

// Onboarding - onboards a vm
func Onboarding(eventType string, path string, address string) error {
	var vm tp.KubeArmorVirtualMachinePolicy

	vmFile, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(vmFile, &vm)
	if err != nil {
		return err
	}

	vmEvent := tp.KubeArmorVirtualMachinePolicyEvent{
		Type:   eventType,
		Object: vm,
	}

	vmEventData, err := json.Marshal(vmEvent)
	if err != nil {
		return err
	}

	if _, err = postHTTPRequest(vmEventData, "vm", address); err != nil {
		return err
	}

	fmt.Println("Success")
	return nil
}
