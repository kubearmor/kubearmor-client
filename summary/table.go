package summary

import (
	"fmt"
	"net"
	"os"
	"sort"
	"strings"

	opb "github.com/accuknox/auto-policy-discovery/src/protobuf/v1/observability"
	"github.com/mgutz/ansi"

	"github.com/olekukonko/tablewriter"
)

var (
	// SysProcHeader variable contains source process, destination process path, count, timestamp and status
	SysProcHeader = []string{"Src Process", "Destination Process Path", "Count", "Last Updated Time", "Status"}
	// SysFileHeader variable contains source process, destination file path, count, timestamp and status
	SysFileHeader = []string{"Src Process", "Destination File Path", "Count", "Last Updated Time", "Status"}
	// SysNwHeader variable contains protocol, command, POD/SVC/IP, Port, Namespace, and Labels
	SysNwHeader = []string{"Protocol", "Command", "POD/SVC/IP", "Port", "Namespace", "Labels", "Count", "Last Updated Time"}
)

// DisplaySummaryOutput function
func DisplaySummaryOutput(resp *opb.Response, revDNSLookup bool, requestType string) {

	if len(resp.ProcessData) <= 0 && len(resp.FileData) <= 0 && len(resp.InNwData) <= 0 && len(resp.OutNwData) <= 0 {
		return
	}

	writePodInfoToTable(resp.PodName, resp.Namespace, resp.ClusterName, resp.ContainerName, resp.Label)

	// Colored Status for Allow and Deny
	agc := ansi.ColorFunc("green")
	arc := ansi.ColorFunc("red")

	if strings.Contains(requestType, "process") {
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
				if procData.Status == "Allow" {
					procStrSlice = append(procStrSlice, agc(procData.Status))
				} else if procData.Status == "Deny" {
					procStrSlice = append(procStrSlice, arc(procData.Status))
				}
				procRowData = append(procRowData, procStrSlice)
			}
			sort.Slice(procRowData[:], func(i, j int) bool {
				for x := range procRowData[i] {
					if procRowData[i][x] == procRowData[j][x] {
						continue
					}
					return procRowData[i][x] < procRowData[j][x]
				}
				return false
			})
			WriteTable(SysProcHeader, procRowData)
			fmt.Printf("\n")
		}
	}

	if strings.Contains(requestType, "file") {
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
				if fileData.Status == "Allow" {
					fileStrSlice = append(fileStrSlice, agc(fileData.Status))
				} else if fileData.Status == "Deny" {
					fileStrSlice = append(fileStrSlice, arc(fileData.Status))
				}
				fileRowData = append(fileRowData, fileStrSlice)
			}
			sort.Slice(fileRowData[:], func(i, j int) bool {
				for x := range fileRowData[i] {
					if fileRowData[i][x] == fileRowData[j][x] {
						continue
					}
					return fileRowData[i][x] < fileRowData[j][x]
				}
				return false
			})
			WriteTable(SysFileHeader, fileRowData)
			fmt.Printf("\n")
		}
	}

	if strings.Contains(requestType, "network") {
		if len(resp.InNwData) > 0 {
			fmt.Printf("\nIngress connections\n")
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
				inNwStrSlice = append(inNwStrSlice, inNwData.Count)
				inNwStrSlice = append(inNwStrSlice, inNwData.UpdatedTime)
				inNwRowData = append(inNwRowData, inNwStrSlice)
			}
			WriteTable(SysNwHeader, inNwRowData)
			fmt.Printf("\n")
		}

		if len(resp.OutNwData) > 0 {
			fmt.Printf("\nEgress connections\n")
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
				outNwStrSlice = append(outNwStrSlice, outNwData.Count)
				outNwStrSlice = append(outNwStrSlice, outNwData.UpdatedTime)
				outNwRowData = append(outNwRowData, outNwStrSlice)
			}
			WriteTable(SysNwHeader, outNwRowData)
			fmt.Printf("\n")
		}
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

// WriteTable function
func WriteTable(header []string, data [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	for _, v := range data {
		table.Append(v)
	}
	table.Render()
}

func writePodInfoToTable(podname, namespace, clustername, containername, labels string) {

	fmt.Printf("\n")

	podinfo := [][]string{
		{"Pod Name", podname},
		{"Namespace Name", namespace},
		{"Cluster Name", clustername},
		{"Container Name", containername},
		{"Labels", labels},
	}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetBorder(false)
	table.SetTablePadding("\t")
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	for _, v := range podinfo {
		table.Append(v)
	}
	table.Render()
}
