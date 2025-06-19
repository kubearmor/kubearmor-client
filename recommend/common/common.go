// SPDX-License-Identifier: Apache-2.0
// Copyright 2023 Authors of KubeArmor

// Package common contains object types used by multiple packages
package common

import (
	"os"
	"runtime"
	"strings"

	pol "github.com/kubearmor/KubeArmor/pkg/KubeArmorController/api/security.kubearmor.com/v1"
)

// Handler interface
var Handler interface{}

// LabelMap is an alias for map[string]string
type LabelMap = map[string]string

type Client interface {
	ListObjects(o Options) ([]Object, error)
}

// Object contains brief information about a k8s object
type Object struct {
	Name      string
	Namespace string
	Labels    LabelMap
	Images    []string
}

// MatchSpec spec to match for defining policy
type MatchSpec struct {
	Name         string                  `json:"name" yaml:"name"`
	Precondition []string                `json:"precondition" yaml:"precondition"`
	Description  Description             `json:"description" yaml:"description"`
	Yaml         string                  `json:"yaml" yaml:"yaml"`
	Spec         pol.KubeArmorPolicySpec `json:"spec,omitempty" yaml:"spec,omitempty"`
}

// Ref for the policy rules
type Ref struct {
	Name string   `json:"name" yaml:"name"`
	URL  []string `json:"url" yaml:"url"`
}

// Description detailed description for the policy rule
type Description struct {
	Refs     []Ref  `json:"refs" yaml:"refs"`
	Tldr     string `json:"tldr" yaml:"tldr"`
	Detailed string `json:"detailed" yaml:"detailed"`
}

// Options for karmor recommend
type Options struct {
	Images     []string
	Labels     []string
	Tags       []string
	Policy     []string
	Namespace  string
	OutDir     string
	ReportFile string
	Config     string
	K8s        bool
}

// UserHome function returns users home directory
func UserHome() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}

func labelSplitter(r rune) bool {
	return r == ':' || r == '='
}
func LabelArrayToLabelMap(labels []string) LabelMap {
	labelMap := LabelMap{}
	for _, label := range labels {
		kvPair := strings.FieldsFunc(label, labelSplitter)
		if len(kvPair) != 2 {
			continue
		}
		labelMap[kvPair[0]] = kvPair[1]
	}
	return labelMap
}
