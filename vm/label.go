package vm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

//LabelOptions are optional configuration for kArmor vm policy
type LabelOptions struct {
	VmName   string
	VmLabels string
}

// Label struct for KVMS control plane
type KubeArmorVirtualMachineLabel struct {
	Type   string              `json:"type"`
	Name   string              `json:"name"`
	Labels []map[string]string `json:"labels,omitempty"`
}

//LabelHandling Function recives path to YAML file with the type of event and HTTP Server
func LabelHandling(t string, o LabelOptions, address string, isKvmsEnv bool) error {

	var respBody []byte

	if isKvmsEnv {

		labelEvent := KubeArmorVirtualMachineLabel{
			Type: t,
			Name: o.VmName,
		}

		if t == "LIST" {
			// List all labels for mentioned VM
			labelEvent.Labels = nil
		} else {
			labelArr := strings.Split(o.VmLabels, ",")

			for _, labelList := range labelArr {
				label := make(map[string]string)

				labelVal := strings.Split(labelList, ":")
				label[labelVal[0]] = labelVal[1]
				labelEvent.Labels = append(labelEvent.Labels, label)
			}
		}

		labelEventData, err := json.Marshal(labelEvent)
		if err != nil {
			return err
		}

		timeout := time.Duration(5 * time.Second)
		client := http.Client{
			Timeout: timeout,
		}

		request, err := http.NewRequest("POST", address+"/label", bytes.NewBuffer(labelEventData))
		request.Header.Set("Content-type", "application/json")
		if err != nil {
			return fmt.Errorf("failed to manage labels")
		}

		resp, err := client.Do(request)
		if err != nil {
			return fmt.Errorf("failed to manage labels")
		}
		defer resp.Body.Close()

		respBody, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to send label")
		}
	}

	if t == "LIST" {
		if string(respBody) == "" {
			return fmt.Errorf("failed to get label list")
		} else {
			fmt.Printf("The label list for %s is %s\n", o.VmName, string(respBody))
		}
	} else {
		fmt.Println("Success")
	}
	return nil
}
