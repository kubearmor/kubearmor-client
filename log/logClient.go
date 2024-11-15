// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	pb "github.com/kubearmor/KubeArmor/protobuf"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/client-go/kubernetes"
)

// EventInfo Event data signalled on EventChan
type EventInfo struct {
	Data []byte // json marshalled byte data for alert/log
	Type string // "Alert"/"Log"
}

// Limitchan handles telemetry event output limit
var (
	Limitchan chan bool
	i         uint32
)

// ============ //
// == Common == //
// ============ //

// StrToFile Function
func StrToFile(str, destFile string) {
	if _, err := os.Stat(destFile); err != nil {
		newFile, err := os.Create(filepath.Clean(destFile))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create a file (%s, %s)\n", destFile, err.Error())
			return
		}
		if err := newFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close the file (%s, %s)\n", destFile, err.Error())
		}
	}

	// #nosec
	file, err := os.OpenFile(destFile, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open a file (%s, %s)\n", destFile, err.Error())
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close the file (%s, %s)\n", destFile, err.Error())
		}
	}()

	_, err = file.WriteString(str)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write a string into the file (%s, %s)\n", destFile, err.Error())
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

	// limit
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
func NewClient(server string, o Options, c kubernetes.Interface) (*Feeder, error) {
	fd := &Feeder{}

	fd.Running = true

	fd.server = server

	fd.limit = o.Limit

	var creds credentials.TransportCredentials
	if o.Secure {
		tlsCreds, err := loadTLSCredentials(c, o)
		if err != nil {
			return nil, err
		}
		creds = tlsCreds
	} else {
		creds = insecure.NewCredentials()
	}
	conn, err := grpc.Dial(fd.server, grpc.WithTransportCredentials(creds))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error dialing the server: %s", err)
		return nil, err
	}
	fd.conn = conn

	fd.client = pb.NewLogServiceClient(fd.conn)

	msgIn := pb.RequestMessage{}
	msgIn.Filter = ""

	if o.MsgPath != "none" {
		msgStream, err := fd.client.WatchMessages(context.Background(), &msgIn)
		if err != nil {
			return nil, err
		}
		fd.msgStream = msgStream
	}

	alertIn := pb.RequestMessage{}
	alertIn.Filter = o.LogFilter

	if o.LogPath != "none" && (alertIn.Filter == "all" || alertIn.Filter == "policy") {
		alertStream, err := fd.client.WatchAlerts(context.Background(), &alertIn)
		if err != nil {
			return nil, err
		}
		fd.alertStream = alertStream
	}

	logIn := pb.RequestMessage{}
	logIn.Filter = o.LogFilter

	if o.LogPath != "none" && (logIn.Filter == "all" || logIn.Filter == "system") {
		logStream, err := fd.client.WatchLogs(context.Background(), &logIn)
		if err != nil {
			return nil, err
		}
		fd.logStream = logStream
	}

	fd.WgClient = sync.WaitGroup{}

	return fd, nil
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
			fmt.Fprintf(os.Stderr, "Failed to receive a message (%s)\n", err.Error())
			UnblockSignal = err
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
		} else if msgPath != "" {
			StrToFile(str, msgPath)
		}
	}

	fmt.Fprintln(os.Stderr, "Stopped WatchMessages")

	return nil
}

func regexMatcher(filter *regexp.Regexp, res string) bool {
	match := filter.MatchString(res)
	if !match {
		return false
	}
	return true
}

// WatchAlerts Function
func (fd *Feeder) WatchAlerts(o Options) error {
	fd.WgClient.Add(1)
	defer fd.WgClient.Done()

	if o.Limit > 0 {
		for i = 0; i < o.Limit; i++ {
			res, err := fd.alertStream.Recv()
			if err != nil {
				break
			}

			t, _ := json.Marshal(res)
			WatchTelemetryHelper(t, "Alert", o)

		}
		Limitchan <- true
		return nil
	}
	for fd.Running {
		res, err := fd.alertStream.Recv()
		if err != nil {
			UnblockSignal = err
			break
		}

		t, _ := json.Marshal(res)
		WatchTelemetryHelper(t, "Alert", o)

	}

	fmt.Fprintln(os.Stderr, "Stopped WatchAlerts")

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
				break
			}

			t, _ := json.Marshal(res)
			WatchTelemetryHelper(t, "Log", o)

		}
		Limitchan <- true
		return nil
	}
	for fd.Running {
		res, err := fd.logStream.Recv()
		if err != nil {
			UnblockSignal = err
			break
		}

		t, _ := json.Marshal(res)
		WatchTelemetryHelper(t, "Log", o)
	}

	fmt.Fprintln(os.Stderr, "Stopped WatchLogs")

	return nil
}

// WatchTelemetryHelper handles Alerts and Logs
func WatchTelemetryHelper(arr []byte, t string, o Options) {
	var res map[string]interface{}
	err := json.Unmarshal(arr, &res)
	if err != nil {
		return
	}
	// Filter Telemetry based on provided options
	if len(o.Selector) != 0 && res["Labels"] != nil {
		labels := strings.Split(res["Labels"].(string), ",")
		val := selectLabels(o, labels)
		if val != nil {
			return
		}
	}

	if o.Namespace != "" {
		ns, ok := res["NamespaceName"].(string)
		if !ok {
			return
		}
		match := regexMatcher(CNamespace, ns)
		if !match {
			return
		}
	}

	if o.LogType != "" {
		t, ok := res["Type"].(string)
		if !ok {
			return
		}
		match := regexMatcher(CLogtype, t)
		if !match {
			return
		}
	}

	if o.Operation != "" {
		op, ok := res["Operation"].(string)
		if !ok {
			return
		}
		match := regexMatcher(COperation, op)
		if !match {
			return
		}
	}

	if o.ContainerName != "" {
		cn, ok := res["ContainerName"].(string)
		if !ok {
			return
		}
		match := regexMatcher(CContainerName, cn)
		if !match {
			return
		}
	}

	if o.PodName != "" {
		pn, ok := res["PodName"].(string)
		if !ok {
			return
		}
		match := regexMatcher(CPodName, pn)
		if !match {
			return
		}
	}

	if o.Source != "" {
		src, ok := res["Source"].(string)
		if !ok {
			return
		}
		match := regexMatcher(CSource, src)
		if !match {
			return
		}
	}

	if o.Resource != "" {
		rs, ok := res["Resource"].(string)
		if !ok {
			return
		}
		match := regexMatcher(CResource, rs)
		if !match {
			return
		}
	}

	str := ""

	// Pass Events to Channel for further handling
	if o.EventChan != nil {
		o.EventChan <- EventInfo{Data: arr, Type: t}
	}

	if o.JSON || o.Output == "json" {
		str = fmt.Sprintf("%s\n", string(arr))
	} else if o.Output == "pretty-json" {

		var prettyJSON bytes.Buffer
		err = json.Indent(&prettyJSON, arr, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to prettify JSON (%s)\n", err.Error())
		}
		str = fmt.Sprintf("%s\n", prettyJSON.String())

	} else {

		if time, ok := res["UpdatedTime"]; ok {
			updatedTime := strings.Replace(time.(string), "T", " ", -1)
			updatedTime = strings.Replace(updatedTime, "Z", "", -1)
			str = fmt.Sprintf("== %s / %s ==\n", t, updatedTime)
		} else {
			str = fmt.Sprintf("== %s ==\n", t)
		}

		// Array of Keys to preserve order in Output
		telKeys := []string{
			"UpdatedTime",
			"Timestamp",
			"ClusterName",
			"HostName",
			"NamespaceName",
			"PodName",
			"Labels",
			"ContainerName",
			"ContainerID",
			"ContainerImage",
			"Type",
			"PolicyName",
			"Severity",
			"Message",
			"Source",
			"Resource",
			"Operation",
			"Action",
			"Data",
			"Enforcer",
			"Result",
		}

		var additionalKeys []string
		// Looping through the Map to find additional keys not present in our array
		for k := range res {
			if !slices.Contains(telKeys, k) {
				additionalKeys = append(additionalKeys, k)
			}
		}
		sort.Strings(additionalKeys)
		telKeys = append(telKeys, additionalKeys...)

		for i := 2; i < len(telKeys); i++ { // Starting the loop from index 2 to skip printing timestamp again
			k := telKeys[i]
			// Check if fields are present in the structure and if present verifying that they are not empty
			// Certain fields like Container* are not present in HostLogs, this check handles that and other edge cases
			if v, ok := res[k]; ok && v != "" {
				if _, ok := res[k].(float64); ok {
					str = str + fmt.Sprintf("%s: %.0f\n", k, res[k])
				} else {
					str = str + fmt.Sprintf("%s: %v\n", k, res[k])
				}
			}
		}
	}

	if o.LogPath == "stdout" {
		fmt.Printf("%s", str)
	} else if o.LogPath != "" {
		StrToFile(str, o.LogPath)
	}
}

// DestroyClient Function
func (fd *Feeder) DestroyClient() error {
	if err := fd.conn.Close(); err != nil {
		return err
	}
	fd.WgClient.Wait()
	return nil
}

func selectLabels(o Options, labels []string) error {
	for _, val := range o.Selector {
		for _, label := range labels {
			if val == label {
				return nil
			}
		}
	}
	return errors.New("Not found any flag")
}

func isDialingError(err error) bool {
	return strings.Contains(err.Error(), "Error while dialing")
}
