package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"testing"

	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/kubearmor/kubearmor-client/selfupdate"
	core "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8_runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestVersionCmd(t *testing.T) {
	testCases := []struct {
		name           string
		objects        []k8_runtime.Object
		expectedOutput string
	}{
		{
			name:           "kubernetes off",
			objects:        nil,
			expectedOutput: fmt.Sprintf("karmor version %s %s/%s BuildDate=%s\n", selfupdate.GitSummary, runtime.GOOS, runtime.GOARCH, selfupdate.BuildDate),
		},

		{
			name:           "kubernetes on",
			expectedOutput: "kubearmor image (running) version kubearmor",
			objects: []k8_runtime.Object{
				&core.Pod{
					ObjectMeta: meta_v1.ObjectMeta{
						Namespace: "",
						Labels:    map[string]string{"kubearmor-app": "kubearmor"},
					},
					Spec: core.PodSpec{
						Containers: []core.Container{
							{
								Image: "kubearmor",
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := fake.NewClientset(tc.objects...)
			originalClient := k8sClient
			k8sClient = &k8s.Client{
				K8sClientset: client,
			}
			defer func() {
				k8sClient = originalClient
			}()
			var buffer bytes.Buffer
			originalStdout := os.Stdout

			defer func() {
				os.Stdout = originalStdout
			}()

			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			os.Stdout = w
			if err := versionCmd.RunE(versionCmd, []string{}); err != nil {
				t.Fatal(err)
			}

			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := io.Copy(&buffer, r); err != nil {
				t.Fatal(err)
			}

			out := buffer.String()
			if !bytes.Contains(buffer.Bytes(), []byte(tc.expectedOutput)) {
				t.Fatalf("expected output to contain %s but got %s", tc.expectedOutput, string(out))
			}
		})
	}
}
