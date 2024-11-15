package profileclient

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/evertras/bubble-table/table"
	pb "github.com/kubearmor/KubeArmor/protobuf"
)

func generateRowFromLog(entry pb.Log) table.Row {
	logType := "Container"
	if entry.Type == "HostLog" {
		logType = "Host"
		entry.NamespaceName = "--"
		entry.ContainerName = "--"
	}

	p := Profile{
		LogSource:     logType,
		Namespace:     entry.NamespaceName,
		ContainerName: entry.ContainerName,
		Process:       entry.ProcessName,
		Resource:      entry.Resource,
		Result:        entry.Result,
	}

	if entry.Operation == "Syscall" {
		p.Resource = entry.Data
	}

	row := table.NewRow(table.RowData{
		ColumnLogSource:     p.LogSource,
		ColumnNamespace:     p.Namespace,
		ColumnContainerName: p.ContainerName,
		ColumnProcessName:   p.Process,
		ColumnResource:      p.Resource,
		ColumnResult:        p.Result,
		ColumnCount:         1,
		ColumnTimestamp:     entry.UpdatedTime,
	})

	return row
}

func isCorrectLog(entry pb.Log) bool {
	if (ProfileOpts.Namespace != "") && (entry.NamespaceName != ProfileOpts.Namespace) {
		return false
	}
	if (ProfileOpts.Pod != "") && (entry.PodName != ProfileOpts.Pod) {
		return false
	}
	if (ProfileOpts.Container != "") && (entry.ContainerName != ProfileOpts.Container) {
		return false
	}

	return true
}

func ExportRowsToJSON(columns []table.Column, rows []table.Row, operation string) (string, error) {
	out := make([]map[string]interface{}, len(rows))

	for i, row := range rows {
		rowMap := make(map[string]interface{}, len(columns))
		for _, col := range columns {
			key := col.Key()
			if val, ok := row.Data[key]; ok {
				rowMap[key] = val
			} else {
				rowMap[key] = nil
			}
		}
		out[i] = rowMap
	}

	jsonBytes, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Create ProfileSummary directory if it doesn't exist
	outputDir := filepath.Join(cwd, "ProfileSummary")
	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Define the output file path
	fileName := fmt.Sprintf("%s.json", operation)
	filePath := filepath.Join(outputDir, fileName)

	// Write JSON to file
	if err := os.WriteFile(filePath, jsonBytes, 0o600); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return filePath, nil
}

func makeKeyFromEntry(e pb.Log) string {
	return fmt.Sprintf("%s|%s|%s|%s", e.NamespaceName, e.ContainerName, e.ProcessName, e.Operation)
}
