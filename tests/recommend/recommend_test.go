// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"

	"github.com/google/go-cmp/cmp"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/kubearmor/kubearmor-client/recommend"
	. "github.com/onsi/gomega"
)

var testOptions recommend.Options
var err error

var client *k8s.Client

func compareData(file1, file2 string) bool {
	data1, err := os.ReadFile(filepath.Clean(file1))
	if err != nil {
		return false
	}
	data2, err := os.ReadFile(filepath.Clean(file2))
	if err != nil {
		return false
	}
	return cmp.Equal(data1, data2)
}

var _ = Describe("karmor", func() {

	BeforeEach(func() {
		testOptions.OutDir = "out"
		testOptions.ReportFile = "report.txt"
		//Initialise k8sClient for all child commands to inherit
		client, err = k8s.ConnectK8sClient()
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		testOptions = recommend.Options{}
	})

	Describe("recommend", func() {

		Context("when called with `update` command", func() {

			It("should fetch the latest policy-template release and modify the rule under ~/.cache/karmor/", func() {
				//os.MkdirAll(testOptions.OutDir, 0777)
				_, err := recommend.DownloadAndUnzipRelease()
				Expect(err).To(BeNil())
				files, err := os.ReadDir(fmt.Sprintf("%s/.cache/karmor", os.Getenv("HOME")))
				Expect(err).To(BeNil())
				Expect(len(files)).To(BeNumerically(">=", 1))
				//os.RemoveAll(testOptions.OutDir)
			})
		})

		Context("when called with --image=ubuntu:18.04", func() {
			var files []fs.DirEntry
			count := 0
			It("should fetch the ubuntu:18.04 image and create a directory `ubuntu-18-04` under `out` folder", func() {
				testOptions.Images = []string{"ubuntu:18.04"}
				err = recommend.Recommend(client, testOptions)
				Expect(err).To(BeNil())
				files, err = os.ReadDir(fmt.Sprintf("%s/ubuntu-18-04", testOptions.OutDir))
				Expect(len(files)).To(BeNumerically(">=", 1))
				Expect(err).To(BeNil())
			})
			It("should contain `4` policy files under directory `ubuntu-18-04` and should match with the files under `res/out/ubuntu-18-04`", func() {
				testOptions.Images = []string{"ubuntu:18.04"}
				for _, file := range files {
					filesRes, err := os.ReadDir("res/out/ubuntu-18-04")
					Expect(err).To(BeNil())
					for _, fileRes := range filesRes {
						if file.Name() == fileRes.Name() {
							if compareData(testOptions.OutDir+"/ubuntu-18-04/"+file.Name(), "res/out/ubuntu-18-04/"+fileRes.Name()) {
								count++
							}
						}
					}
				}
				Expect(count).To(BeNumerically("==", len(files)))
			})
		})

		Context("when called with --image=ubuntu and --outdir=ubuntu-test", func() {
			var files []fs.DirEntry
			count := 0
			It("should fetch the ubuntu:18.04 image and create a directory `ubuntu-18-04` under `ubuntu-test` folder", func() {
				testOptions.OutDir = "ubuntu-test"
				testOptions.Images = []string{"ubuntu:18.04"}
				err = recommend.Recommend(client, testOptions)
				Expect(err).To(BeNil())
				files, err = os.ReadDir(fmt.Sprintf("%s/ubuntu-18-04", testOptions.OutDir))
				Expect(len(files)).To(BeNumerically(">=", 1))
				Expect(err).To(BeNil())
			})
			It("should contain `4` policy files under directory `ubuntu-18-04` and should match with the files under `res/out/ubuntu-18-04`", func() {
				testOptions.OutDir = "ubuntu-test"
				testOptions.Images = []string{"ubuntu:18.04"}
				for _, file := range files {
					filesRes, err := os.ReadDir("res/out/ubuntu-18-04")
					Expect(err).To(BeNil())
					for _, fileRes := range filesRes {
						if file.Name() == fileRes.Name() {
							if compareData(testOptions.OutDir+"/ubuntu-18-04/"+file.Name(), "res/out/ubuntu-18-04/"+fileRes.Name()) {
								count++
							}
						}
					}
				}
				Expect(count).To(BeNumerically("==", len(files)))
			})
		})

		Context("when called with --namespace=wordpress-mysql and --labels=app=wordpress", func() {
			var files []fs.DirEntry
			count := 0
			It("should fetch the image and create a folder wordpress-mysql-wordpress under `out` directory", func() {
				testOptions.Labels = []string{"app=wordpress"}
				testOptions.Namespace = "wordpress-mysql"
				err = recommend.Recommend(client, testOptions)
				Expect(err).To(BeNil())
				files, err = os.ReadDir(fmt.Sprintf("%s/wordpress-mysql-wordpress", testOptions.OutDir))
				Expect(len(files)).To(BeNumerically(">=", 1))
				Expect(err).To(BeNil())
			})
			It("should contain `6` policy files under directory `wordpress-mysql-wordpress` and should match with the files under `res/out/wordpress-mysql-wordpress`", func() {
				testOptions.Labels = []string{"app=wordpress"}
				testOptions.Namespace = "wordpress-mysql"
				for _, file := range files {
					filesRes, err := os.ReadDir("res/out/wordpress-mysql-wordpress")
					Expect(err).To(BeNil())
					for _, fileRes := range filesRes {
						if file.Name() == fileRes.Name() {
							if compareData(testOptions.OutDir+"/wordpress-mysql-wordpress/"+file.Name(), "res/out/wordpress-mysql-wordpress/"+fileRes.Name()) {
								count++
							}
						}
					}
				}
				Expect(count).To(BeNumerically("==", len(files)))
			})
		})

		Context("when called with --namespace=wordpress-mysql , --labels=app=wordpress and --outdir=wordpress-test", func() {
			var files []fs.DirEntry
			count := 0
			It("should fetch the image and create a folder wordpress-mysql-wordpress under `wordpress-test` directory", func() {
				testOptions.Labels = []string{"app=wordpress"}
				testOptions.Namespace = "wordpress-mysql"
				testOptions.OutDir = "wordpress-test"
				err = recommend.Recommend(client, testOptions)
				Expect(err).To(BeNil())
				files, err = os.ReadDir(fmt.Sprintf("%s/wordpress-mysql-wordpress", testOptions.OutDir))
				Expect(len(files)).To(BeNumerically(">=", 1))
				Expect(err).To(BeNil())
			})
			It("should contain `6` policy files under directory `wordpress-mysql-wordpress` and should match with the files under `res/out/wordpress-mysql-wordpress`", func() {
				testOptions.Labels = []string{"app=wordpress"}
				testOptions.Namespace = "wordpress-mysql"
				testOptions.OutDir = "wordpress-test"
				for _, file := range files {
					filesRes, err := os.ReadDir("res/out/wordpress-mysql-wordpress")
					Expect(err).To(BeNil())
					for _, fileRes := range filesRes {
						if file.Name() == fileRes.Name() {
							if compareData(testOptions.OutDir+"/wordpress-mysql-wordpress/"+file.Name(), "res/out/wordpress-mysql-wordpress/"+fileRes.Name()) {
								count++
							}
						}
					}
				}
				Expect(count).To(BeNumerically("==", len(files)))
			})
		})
	})
})
