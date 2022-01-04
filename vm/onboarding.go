package vm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	tp "github.com/kubearmor/KVMService/src/types"
	"sigs.k8s.io/yaml"
)

func postHttpRequest(eventData []byte, vmAction string, address string) (string, error) {

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
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(respBody), err
}

func VmList(address string) error {

	vmlist, err := postHttpRequest(nil, "vmlist", address)
	if err != nil {
		fmt.Println("Failed to get vm list")
		return err
	}

	if vmlist == "" {
		fmt.Println("No VMs configured")
	} else {
		fmt.Printf("List of configured vms are : \n%s\n", vmlist)
	}

	return nil
}

func Onboarding(eventType string, path string, address string) error {
	var vm tp.K8sKubeArmorExternalWorkloadPolicy

	vmFile, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(vmFile, &vm)
	if err != nil {
		return err
	}

	vmEvent := tp.K8sKubeArmorExternalWorkloadPolicyEvent{
		Type:   eventType,
		Object: vm,
	}

	vmEventData, err := json.Marshal(vmEvent)
	if err != nil {
		return err
	}

	if _, err = postHttpRequest(vmEventData, "vm", address); err != nil {
		return err
	}

	fmt.Println("Success")
	return nil
}
