package k8s

import (
	"flag"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	client "github.com/kubearmor/KubeArmor/pkg/KubeArmorPolicy/client/clientset/versioned/typed/security.kubearmor.com/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // Needed to auth with cloud providers
)

func ConnectK8sClient() (*kubernetes.Clientset, *client.SecurityV1Client, error) {
	var kubeconfig *string
	homeDir := ""
	if h := os.Getenv("HOME"); h != "" {
		homeDir = h
	} else {
		homeDir = os.Getenv("USERPROFILE") // windows
	}

	if home := homeDir; home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		return nil, nil, err
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Error().Msg(err.Error())
		return nil, nil, err
	}

	ctrlClient, err := client.NewForConfig(config)
	if err != nil {
		log.Error().Msg(err.Error())
		return clientset, nil, err
	}

	return clientset, ctrlClient, nil
}
