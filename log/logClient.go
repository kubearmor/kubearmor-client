// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package log

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	pb "github.com/kubearmor/KubeArmor/protobuf"
	"google.golang.org/grpc"
)

var Limitchan chan bool
var i uint32

// ============ //
// == Common == //
// ============ //

// StrToFile Function
func StrToFile(str, destFile string) {
	if _, err := os.Stat(destFile); err != nil {
		newFile, err := os.Create(filepath.Clean(destFile))
		if err != nil {
			fmt.Printf("Failed to create a file (%s, %s)\n", destFile, err.Error())
			return
		}
		if err := newFile.Close(); err != nil {
			fmt.Printf("Failed to close the file (%s, %s)\n", destFile, err.Error())
		}
	}

	// #nosec
	file, err := os.OpenFile(destFile, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Printf("Failed to open a file (%s, %s)\n", destFile, err.Error())
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Failed to close the file (%s, %s)\n", destFile, err.Error())
		}
	}()

	_, err = file.WriteString(str)
	if err != nil {
		fmt.Printf("Failed to write a string into the file (%s, %s)\n", destFile, err.Error())
	}
}

// =============== //
// == Log Feeds == //
// =============== //

// Feeder Structure
type Feeder struct {
	// flag
	Running bool

	// server
	server string

	//limit
	limit uint32

	// connection
	conn *grpc.ClientConn

	// client
	client pb.LogServiceClient

	// messages
	msgStream pb.LogService_WatchMessagesClient

	// alerts
	alertStream pb.LogService_WatchAlertsClient

	// logs
	logStream pb.LogService_WatchLogsClient

	// wait group
	WgClient sync.WaitGroup
}

// NewClient Function
func NewClient(server, msgPath, logPath, logFilter string, limit uint32) *Feeder {
	fd := &Feeder{}

	fd.Running = true

	fd.server = server

	fd.limit = limit

	conn, err := grpc.Dial(fd.server, grpc.WithInsecure())
	if err != nil {
		return nil
	}
	fd.conn = conn

	fd.client = pb.NewLogServiceClient(fd.conn)

	msgIn := pb.RequestMessage{}
	msgIn.Filter = ""

	if msgPath != "none" {
		msgStream, err := fd.client.WatchMessages(context.Background(), &msgIn)
		if err != nil {
			return nil
		}
		fd.msgStream = msgStream
	}

	alertIn := pb.RequestMessage{}
	alertIn.Filter = logFilter

	if logPath != "none" && (alertIn.Filter == "all" || alertIn.Filter == "policy") {
		alertStream, err := fd.client.WatchAlerts(context.Background(), &alertIn)
		if err != nil {
			return nil
		}
		fd.alertStream = alertStream
	}

	logIn := pb.RequestMessage{}
	logIn.Filter = logFilter

	if logPath != "none" && (logIn.Filter == "all" || logIn.Filter == "system") {
		logStream, err := fd.client.WatchLogs(context.Background(), &logIn)
		if err != nil {
			return nil
		}
		fd.logStream = logStream
	}

	fd.WgClient = sync.WaitGroup{}

	return fd
}

// DoHealthCheck Function
func (fd *Feeder) DoHealthCheck() bool {
	// #nosec
	randNum := rand.Int31()

	// send a nonce
	nonce := pb.NonceMessage{Nonce: randNum}
	res, err := fd.client.HealthCheck(context.Background(), &nonce)
	if err != nil {
		return false
	}

	// check nonce
	if randNum != res.Retval {
		return false
	}

	return true
}

// WatchMessages Function
func (fd *Feeder) WatchMessages(msgPath string, jsonFormat bool) error {
	fd.WgClient.Add(1)
	defer fd.WgClient.Done()

	for fd.Running {
		res, err := fd.msgStream.Recv()
		if err != nil {
			fmt.Printf("Failed to receive a message (%s)\n", err.Error())
			break
		}

		str := ""

		if jsonFormat {
			arr, _ := json.Marshal(res)
			str = fmt.Sprintf("%s\n", string(arr))
		} else {
			updatedTime := strings.Replace(res.UpdatedTime, "T", " ", -1)
			updatedTime = strings.Replace(updatedTime, "Z", "", -1)

			str = fmt.Sprintf("%s  %s  %s  [%s]  %s\n", updatedTime, res.ClusterName, res.HostName, res.Level, res.Message)
		}

		if msgPath == "stdout" {
			fmt.Printf("%s", str)
		} else {
			StrToFile(str, msgPath)
		}
	}

	fmt.Println("Stopped WatchMessages")

	return nil
}

func regexMatcher(filter *regexp.Regexp, res string) bool {

	match := filter.MatchString(res)
	if !match {
		return false
	}
	return true
}

func watchAlertsHelper(res *pb.Alert, o Options) error {
	if o.Namespace != "" {
		match := regexMatcher(CNamespace, res.NamespaceName)
		if !match {
			return nil
		}
	}

	if o.LogType != "" {
		match := regexMatcher(CLogtype, res.Type)
		if !match {
			return nil
		}
	}

	if o.Operation != "" {
		match := regexMatcher(COperation, res.Operation)
		if !match {
			return nil
		}
	}

	if o.ContainerName != "" {
		match := regexMatcher(CContainerName, res.ContainerName)
		if !match {
			return nil
		}
	}

	if o.PodName != "" {
		match := regexMatcher(CPodName, res.PodName)
		if !match {
			return nil
		}
	}

	if o.Source != "" {
		match := regexMatcher(CSource, res.Source)
		if !match {
			return nil
		}
	}

	if o.Resource != "" {
		match := regexMatcher(CResource, res.Resource)
		if !match {
			return nil
		}
	}

	str := ""

	if o.JSON {
		arr, _ := json.Marshal(res)
		str = fmt.Sprintf("%s\n", string(arr))
	} else {
		updatedTime := strings.Replace(res.UpdatedTime, "T", " ", -1)
		updatedTime = strings.Replace(updatedTime, "Z", "", -1)

		str = fmt.Sprintf("== Alert / %s ==\n", updatedTime)

		str = str + fmt.Sprintf("Cluster Name: %s\n", res.ClusterName)
		str = str + fmt.Sprintf("Host Name: %s\n", res.HostName)

		if res.NamespaceName != "" {
			str = str + fmt.Sprintf("Namespace Name: %s\n", res.NamespaceName)
			str = str + fmt.Sprintf("Pod Name: %s\n", res.PodName)
			str = str + fmt.Sprintf("Container ID: %s\n", res.ContainerID)
			str = str + fmt.Sprintf("Container Name: %s\n", res.ContainerName)
			str = str + fmt.Sprintf("Labels: %s\n", res.Labels)
		}

		if len(res.PolicyName) > 0 {
			str = str + fmt.Sprintf("Policy Name: %s\n", res.PolicyName)
		}

		if len(res.Severity) > 0 {
			str = str + fmt.Sprintf("Severity: %s\n", res.Severity)
		}

		if len(res.Tags) > 0 {
			str = str + fmt.Sprintf("Tags: %s\n", res.Tags)
		}

		if len(res.Message) > 0 {
			str = str + fmt.Sprintf("Message: %s\n", res.Message)
		}

		str = str + fmt.Sprintf("Type: %s\n", res.Type)
		str = str + fmt.Sprintf("Source: %s\n", res.Source)
		str = str + fmt.Sprintf("Operation: %s\n", res.Operation)
		str = str + fmt.Sprintf("Resource: %s\n", res.Resource)

		if len(res.Data) > 0 {
			str = str + fmt.Sprintf("Data: %s\n", res.Data)
		}

		if len(res.Action) > 0 {
			str = str + fmt.Sprintf("Action: %s\n", res.Action)
		}

		str = str + fmt.Sprintf("Result: %s\n", res.Result)
	}

	if o.LogPath == "stdout" {
		fmt.Printf("%s", str)
	} else {
		StrToFile(str, o.LogPath)
	}
	return nil
}

// WatchAlerts Function
func (fd *Feeder) WatchAlerts(o Options) error {
	fd.WgClient.Add(1)
	defer fd.WgClient.Done()

	if o.Limit > 0 {
		for i = 0; i < o.Limit; i++ {
			res, err := fd.alertStream.Recv()
			if err != nil {
				fmt.Printf("Failed to receive an alert (%s)\n", err.Error())
				break
			}
			_ = watchAlertsHelper(res, o)

		}
		Limitchan <- true

	} else {
		for fd.Running {
			res, err := fd.alertStream.Recv()
			if err != nil {
				fmt.Printf("Failed to receive an alert (%s)\n", err.Error())
				break
			}
			_ = watchAlertsHelper(res, o)

		}
	}

	fmt.Println("Stopped WatchAlerts")

	return nil
}

func WatchLogsHelper(res *pb.Log, o Options) error {
	if o.Namespace != "" {
		match := regexMatcher(CNamespace, res.NamespaceName)
		if !match {
			return nil
		}
	}

	if o.LogType != "" {
		match := regexMatcher(CLogtype, res.Type)
		if !match {
			return nil
		}
	}

	if o.Operation != "" {
		match := regexMatcher(COperation, res.Operation)
		if !match {
			return nil
		}
	}

	if o.ContainerName != "" {
		match := regexMatcher(CContainerName, res.ContainerName)
		if !match {
			return nil
		}
	}

	if o.PodName != "" {
		match := regexMatcher(CPodName, res.PodName)
		if !match {
			return nil
		}
	}

	if o.Source != "" {
		match := regexMatcher(CSource, res.Source)
		if !match {
			return nil
		}
	}

	if o.Resource != "" {
		match := regexMatcher(CResource, res.Resource)
		if !match {
			return nil
		}
	}

	str := ""

	if o.JSON {
		arr, _ := json.Marshal(res)
		str = fmt.Sprintf("%s\n", string(arr))
	} else {
		updatedTime := strings.Replace(res.UpdatedTime, "T", " ", -1)
		updatedTime = strings.Replace(updatedTime, "Z", "", -1)

		str = fmt.Sprintf("== Log / %s ==\n", updatedTime)

		str = str + fmt.Sprintf("Cluster Name: %s\n", res.ClusterName)
		str = str + fmt.Sprintf("Host Name: %s\n", res.HostName)

		if res.NamespaceName != "" {
			str = str + fmt.Sprintf("Namespace Name: %s\n", res.NamespaceName)
			str = str + fmt.Sprintf("Pod Name: %s\n", res.PodName)
			str = str + fmt.Sprintf("Container ID: %s\n", res.ContainerID)
			str = str + fmt.Sprintf("Container Name: %s\n", res.ContainerName)
			str = str + fmt.Sprintf("Labels: %s\n", res.Labels)
		}

		str = str + fmt.Sprintf("Type: %s\n", res.Type)
		str = str + fmt.Sprintf("Source: %s\n", res.Source)
		str = str + fmt.Sprintf("Operation: %s\n", res.Operation)
		str = str + fmt.Sprintf("Resource: %s\n", res.Resource)

		if len(res.Data) > 0 {
			str = str + fmt.Sprintf("Data: %s\n", res.Data)
		}

		str = str + fmt.Sprintf("Result: %s\n", res.Result)
	}

	if o.LogPath == "stdout" {
		fmt.Printf("%s", str)
	} else {
		StrToFile(str, o.LogPath)
	}
	return nil

}

// WatchLogs Function
func (fd *Feeder) WatchLogs(o Options) error {
	fd.WgClient.Add(1)
	defer fd.WgClient.Done()

	if o.Limit > 0 {
		for i = 0; i < o.Limit; i++ {
			res, err := fd.logStream.Recv()
			if err != nil {
				fmt.Printf("Failed to receive an alert (%s)\n", err.Error())
				break
			}
			_ = WatchLogsHelper(res, o)
		}
		Limitchan <- true
	} else {
		for fd.Running {
			res, err := fd.logStream.Recv()
			if err != nil {
				fmt.Printf("Failed to receive an alert (%s)\n", err.Error())
				break
			}
			_ = WatchLogsHelper(res, o)

		}
	}

	fmt.Println("Stopped WatchLogs")

	return nil
}

// DestroyClient Function
func (fd *Feeder) DestroyClient() error {
	if err := fd.conn.Close(); err != nil {
		return err
	}

	fd.WgClient.Wait()

	return nil
}
