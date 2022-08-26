package list

import (
	// tp "github.com/kubearmor/KubeArmor/KubeArmor/types"
	"context"
	"fmt"
	"os"

	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/olekukonko/tablewriter"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Options struct {
	Namespace string
}

func ListPolicies(c *k8s.Client, o Options) error {
	policies, err := c.KSPClientset.KubeArmorPolicies(o.Namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Policy Name", "Namespace"})
	data := [][]string{}

	if len(policies.Items) <= 0 {
		fmt.Printf("No policies found in %s", o.Namespace)
		return nil
	}
	for _, policy := range policies.Items {
		name := policy.ObjectMeta.Name
		ns := policy.ObjectMeta.Namespace
		data = append(data, []string{name, ns})
	}

	for _, pol := range data {
		table.Append(pol)
	}
	table.Render()
	return nil
}
