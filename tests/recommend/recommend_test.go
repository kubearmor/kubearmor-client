// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend_test

import (
	"errors"
	"fmt"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"

	. "github.com/onsi/gomega"

	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/kubearmor/kubearmor-client/recommend"
)

var testOptions recommend.Options
var err error

// mkPathFromTag function from recommend/policy.go
func mkPathFromTag(tag string) string {
	r := strings.NewReplacer(
		"/", "-",
		":", "-",
		"\\", "-",
	)
	return r.Replace(tag)
}

var _ = Describe("Recommend", func() {

	BeforeEach(func() {
		testOptions.OutDir = "out"
		testOptions.ReportFile = "report.txt"
		testOptions.Images = []string{}
		testOptions.Tags = []string{}
		testOptions.UseLabels = []string{}
	})

	AfterEach(func() {
		os.RemoveAll(testOptions.OutDir)
	})
	Describe("Command", func() {
		Context("when called without image name", func() {

			It("should show an error if the number of files in the output folder is greater than 1", func() {

				err = recommend.Recommend(&k8s.Client{}, testOptions)
				Expect(err).To(BeNil())
				files, _ := os.ReadDir(testOptions.OutDir)
				Expect(len(files)).To(BeNumerically("<=", 1))

			})

		})
		Context("when called with a single image name", func() {

			It("should not show any error if a folder with image name is created under output directory", func() {
				testOptions.Images = []string{"ubuntu:latest"}
				err = recommend.Recommend(&k8s.Client{}, testOptions)
				Expect(err).To(BeNil())
				files, _ := os.ReadDir(testOptions.OutDir)
				Expect(len(files)).To(BeNumerically("==", 2))
				for _, file := range files {
					if file.IsDir() && file.Name() != mkPathFromTag(testOptions.Images[0]) {
						err = errors.New("Folder with image name not found")
					}
				}
				Expect(err).To(BeNil())

			})

		})
		Context("when called with multiple image names", func() {

			It("should not show any error if a folder for each image name are created under output directory", func() {
				testOptions.Images = []string{"ubuntu:latest", "debian:9", "alpine:latest"}
				testOptions.Tags = []string{}
				testOptions.UseLabels = []string{}
				err = recommend.Recommend(&k8s.Client{}, testOptions)
				Expect(err).To(BeNil())
				files, _ := os.ReadDir(testOptions.OutDir)
				foundMatch := 0
				Expect(len(files)).To(BeNumerically("==", len(testOptions.Images)+1))
				fmt.Println(files)
				fmt.Println(testOptions.Images)
				for _, file := range files {
					for i := range testOptions.Images {
						if file.IsDir() && file.Name() == mkPathFromTag(testOptions.Images[i]) {
							foundMatch++
						}
					}

				}
				Expect(foundMatch).To(BeNumerically("==", len(testOptions.Images)))

			})

		})

	})

})
