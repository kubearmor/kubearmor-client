package k8s

import (
	"github.com/rs/zerolog/log"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"

	ksp "github.com/kubearmor/KubeArmor/pkg/KubeArmorPolicy/client/clientset/versioned/typed/security.kubearmor.com/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // Needed to auth with cloud providers
)

type Client struct {
	K8sClientset kubernetes.Interface
	KSPClientset ksp.SecurityV1Interface
}

func ConnectK8sClient() (*Client, error) {
	var kubeconfig string
	var contextName string

	restClientGetter := genericclioptions.ConfigFlags{
		Context:    &contextName,
		KubeConfig: &kubeconfig,
	}
	rawKubeConfigLoader := restClientGetter.ToRawKubeConfigLoader()

	config, err := rawKubeConfigLoader.ClientConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Error().Msg(err.Error())
		return nil, err
	}

	kspClientset, err := ksp.NewForConfig(config)
	if err != nil {
		log.Error().Msg(err.Error())
		return nil, err
	}

	return &Client{
		K8sClientset: clientset,
		KSPClientset: kspClientset,
	}, nil
}
