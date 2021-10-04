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

func Onboarding(eventType string, path string) error {
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

	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}

	request, err := http.NewRequest("POST", "http://127.0.0.1:8080/vm", bytes.NewBuffer(vmEventData))
	request.Header.Set("Content-type", "application/json")
	if err != nil {
		return err
	}

	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	fmt.Printf("SUCCESS\n")
	return nil
}
