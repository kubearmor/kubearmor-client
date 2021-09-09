package get

import (
	"context"
	"fmt"

	kspClient "github.com/kubearmor/KubeArmor/pkg/KubeArmorPolicy/client/clientset/versioned/typed/security.kubearmor.com/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Options struct {
	Namespace string
}

func Resources(c *kspClient.SecurityV1Client, o Options) {
	kspInterface := c.KubeArmorPolicies(o.Namespace)
	policies, err := kspInterface.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("error %v", err)
	}
	if len(policies.Items) > 0 {
		fmt.Printf("Resources found in %s namespace: \n", o.Namespace)
		for _, policy := range policies.Items {
			fmt.Printf("%v \n", policy.Name)
		}
	} else {
		fmt.Printf("No Resource found in %s namespace", o.Namespace)
	}
}
