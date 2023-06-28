package probe

import (
	"os"
	"strconv"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
)

func renderOutputInTableWithNoBorders(data [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t") // pad with tabs
	table.SetNoWhiteSpace(true)
	table.AppendBulk(data) // Add Bulk Data
	table.Render()
}

// printDaemonsetData function
func printDaemonsetData(daemonsetStatus *Status) {
	var data [][]string

	color.Green("\nFound KubeArmor running in Kubernetes\n\n")
	_, err := boldWhite.Printf("Daemonset :\n")
	if err != nil {
		color.Red(" Error while printing")
	}
	data = append(data, []string{" ", "kubearmor ", "Desired: " + daemonsetStatus.Desired, "Ready: " + daemonsetStatus.Ready, "Available: " + daemonsetStatus.Available})
	renderOutputInTableWithNoBorders(data)
}

// printKubeArmorDeployments function
func printKubearmorDeployments(deploymentData map[string]*Status) {

	_, err := boldWhite.Printf("Deployments : \n")
	if err != nil {
		color.Red(" Error while printing")
	}
	var data [][]string
	for depName, depStatus := range deploymentData {
		data = append(data, []string{" ", depName, "Desired: " + depStatus.Desired, "Ready: " + depStatus.Ready, "Available: " + depStatus.Available})
	}

	renderOutputInTableWithNoBorders(data)
}

// printKubeArmorContainers function
func printKubeArmorContainers(containerData map[string]*KubeArmorPodSpec) {
	var data [][]string

	_, err := boldWhite.Printf("Containers : \n")
	if err != nil {
		color.Red(" Error while printing")
	}
	for name, spec := range containerData {

		data = append(data, []string{" ", name, "Running: " + spec.Running, "Image Version: " + spec.Image_Version})
	}
	renderOutputInTableWithNoBorders(data)
}

// printKubeArmorprobe function
func printKubeArmorprobe(probeData []KubeArmorProbeData) {

	for i, pd := range probeData {
		_, err := boldWhite.Printf("Node %d : \n", i+1)
		if err != nil {
			color.Red(" Error")
		}
		printKubeArmorProbeOutput(pd)
	}

}

// printKubeArmorProbeOutput function
func printKubeArmorProbeOutput(kd KubeArmorProbeData) {
	var data [][]string
	data = append(data, []string{" ", "OS Image:", green(kd.OSImage)})
	data = append(data, []string{" ", "Kernel Version:", green(kd.KernelVersion)})
	data = append(data, []string{" ", "Kubelet Version:", green(kd.KubeletVersion)})
	data = append(data, []string{" ", "Container Runtime:", green(kd.ContainerRuntime)})
	data = append(data, []string{" ", "Active LSM:", green(kd.ActiveLSM)})
	data = append(data, []string{" ", "Host Security:", green(strconv.FormatBool(kd.HostSecurity))})
	data = append(data, []string{" ", "Container Security:", green(strconv.FormatBool(kd.ContainerSecurity))})
	data = append(data, []string{" ", "Container Default Posture:", green(kd.ContainerDefaultPosture.FileAction) + itwhite("(File)"), green(kd.ContainerDefaultPosture.CapabilitiesAction) + itwhite("(Capabilities)"), green(kd.ContainerDefaultPosture.NetworkAction) + itwhite("(Network)")})
	data = append(data, []string{" ", "Host Default Posture:", green(kd.HostDefaultPosture.FileAction) + itwhite("(File)"), green(kd.HostDefaultPosture.CapabilitiesAction) + itwhite("(Capabilities)"), green(kd.HostDefaultPosture.NetworkAction) + itwhite("(Network)")})
	data = append(data, []string{" ", "Host Visibility:", green(kd.HostVisibility)})
	renderOutputInTableWithNoBorders(data)
}

// printAnnotatedPods function
func printAnnotatedPods(podData [][]string) {

	_, err := boldWhite.Printf("Armored Up pods : \n")
	if err != nil {
		color.Red(" Error printing bold text")
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"NAMESPACE", "DEFAULT POSTURE", "VISIBILITY", "NAME", "POLICY"})
	for _, v := range podData {
		table.Append(v)
	}
	table.SetRowLine(true)
	table.SetAutoMergeCellsByColumnIndex([]int{0, 1, 2})
	table.Render()
}
