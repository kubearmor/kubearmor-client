// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package observe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	proto "github.com/kubearmor/koach/protobuf"
	"github.com/rodaine/table"
	"google.golang.org/grpc"
)

// Options Structure
type TelemetryOptions struct {
	Namespace     string
	AllNamespace  bool
	Labels        string
	ShowLabels    bool
	Since         string
	CustomColumns string
	GRPC          string
}

// Get observability data
func StartObserveTelemetry(args []string, options TelemetryOptions) error {
	gRPC := "localhost:3001"

	if options.GRPC != "" {
		gRPC = options.GRPC
	}

	conn, err := grpc.Dial(gRPC, grpc.WithInsecure())
	if err != nil {
		return errors.New("could not connect to the server. Possible troubleshooting:\n- Check if koach server is running\n- Create a portforward to koach service using\n\t\033[1mkubectl port-forward -n kube-system service/koach --address 0.0.0.0 --address :: 3001:3001\033[0m\n- Configure grpc server information using\n\t\033[1mkarmor observe --grpc <info>\033[0m")
	}

	client := proto.NewObservabilityServiceClient(conn)

	operations := map[string]string{
		"file":    "File",
		"network": "Network",
		"process": "Process",
		"syscall": "Syscall",
	}

	operation, found := operations[args[0]]
	if !found {
		return errors.New("invalid operation to observe, valid operations are [file|network|process|syscall|alert]")
	}

	req := &proto.GetRequest{
		Labels:        options.Labels,
		Time:          options.Since,
		OperationType: operation,
	}

	if options.AllNamespace {
		req.NamespaceId = ""
	} else {
		req.NamespaceId = options.Namespace
	}

	res, err := client.Get(context.Background(), req)
	if err != nil {
		return errors.New("could not connect to the server. Possible troubleshooting:\n- Check if koach server is running\n- Create a portforward to koach service using\n\t\033[1mkubectl port-forward -n kube-system service/koach --address 0.0.0.0 --address :: 3001:3001\033[0m\n- Configure grpc server information using\n\t\033[1mkarmor observe --grpc <info>\033[0m")
	}

	if len(res.Data) == 0 {
		fmt.Println("No observability data found")
		return nil
	}

	if len(res.Data) == 0 {
		fmt.Println("No observability data found")
		return nil
	}

	var tbl table.Table

	if options.CustomColumns == "" {
		columns := []interface{}{"POD", "RESOURCE"}

		if options.ShowLabels {
			columns = append(columns, "LABELS")
		}

		columns = append(columns, "TIMESTAMP")

		tbl = table.New(columns...)

		headerFmt := color.New(color.FgWhite).SprintfFunc()
		tbl.WithHeaderFormatter(headerFmt)

		for _, observability := range res.Data {
			timestamp, _ := time.Parse("2006-01-02 15:04:05.999999 -0700 MST", observability.CreatedAt)
			timestampStr := timestamp.Format("2006-01-02 15:04:05")

			row := []interface{}{observability.PodName, observability.Resource}

			if options.ShowLabels {
				row = append(row, observability.Labels)
			}

			row = append(row, timestampStr)

			tbl.AddRow(row...)
		}

	} else {
		customColumnsSplit := strings.Split(options.CustomColumns, ",")

		columns := []interface{}{}
		keys := []string{}

		for _, customColumn := range customColumnsSplit {
			customColumnSplit := strings.Split(customColumn, ":")

			columns = append(columns, customColumnSplit[0])
			keys = append(keys, strings.Split(customColumnSplit[1], ".")[1])
		}

		tbl = table.New(columns...)

		headerFmt := color.New(color.FgWhite).SprintfFunc()
		tbl.WithHeaderFormatter(headerFmt)

		for _, observability := range res.Data {
			row := []interface{}{}

			var observabilityMap map[string]interface{}
			observabilityJSON, _ := json.Marshal(observability)
			json.Unmarshal(observabilityJSON, &observabilityMap)

			for _, key := range keys {
				value, ok := observabilityMap[key]
				if !ok {
					fmt.Println("Valid key for custom columns:")
					for key := range observabilityMap {
						fmt.Printf("  %s\n", key)
					}
					return fmt.Errorf("unrecognized key in custom columns")
				}

				row = append(row, value)
			}

			tbl.AddRow(row...)
		}
	}

	tbl.Print()

	return nil
}
