package list

import (
	// tp "github.com/kubearmor/KubeArmor/KubeArmor/types"

	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"

	v1 "github.com/kubearmor/KubeArmor/pkg/KubeArmorPolicy/api/security.kubearmor.com/v1"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/olekukonko/tablewriter"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Options struct {
	Namespace string
	Output    string
}

//HostPolicyBackup
type HostPolicyBackup struct {
	Metadata struct {
		PolicyName string `json:"policyName"`
	} `json:"metadata"`
	Spec struct {
		NodeSelector struct {
			MatchLabels struct {
				KubernetesIoHostname string `json:"kubernetes.io/hostname"`
			} `json:"matchLabels"`
			Identities []string `json:"identities"`
		} `json:"nodeSelector"`
		Process struct {
		} `json:"process"`
		File struct {
			MatchPaths []struct {
				Path       string `json:"path"`
				FromSource []struct {
					Path string `json:"path"`
				} `json:"fromSource"`
				Severity int    `json:"severity"`
				Action   string `json:"action"`
			} `json:"matchPaths"`
		} `json:"file"`
		Network struct {
		} `json:"network"`
		Capabilities struct {
		} `json:"capabilities"`
		Severity int    `json:"severity"`
		Action   string `json:"action"`
	} `json:"spec"`
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
		switch format := o.Output; format {
		case "json":
			result, err := json.MarshalIndent(policies.Items, "", "")
			if err != nil {
				return err
			}
			fmt.Println(string(result))
			return nil
		default:
			err = printK8SPolicies(policies.Items, o.Namespace)
			if err != nil {
				return err
			}

		}
	} else if env == "systemd" {
		policies, err := getSystemdPolicies()
		if err != nil {
			return err
		}
		switch format := o.Output; format {
		case "json":
			result, err := json.Marshal(policies)
			if err != nil {
				return err
			}
			fmt.Println(string(result))
			return nil
		default:
			err := printSystemdPolices(policies)
			if err != nil {
				return err
			}
		}

	}
	return nil
}

func printK8SPolicies(policies []v1.KubeArmorPolicy, namespace string) error {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Policy Name", "Namespace", "Pods"})
	data := [][]string{}

	if len(policies) <= 0 {
		fmt.Printf("No policies found in %s", namespace)
		return nil
	}
	for _, policy := range policies {
		name := policy.ObjectMeta.Name
		ns := policy.ObjectMeta.Namespace
		pods := policy.Spec.Selector.MatchLabels["container"]
		data = append(data, []string{name, ns, pods})
	}

	for _, pol := range data {
		table.Append(pol)
	}
	table.Render()
	return nil
}

func printSystemdPolices(policies []HostPolicyBackup) error {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Policy Name", "Host"})
	data := [][]string{}
	for _, policy := range policies {
		name := policy.Metadata.PolicyName
		host := policy.Spec.NodeSelector.MatchLabels.KubernetesIoHostname
		data = append(data, []string{name, host})
	}
	for _, pol := range data {
		table.Append(pol)
	}
	table.Render()
	return nil
}

func getSystemdPolicies() ([]HostPolicyBackup, error) {
	files, err := ioutil.ReadDir("/opt/kubearmor/policies")
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return nil, errors.New("unable to read policy backups. Try running with elevated priviledges")
		}
		return []HostPolicyBackup{}, err
	}
	if len(files) <= 0 {
		fmt.Printf("Unable find host polices")
		return nil, err
	}
	policies := []HostPolicyBackup{}
	for _, file := range files {
		path := filepath.Join("/opt/kubearmor/policies/%s", filepath.Clean(file.Name()))
		fileBytes, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		policy := HostPolicyBackup{}
		err = json.Unmarshal(fileBytes, &policy)
		if err != nil {
			return nil, err
		}
		policies = append(policies, policy)
	}
	return policies, nil
}

func CheckEnv(c *k8s.Client) (string, error) {
	// deployment not found check systemd
	_, err := os.Stat("/usr/lib/systemd/system/kubearmor.service")
	if err == nil {
		return "systemd", nil
	}
	// check if kamor is running in K8s
	_, err = c.K8sClientset.AppsV1().Deployments("kube-system").Get(context.TODO(), "kubearmor-policy-manager", metav1.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) {
			return "", errors.New("unable to find kubearmor in cluster. Try running karmor install")
		}
		return "", err
	}

	if errors.Is(err, os.ErrNotExist) {
		return "", errors.New("unable to detect kubearmor installation")
	}
	return "k8s", nil
}
