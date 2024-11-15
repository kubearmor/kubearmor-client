// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package vm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	kg "github.com/kubearmor/KubeArmor/KubeArmor/log"
)

// LabelOptions are optional configuration for kArmor vm policy
type LabelOptions struct {
	VMName   string
	VMLabels string
}

// KubeArmorVirtualMachineLabel - Label struct for KVMS control plane
type KubeArmorVirtualMachineLabel struct {
	Type   string              `json:"type"`
	Name   string              `json:"name"`
	Labels []map[string]string `json:"labels,omitempty"`
}

// LabelHandling Function recives path to YAML file with the type of event and HTTP Server
func LabelHandling(t string, o LabelOptions, address string, isKvmsEnv bool) error {
	var respBody []byte

	if isKvmsEnv {

		labelEvent := KubeArmorVirtualMachineLabel{
			Type: t,
			Name: o.VMName,
		}

		if t == "LIST" {
			// List all labels for mentioned VM
			labelEvent.Labels = nil
		} else {
			labelArr := strings.Split(o.VMLabels, ",")

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
		defer func() {
			if err := resp.Body.Close(); err != nil {
				kg.Warnf("Error closing http stream %s\n", err)
			}
		}()

		respBody, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to send label")
		}
	}

	if t == "LIST" {
		if string(respBody) == "" {
			return fmt.Errorf("failed to get label list")
		}
		fmt.Printf("The label list for %s is %s\n", o.VMName, string(respBody))
		return nil
	}

	fmt.Println("Success")
	return nil
}
