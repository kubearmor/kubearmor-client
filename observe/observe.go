// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package observe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	proto "github.com/kubearmor/koach/protobuf"
	"github.com/rodaine/table"
	"google.golang.org/grpc"
)

// Options Structure
type Options struct {
	Operation     string
	Namespace     string
	AllNamespace  bool
	Labels        string
	ShowLabels    bool
	Since         string
	CustomColumns string
}

// Get observability data
func StartObserve(args []string, options Options) error {
	conn, err := grpc.Dial("localhost:3001", grpc.WithInsecure())
	if err != nil {
		panic(err)
	}

	client := proto.NewObservabilityServiceClient(conn)

	req := &proto.GetRequest{
		OperationType: options.Operation,
		Labels:        options.Labels,
		Time:          options.Since,
	}

	if options.AllNamespace {
		req.NamespaceId = ""
	} else {
		req.NamespaceId = options.Namespace
	}

	if len(args) >= 2 {
		resourceType := args[0]
		resourceName := args[1]

		switch resourceType {
		case "pod":
		case "pods":
			req.PodId = resourceName
		}
	}

	res, err := client.Get(context.Background(), req)
	if err != nil {
		return err
	}

	var tbl table.Table

	if options.CustomColumns == "" {
		columns := []interface{}{"POD", "RESOURCE"}

		if options.ShowLabels {
			columns = append(columns, "LABELS")
		}

		columns = append(columns, "TIMESTAMP")

		tbl = table.New(columns...)

		headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()
		tbl.WithHeaderFormatter(headerFmt)

		if len(res.Data) == 0 {
			fmt.Println("No observability data found")
			return nil
		}

		for _, observability := range res.Data {
			timestamp, _ := time.Parse("2006-01-02 15:04:05.999999 Z0700 Z0700", observability.CreatedAt)
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

		headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()
		tbl.WithHeaderFormatter(headerFmt)

		for _, observability := range res.Data {
			row := []interface{}{}

			var observabilityMap map[string]interface{}
			observabilityJSON, _ := json.Marshal(observability)
			json.Unmarshal(observabilityJSON, &observabilityMap)

			for _, key := range keys {
				value, ok := observabilityMap[key]
				if !ok {
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
