package utils

import pb "github.com/kubearmor/KubeArmor/protobuf"

func GetArmoredContainerData(containerList []string, containerMap map[string]*pb.ContainerData) ([][]string, map[string][]string) {
	var data [][]string
	for _, containerName := range containerList {
		if _, ok := containerMap[containerName]; ok {
			if containerMap[containerName].PolicyEnabled == 1 {
				for _, policyName := range containerMap[containerName].PolicyList {
					data = append(data, []string{containerName, policyName})
				}
			}
		} else {
			data = append(data, []string{containerName, ""})
		}
	}
	mp := make(map[string][]string)
	for _, v := range data {
		if val, exists := mp[v[0]]; exists {
			val = append(val, v[1])
			mp[v[0]] = val
		} else {
			mp[v[0]] = []string{v[1]}
		}
	}
	return data, mp
}
func GetHostPolicyData(policyData *pb.ProbeResponse) [][]string {
	var data [][]string
	for k, v := range policyData.HostMap {
		for _, policy := range v.PolicyList {
			data = append(data, []string{k, policy})
		}
	}
	return data
}
