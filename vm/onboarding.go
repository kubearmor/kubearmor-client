package vm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	tp "github.com/kubearmor/KVMService/src/types"
	"sigs.k8s.io/yaml"
)

func postHttpRequest(buffer *bytes.Buffer, vmAction string, address string) error {

	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}

	request, err := http.NewRequest("POST", address+"/"+vmAction, buffer)
	request.Header.Set("Content-type", "application/json")
	if err != nil {
		return err
	}

	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return err
}

func VmList(address string) error {
	if err := postHttpRequest(nil, "vmlist", address); err != nil {
		fmt.Println("Failed to send http request")
		return err
	}

	fmt.Println("Success")
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

	vmEventBuffer := bytes.NewBuffer(vmEventData)

	if err = postHttpRequest(vmEventBuffer, "vm", address); err != nil {
		return err
	}

	fmt.Println("Success")
	return nil
}
