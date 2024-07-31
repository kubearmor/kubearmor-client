package probe

import (
	"fmt"
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

func (o *Options) getPrintableString(c *color.Color, s string) string {
	if o.Output == "nocolor-text" || c == nil {
		return s
	} else {
		return c.SprintFunc()(s)
	}
}

func (o *Options) printToOutput(c *color.Color, s string) {
	if o.Output == "nocolor-text" || c == nil {
		_, err := fmt.Fprint(os.Stdout, s)
		if err != nil {
			_, printErr := red.Printf(" error while printing to os.Stdout %s ", err.Error())
			if printErr != nil {
				fmt.Printf("Printing error")
			}
		}
	} else {
		_, err := c.Fprintf(os.Stdout, s)
		if err != nil {
			_, printErr := red.Printf("Can't print to os.Stdout")
			if printErr != nil {
				fmt.Printf("Printing error")
			}
		}
	}
}

// printDaemonsetData function
func (o *Options) printDaemonsetData(daemonsetStatus *Status) {
	var data [][]string
	o.printToOutput(green, "\nFound KubeArmor running in Kubernetes\n\n")
	o.printToOutput(itwhite, "Daemonset :\n")
	data = append(data, []string{" ", "kubearmor ", "Desired: " + daemonsetStatus.Desired, "Ready: " + daemonsetStatus.Ready, "Available: " + daemonsetStatus.Available})
	renderOutputInTableWithNoBorders(data)
}

// printKubeArmorDeployments function
func (o *Options) printKubearmorDeployments(deploymentData map[string]*Status) {
	o.printToOutput(itwhite, "Deployments : \n")
	var data [][]string
	for depName, depStatus := range deploymentData {
		data = append(data, []string{" ", depName, "Desired: " + depStatus.Desired, "Ready: " + depStatus.Ready, "Available: " + depStatus.Available})
	}

	renderOutputInTableWithNoBorders(data)
}

// printKubeArmorContainers function
func (o *Options) printKubeArmorContainers(containerData map[string]*KubeArmorPodSpec) {
	var data [][]string

	o.printToOutput(itwhite, "Containers : \n")
	for name, spec := range containerData {

		data = append(data, []string{" ", name, "Running: " + spec.Running, "Image Version: " + spec.Image_Version})
	}
	renderOutputInTableWithNoBorders(data)
}

// printKubeArmorprobe function
func (o *Options) printKubeArmorprobe(probeData []KubeArmorProbeData) {

	for i, pd := range probeData {
		o.printToOutput(itwhite, "Node "+fmt.Sprint(i+1)+" : \n")
		o.printKubeArmorProbeOutput(pd)
	}

}

// printKubeArmorProbeOutput function
func (o *Options) printKubeArmorProbeOutput(kd KubeArmorProbeData) {
	var data [][]string
	data = append(data, []string{" ", "OS Image:", o.getPrintableString(green, kd.OSImage)})
	data = append(data, []string{" ", "Kernel Version:", o.getPrintableString(green, kd.KernelVersion)})
	data = append(data, []string{" ", "Kubelet Version:", o.getPrintableString(green, kd.KubeletVersion)})
	data = append(data, []string{" ", "Container Runtime:", o.getPrintableString(green, kd.ContainerRuntime)})
	data = append(data, []string{" ", "Active LSM:", o.getPrintableString(green, kd.ActiveLSM)})
	data = append(data, []string{" ", "Host Security:", o.getPrintableString(green, strconv.FormatBool(kd.HostSecurity))})
	data = append(data, []string{" ", "Container Security:", o.getPrintableString(green, strconv.FormatBool(kd.ContainerSecurity))})
	data = append(data, []string{" ", "Container Default Posture:", o.getPrintableString(green, kd.ContainerDefaultPosture.FileAction) + o.getPrintableString(itwhite, "(File)"), o.getPrintableString(green, kd.ContainerDefaultPosture.CapabilitiesAction) + o.getPrintableString(itwhite, "(Capabilities)"), o.getPrintableString(green, kd.ContainerDefaultPosture.NetworkAction) + o.getPrintableString(itwhite, "(Network)")})
	data = append(data, []string{" ", "Host Default Posture:", o.getPrintableString(green, kd.HostDefaultPosture.FileAction) + o.getPrintableString(itwhite, "(File)"), o.getPrintableString(green, kd.HostDefaultPosture.CapabilitiesAction) + o.getPrintableString(itwhite, "(Capabilities)"), o.getPrintableString(green, kd.HostDefaultPosture.NetworkAction) + o.getPrintableString(itwhite, "(Network)")})
	data = append(data, []string{" ", "Host Visibility:", o.getPrintableString(green, kd.HostVisibility)})
	renderOutputInTableWithNoBorders(data)
}

// printAnnotatedPods function
func (o *Options) printAnnotatedPods(podData [][]string) {

	o.printToOutput(itwhite, "Armored Up pods : \n")
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"NAMESPACE", "DEFAULT POSTURE", "VISIBILITY", "NAME", "POLICY"})
	for _, v := range podData {
		table.Append(v)
	}
	table.SetRowLine(true)
	table.SetAutoMergeCellsByColumnIndex([]int{0, 1, 2})
	table.Render()
}
func (o *Options) printContainersSystemd(podData [][]string) {
	o.printToOutput(boldWhite, "Armored Up Containers : \n")

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"CONTAINER NAME", "POLICY"})
	for _, v := range podData {
		table.Append(v)
	}
	table.SetRowLine(true)
	table.SetAutoMergeCellsByColumnIndex([]int{0, 1})
	table.Render()

}
func (o *Options) printHostPolicy(hostPolicy [][]string) {
	o.printToOutput(boldWhite, "Host Policies : \n")

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"HOST NAME ", "POLICY"})
	for _, v := range hostPolicy {
		table.Append(v)
	}
	table.SetRowLine(true)
	table.SetAutoMergeCellsByColumnIndex([]int{0, 1})
	table.Render()
}
