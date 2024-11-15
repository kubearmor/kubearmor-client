// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

// Package selfupdate exposes KubeArmor build details and provides interface to check and update the CLI itself
package selfupdate

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/blang/semver"
	"github.com/fatih/color"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
)

// GitSummary for accuknox-cli git build
var GitSummary string

// BuildDate for accuknox-cli git build
var BuildDate string

const ghrepo = "kubearmor/kubearmor-client"

func isValidVersion(ver string) bool {
	_, err := semver.Make(ver)
	return err == nil
}

// ConfirmUserAction - returns true if user inputs `y`
func ConfirmUserAction(action string) bool {
	fmt.Printf("%s (y/n): ", action)
	input, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil || (input != "y\n" && input != "n\n") {
		fmt.Println("Invalid input")
		return false
	}
	if input == "n\n" {
		return false
	}
	return true
}

func getLatest() (*selfupdate.Release, error) {
	latest, found, err := selfupdate.DetectLatest(ghrepo)
	if err != nil {
		fmt.Println("Error occurred while detecting version:", err)
		return nil, err
	}
	if !found {
		fmt.Println("could not find latest release details")
		return nil, errors.New("could not find latest release")
	}
	return latest, nil
}

// IsLatest - check if the current binary is the latest
func IsLatest(curver string) (bool, string) {
	if curver != "" && !isValidVersion(curver) {
		return true, ""
	}
	latest, err := getLatest()
	if err != nil {
		fmt.Println("failed getting latest info")
		return true, ""
	}
	if curver != "" {
		v := semver.MustParse(curver)
		if latest.Version.LTE(v) {
			fmt.Println("current version is the latest")
			return true, ""
		}
	}
	return false, latest.Version.String()
}

func doSelfUpdate(curver string) error {
	latest, err := getLatest()
	if err != nil {
		return err
	}
	if curver != "" {
		v := semver.MustParse(curver)
		if latest.Version.LTE(v) {
			fmt.Println("current version is the latest")
			return nil
		}
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Println("Could not locate executable path")
		return errors.New("could not locate exec path")
	}
	fmt.Println("updating from " + latest.AssetURL)
	if err := selfupdate.UpdateTo(latest.AssetURL, exe); err != nil {
		if strings.Contains(err.Error(), "permission denied") {
			color.Red("use [sudo karmor selfupdate]")
		}
		return err
	}
	fmt.Println("update successful.")
	return nil
}

// SelfUpdate handler for karmor cli tool
func SelfUpdate(c *k8s.Client) error {
	ver := GitSummary
	fmt.Printf("current karmor version %s\n", ver)
	if !isValidVersion(ver) {
		fmt.Println("version does not match the pattern. Maybe using a locally built karmor!")
		if !ConfirmUserAction("Do you want to update it?") {
			return nil
		}
		return doSelfUpdate("")
	}
	return doSelfUpdate(ver)
}
