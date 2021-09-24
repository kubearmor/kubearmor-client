package install

import (
	"context"
	"fmt"

	hsp "github.com/kubearmor/KubeArmor/pkg/KubeArmorHostPolicy/crd"
	ksp "github.com/kubearmor/KubeArmor/pkg/KubeArmorPolicy/crd"
	"github.com/kubearmor/kubearmor-client/k8s"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var kspName = "kubearmorpolicies.security.kubearmor.com"
var hspName = "kubearmorhostpolicies.security.kubearmor.com"

// CreateCustomResourceDefinition creates the CRD and add it into Kubernetes.
func CreateCustomResourceDefinition(c *k8s.Client, crdName string) (*apiextensions.CustomResourceDefinition, error) {
	var crd apiextensions.CustomResourceDefinition
	switch crdName {
	case kspName:
		crd = ksp.GetCRD()
	case hspName:
		crd = hsp.GetCRD()
	}
	_, err := c.APIextClientset.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), &crd, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("CRD %s already exists %+v", crdName, err)
		}
		return nil, fmt.Errorf("failed to create CRD %s: %+v", crdName, err)
	}

	return &crd, nil
}
