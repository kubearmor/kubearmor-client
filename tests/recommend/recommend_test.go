// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend_test

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"

	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/kubearmor/kubearmor-client/recommend"
	. "github.com/onsi/gomega"
)

var testOptions recommend.Options
var err error

var client *k8s.Client

var _ = Describe("Recommend", func() {

	BeforeEach(func() {
		testOptions.OutDir = "out"
		testOptions.ReportFile = "report.txt"
		//Initialise k8sClient for all child commands to inherit
		client, err = k8s.ConnectK8sClient()
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		os.RemoveAll(testOptions.OutDir)
		testOptions.Images = []string{}
		testOptions.Namespace = ""
	})

	Describe("Command", func() {
		Context("when called with image name ubuntu", func() {

			It("should fetch the ubuntu:latest image and create a directory ubuntu-latest under `out` folder", func() {
				testOptions.Images = []string{"ubuntu"}
				err = recommend.Recommend(client, testOptions)
				Expect(err).To(BeNil())
				files, err := os.ReadDir(fmt.Sprintf("%s/%s-latest", testOptions.OutDir, testOptions.Images[0]))
				Expect(len(files)).To(BeNumerically(">=", 1))
				fmt.Printf("files: %+v", files)
				Expect(err).To(BeNil())
			})
		})

		Context("when called with namespace wordpress-mysql and labels app=wordpress", func() {

			It("should fetch the pod and create a folder wordpress-mysql-wordpress under `out` directory", func() {
				testOptions.Labels = []string{"app=wordpress"}
				testOptions.Namespace = "wordpress-mysql"
				err = recommend.Recommend(client, testOptions)
				Expect(err).To(BeNil())
				files, err := os.ReadDir(fmt.Sprintf("%s/wordpress-mysql-wordpress", testOptions.OutDir))
				Expect(len(files)).To(BeNumerically(">=", 1))
				fmt.Printf("files: %+v", files)
				Expect(err).To(BeNil())
			})
		})
	})
})
