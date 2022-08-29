package summary

import (
	"fmt"
	"net"
	"os"
	"strings"

	opb "github.com/accuknox/auto-policy-discovery/src/protobuf/v1/observability"

	"github.com/olekukonko/tablewriter"
)

var (
	SysProcHeader = []string{"Src Process", "Destination Process Path", "Count", "Last Updated Time", "Status"}
	SysFileHeader = []string{"Src Process", "Destination File Path", "Count", "Last Updated Time", "Status"}
	SysNwHeader   = []string{"Protocol", "Command", "POD/SVC/IP", "Port", "Namespace", "Labels"}
)

func DisplaySummaryOutput(resp *opb.Response, revDNSLookup bool) {

	podInfo := resp.PodName + "/" + resp.Namespace + "/" + resp.ClusterName + "/" + resp.Label + "/" + resp.ContainerName

	fmt.Printf("\nPodInfo : [%s]\n", podInfo)

	if len(resp.ProcessData) > 0 {

		procRowData := [][]string{}
		// Display process data
		fmt.Printf("\nProcess Data\n")
		for _, procData := range resp.ProcessData {
			procStrSlice := []string{}
			procStrSlice = append(procStrSlice, procData.ParentProcName)
			procStrSlice = append(procStrSlice, procData.ProcName)
			procStrSlice = append(procStrSlice, procData.Count)
			procStrSlice = append(procStrSlice, procData.UpdatedTime)
			procStrSlice = append(procStrSlice, procData.Status)
			procRowData = append(procRowData, procStrSlice)
		}
		WriteTable(SysProcHeader, procRowData)
		fmt.Printf("\n")
	}

	if len(resp.FileData) > 0 {
		fmt.Printf("\nFile Data\n")
		// Display file data
		fileRowData := [][]string{}
		for _, fileData := range resp.FileData {
			fileStrSlice := []string{}
			fileStrSlice = append(fileStrSlice, fileData.ParentProcName)
			fileStrSlice = append(fileStrSlice, fileData.ProcName)
			fileStrSlice = append(fileStrSlice, fileData.Count)
			fileStrSlice = append(fileStrSlice, fileData.UpdatedTime)
			fileStrSlice = append(fileStrSlice, fileData.Status)
			fileRowData = append(fileRowData, fileStrSlice)
		}
		WriteTable(SysFileHeader, fileRowData)
		fmt.Printf("\n")
	}

	if len(resp.InNwData) > 0 {
		fmt.Printf("\nIncoming Server connection\n")
		// Display server conn data
		inNwRowData := [][]string{}
		for _, inNwData := range resp.InNwData {
			inNwStrSlice := []string{}
			domainName := dnsLookup(inNwData.IP, revDNSLookup)
			inNwStrSlice = append(inNwStrSlice, inNwData.Protocol)
			inNwStrSlice = append(inNwStrSlice, inNwData.Command)
			inNwStrSlice = append(inNwStrSlice, domainName)
			inNwStrSlice = append(inNwStrSlice, inNwData.Port)
			inNwStrSlice = append(inNwStrSlice, inNwData.Namespace)
			inNwStrSlice = append(inNwStrSlice, inNwData.Labels)
			inNwRowData = append(inNwRowData, inNwStrSlice)
		}
		WriteTable(SysNwHeader, inNwRowData)
		fmt.Printf("\n")
	}

	if len(resp.OutNwData) > 0 {
		fmt.Printf("\nOutgoing Server connection\n")
		// Display server conn data
		outNwRowData := [][]string{}
		for _, outNwData := range resp.OutNwData {
			outNwStrSlice := []string{}
			domainName := dnsLookup(outNwData.IP, revDNSLookup)
			outNwStrSlice = append(outNwStrSlice, outNwData.Protocol)
			outNwStrSlice = append(outNwStrSlice, outNwData.Command)
			outNwStrSlice = append(outNwStrSlice, domainName)
			outNwStrSlice = append(outNwStrSlice, outNwData.Port)
			outNwStrSlice = append(outNwStrSlice, outNwData.Namespace)
			outNwStrSlice = append(outNwStrSlice, outNwData.Labels)
			outNwRowData = append(outNwRowData, outNwStrSlice)
		}
		WriteTable(SysNwHeader, outNwRowData)
		fmt.Printf("\n")
	}
}

func dnsLookup(ip string, revDNSLookup bool) string {
	if revDNSLookup {
		if strings.Contains(ip, "svc") || strings.Contains(ip, "pod") {
			return ip
		}
		dns, err := net.LookupAddr(ip)
		if err != nil {
			return ip
		}
		if dns[0] != "" {
			return dns[0]
		}
	}
	return ip
}

func WriteTable(header []string, data [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	for _, v := range data {
		table.Append(v)
	}
	table.Render()
}
