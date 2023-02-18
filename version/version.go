// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

// Package version checks the current CLI version and if there's a need to update it
package version

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/blang/semver"
	"github.com/fatih/color"
	"github.com/google/go-github/github"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/kubearmor/kubearmor-client/selfupdate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PrintVersion handler for karmor version
func PrintVersion(c *k8s.Client) error {
	fmt.Printf("karmor version %s %s/%s BuildDate=%s\n", selfupdate.GitSummary, runtime.GOOS, runtime.GOARCH, selfupdate.BuildDate)
	curver := selfupdate.GitSummary
	latest, latestVer := selfupdate.IsLatest(curver)
	if !latest {
		color.HiMagenta("update available version " + latestVer)
		color.HiMagenta("use [karmor selfupdate] to update to latest")
	}

	mandatory, mandatoryVer := isLatestMandatory(curver)
	if !mandatory {
		color.HiMagenta("mandatory update available %s\n", mandatoryVer)
	}
	kubearmorVersion, err := getKubeArmorVersion(c)
	if err != nil {
		return nil
	}
	if kubearmorVersion == "" {
		fmt.Printf("kubearmor not running\n")
		return nil
	}
	fmt.Printf("kubearmor image (running) version %s\n", kubearmorVersion)
	return nil
}

func isLatestMandatory(curver string) (bool, string) {
	if curver != "" && !selfupdate.IsValidVersion(curver) {
		return true, ""
	}

	latestMandatoryRelease, err := GetLatestMandatoryRelease(curver)
	if err != nil {
		fmt.Println("Failed to get info on mandatory release")
		return true, ""
	}

	if latestMandatoryRelease == nil {
		fmt.Println("No mandatory release found")
		return true, ""
	}

	latestMandatory, err := semver.ParseTolerant(*latestMandatoryRelease.TagName)
	if err != nil {
		return true, ""
	}
	return false, latestMandatory.String()
}

// GetLatestMandatoryRelease finds the latest mandatory release in the given repository
// with a version greater than or equal to the given current version (in string format).
// If no such release is found, it returns an empty string and a nil error.
func GetLatestMandatoryRelease(curver string) (*github.RepositoryRelease, error) {
	releases, err := FetchReleases()
	if err != nil {
		return nil, err
	}

	var latestMandatoryRelease *github.RepositoryRelease
	var latestMandatoryReleaseVer *semver.Version

	for _, release := range releases {
		if strings.Contains(*release.Body, "mandatory") || strings.Contains(*release.Body, "MANDATORY") {
			// parse the version string of the release
			releaseVer, err := semver.ParseTolerant(*release.TagName)

			if err != nil {
				// skip the release if the version string is invalid
				continue
			}

			// initialize the latest mandatory release version and release if they are nil
			if latestMandatoryRelease == nil || latestMandatoryReleaseVer == nil {
				latestMandatoryRelease = release
				latestMandatoryReleaseVer = &releaseVer
				continue
			}

			// check if the release version is greater than or equal to the current version
			if curver != "" && releaseVer.GTE(semver.MustParse(curver)) && releaseVer.GT(*latestMandatoryReleaseVer) {
				latestMandatoryRelease = release
				latestMandatoryReleaseVer = &releaseVer
			}
		}
	}

	if latestMandatoryRelease == nil {
		return nil, nil
	}

	return latestMandatoryRelease, nil
}

// FetchReleases fetches the list of all releases in the given repository.
func FetchReleases() ([]*github.RepositoryRelease, error) {
	client := github.NewClient(nil)
	releases, _, err := client.Repositories.ListReleases(context.Background(),
		"kubearmor",
		"kubearmor-client",
		&github.ListOptions{
			Page:    1,
			PerPage: 100,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("error fetching releases from GitHub: %v", err)
	}
	return releases, nil
}

func getKubeArmorVersion(c *k8s.Client) (string, error) {
	pods, err := c.K8sClientset.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{LabelSelector: "kubearmor-app=kubearmor"})
	if err != nil {
		return "", err
	}
	if len(pods.Items) > 0 {
		image := pods.Items[0].Spec.Containers[0].Image
		return image, nil
	}
	return "", nil
}
