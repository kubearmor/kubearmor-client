package list

import (
	// tp "github.com/kubearmor/KubeArmor/KubeArmor/types"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"

	tp "github.com/kubearmor/KubeArmor/KubeArmor/types"
	v1 "github.com/kubearmor/KubeArmor/pkg/KubeArmorPolicy/api/security.kubearmor.com/v1"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/olekukonko/tablewriter"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type Options struct {
	Namespace string
}

func ListPolicies(c *k8s.Client, o Options) error {
	env, err := CheckEnv(c)
	if err != nil {
		return err
	}
	if env == "k8s" {
		policies, err := c.KSPClientset.KubeArmorPolicies(o.Namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return err
		}
		printK8SPolicies(policies.Items, o.Namespace)
	} else if env == "systemd" {
		err := printSystemdPolices()
		if err != nil {
			return err
		}
	}

	return nil
}

func printK8SPolicies(policies []v1.KubeArmorPolicy, namespace string) error {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Policy Name", "Namespace"})
	data := [][]string{}

	if len(policies) <= 0 {
		fmt.Printf("No policies found in %s", namespace)
		return nil
	}
	for _, policy := range policies {
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

func printSystemdPolices() error {
	files, err := ioutil.ReadDir("/opt/kubearmor/policies/")
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return errors.New("unable to read policy backups. Try running with elevated priviledges")
		}
		return err
	}
	if len(files) <= 0 {
		fmt.Printf("Unable find host polices")
		return nil
	}
	policies := []tp.K8sKubeArmorHostPolicy{}
	for _, file := range files {
		path := fmt.Sprintf("/opt/kubearmor/policies/%s", file.Name())
		reader, err := os.Open(path)
		if err != nil {
			return err
		}
		fileBytes, err := ioutil.ReadAll(reader)
		if err != nil {
			return err
		}
		policy := tp.K8sKubeArmorHostPolicy{}
		err = yaml.Unmarshal(fileBytes, &policy)
		if err != nil {
			return err
		}
		policies = append(policies, policy)

	}

	if err != nil {
		fmt.Println(err)
	}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Policy Name"})
	data := [][]string{}

	for _, policy := range policies {
		name := policy.Metadata.Name
		data = append(data, []string{name})
	}
	for _, pol := range data {
		table.Append(pol)
	}
	table.Render()
	return nil
}

func CheckEnv(c *k8s.Client) (string, error) {
	// deployment not found check systemd
	_, err := os.Stat("/usr/lib/systemd/system/kubearmor.service")
	if err == nil {
		return "systemd", nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return "", errors.New("unable to detect kubearmor installation")
	} else {
		// check if kamor is running in K8s
		_, err := c.K8sClientset.AppsV1().Deployments("kube-system").Get(context.Background(), "kubearmor-policy-manager", metav1.GetOptions{})
		if err != nil {
			if kerr.IsNotFound(err) {
				return "", errors.New("unable to find kubearmor in cluster. Try running karmor install")
			}
			return "", err
		}
	}
	return "k8s", nil
}
