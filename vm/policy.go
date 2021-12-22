package vm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"time"

	tp "github.com/kubearmor/KubeArmor/KubeArmor/types"
	"sigs.k8s.io/yaml"
)

type PolicyOption struct {
	PolicyFile string
}

func postPolicyEventToControlPlane(policyEvent tp.K8sKubeArmorHostPolicyEvent) error {
	var err error

	requestBody, err := json.Marshal(policyEvent)
	if err != nil {
		log.Fatal(err.Error())
		return err
	}

	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}

	request, err := http.NewRequest("POST", "http://127.0.0.1:8080/policy", bytes.NewBuffer(requestBody))
	request.Header.Set("Content-type", "application/json")
	if err != nil {
		return err
	}

	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err.Error())
		return err
	}

	fmt.Println(string(respBody))

	return err
}

func parsePolicyYamlFile(path string) (tp.K8sKubeArmorHostPolicy, error) {

	policy := tp.K8sKubeArmorHostPolicy{}
	var err error

	policyYaml, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		return policy, err
	}

	err = yaml.Unmarshal(policyYaml, &policy)
	if err != nil {
		return policy, err
	}

	return policy, err
}

func PolicyAdd(path string) error {

	policy := tp.K8sKubeArmorHostPolicy{}
	policyEvent := tp.K8sKubeArmorHostPolicyEvent{}

	policy, err := parsePolicyYamlFile(filepath.Clean(path))
	if err == nil {
		policyEvent = tp.K8sKubeArmorHostPolicyEvent{
			Type:   "ADDED",
			Object: policy,
		}

		err = postPolicyEventToControlPlane(policyEvent)
		if err != nil {
			return err
		}
	}

	return err
}

func PolicyUpdate(path string) error {

	policy := tp.K8sKubeArmorHostPolicy{}
	policyEvent := tp.K8sKubeArmorHostPolicyEvent{}

	policy, err := parsePolicyYamlFile(filepath.Clean(path))
	if err == nil {
		policyEvent = tp.K8sKubeArmorHostPolicyEvent{
			Type:   "MODIFIED",
			Object: policy,
		}

		err = postPolicyEventToControlPlane(policyEvent)
		if err != nil {
			return err
		}
	}

	return err
}

func PolicyDelete(path string) error {
	policy := tp.K8sKubeArmorHostPolicy{}
	policyEvent := tp.K8sKubeArmorHostPolicyEvent{}

	policy, err := parsePolicyYamlFile(filepath.Clean(path))
	if err == nil {
		policyEvent = tp.K8sKubeArmorHostPolicyEvent{
			Type:   "DELETED",
			Object: policy,
		}

		err = postPolicyEventToControlPlane(policyEvent)
		if err != nil {
			return err
		}
	}

	return err
}
