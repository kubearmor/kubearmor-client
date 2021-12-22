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

	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type VmOption struct {
	PolicyFile string
}

// KubeArmorExternalWorkloadPolicyEvent Structure
type KubeArmorExternalWorkloadPolicyEvent struct {
	Type   string                          `json:"type"`
	Object KubeArmorExternalWorkloadPolicy `json:"object"`
}

// KubeArmorExternalWorkloadPolicy Structure
type KubeArmorExternalWorkloadPolicy struct {
	Metadata metav1.ObjectMeta                     `json:"metadata"`
	Spec     ExternalWorkloadSecuritySpec          `json:"spec"`
	Status   KubeArmorExternalWorkloadPolicyStatus `json:"status,omitempty"`
}

type KubeArmorExternalWorkloadPolicyStatus struct {
	ID     uint64 `json:"id,omitempty"`
	IP     string `json:"ip,omitempty"`
	Status string `json:"status,omitempty"`
}

// ExternalWorkloadSecuritySpec Structure
type ExternalWorkloadSecuritySpec struct {
	IPv4AllocCIDR string `json:"ipv4-alloc-cidr,omitempty"`
	IPv6AllocCIDR string `json:"ipv6-alloc-cidr,omitempty"`
}

func postVmEventToControlPlane(vmEvent KubeArmorExternalWorkloadPolicyEvent) error {
	var err error

	requestBody, err := json.Marshal(vmEvent)
	if err != nil {
		log.Fatal(err.Error())
		return err
	}

	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}

	request, err := http.NewRequest("POST", "http://127.0.0.1:8080/vm", bytes.NewBuffer(requestBody))
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

func parseVmYamlFile(path string) (KubeArmorExternalWorkloadPolicy, error) {

	vm := KubeArmorExternalWorkloadPolicy{}
	var err error

	vmYaml, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		log.Fatal(err.Error())
		return vm, err
	}

	err = yaml.Unmarshal(vmYaml, &vm)
	if err != nil {
		log.Fatal(err.Error())
		return vm, err
	}
	return vm, err
}

func VmAdd(path string) error {

	vmEvent := KubeArmorExternalWorkloadPolicyEvent{}

	vm, err := parseVmYamlFile(filepath.Clean(path))
	if err == nil {
		vmEvent = KubeArmorExternalWorkloadPolicyEvent{
			Type:   "ADDED",
			Object: vm,
		}

		err = postVmEventToControlPlane(vmEvent)
		if err != nil {
			return err
		}
	}

	return err
}

func VmUpdate(path string) error {

	vmEvent := KubeArmorExternalWorkloadPolicyEvent{}

	vm, err := parseVmYamlFile(filepath.Clean(path))
	if err == nil {
		vmEvent = KubeArmorExternalWorkloadPolicyEvent{
			Type:   "MODIFIED",
			Object: vm,
		}

		err = postVmEventToControlPlane(vmEvent)
		if err != nil {
			return err
		}
	}

	return err
}

func VmDelete(path string) error {

	vmEvent := KubeArmorExternalWorkloadPolicyEvent{}

	vm, err := parseVmYamlFile(filepath.Clean(path))
	if err == nil {
		vmEvent = KubeArmorExternalWorkloadPolicyEvent{
			Type:   "DELETED",
			Object: vm,
		}

		err = postVmEventToControlPlane(vmEvent)
		if err != nil {
			return err
		}
	}

	return err
}

func VmList() error {

	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}

	request, err := http.NewRequest("POST", "http://127.0.0.1:8080/vmlist", nil)
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

	fmt.Printf("Configured list of VMs for nonk8s control plane is : %v\n", string(respBody))
	return err
}
