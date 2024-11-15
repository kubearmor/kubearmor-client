// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

// Package log connects and observes telemetry from KubeArmor
package log

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/kubearmor/kubearmor-client/utils"
)

type regexType *regexp.Regexp

const (
	SelfCertProvider   string = "self"
	ExternalCertLoader string = "external"
)

// Regex Compiled Structs
var (
	CNamespace     regexType
	CLogtype       regexType
	COperation     regexType
	CContainerName regexType
	CPodName       regexType
	CSource        regexType
	CResource      regexType
)

// Options Structure
type Options struct {
	GRPC             string
	Secure           bool
	TlsCertPath      string
	TlsCertProvider  string
	ReadCAFromSecret bool
	MsgPath          string
	LogPath          string
	LogFilter        string
	JSON             bool
	Output           string
	Namespace        string
	LogType          string
	Operation        string
	ContainerName    string
	PodName          string
	Source           string
	Resource         string
	Limit            uint32
	Selector         []string
	EventChan        chan EventInfo // channel to send events on
}

// StopChan Channel
var (
	StopChan chan struct{}
	sigChan  chan os.Signal
)

// UnblockSignal is a flag to check whether the Watch* APIs have exited or signal has rcvd
var (
	UnblockSignal error
	matchLabels         = map[string]string{"kubearmor-app": "kubearmor-relay"}
	port          int64 = 32767
)

// GetOSSigChannel Function
func GetOSSigChannel() chan os.Signal {
	c := make(chan os.Signal, 1)

	signal.Notify(c,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
		os.Interrupt)

	return c
}

func regexCompile(o Options) error {
	var err error

	CNamespace, err = regexp.Compile("(?i)" + o.Namespace)
	if err != nil {
		return err
	}
	CLogtype, err = regexp.Compile("(?i)" + o.LogType)
	if err != nil {
		return err
	}
	COperation, err = regexp.Compile("(?i)" + o.Operation)
	if err != nil {
		return err
	}
	CContainerName, err = regexp.Compile("(?i)" + o.ContainerName)
	if err != nil {
		return err
	}
	CPodName, err = regexp.Compile("(?i)" + o.PodName)
	if err != nil {
		return err
	}
	CSource, err = regexp.Compile(o.Source)
	if err != nil {
		return err
	}
	CResource, err = regexp.Compile(o.Resource)
	if err != nil {
		return err
	}
	return nil
}

func closeStopChan() {
	if StopChan == nil {
		return
	}
	close(StopChan)
	StopChan = nil
}

// StartObserver Function
func StartObserver(c *k8s.Client, o Options) error {
	gRPC := ""
	targetSvc := "kubearmor-relay"

	if o.GRPC != "" {
		gRPC = o.GRPC
	} else if val, ok := os.LookupEnv("KUBEARMOR_SERVICE"); ok {
		gRPC = val
	} else {
		pf, err := utils.InitiatePortForward(c, port, port, matchLabels, targetSvc)
		if err != nil {
			return err
		}
		gRPC = "localhost:" + strconv.FormatInt(pf.LocalPort, 10)
	}

	if o.MsgPath == "none" && o.LogPath == "none" {
		flag.PrintDefaults()
		return nil
	}

	if o.LogFilter != "all" && o.LogFilter != "policy" && o.LogFilter != "system" {
		flag.PrintDefaults()
		return nil
	}

	// create client
	logClient, err := NewClient(gRPC, o, c.K8sClientset)
	if err != nil {
		if !o.Secure && !isDialingError(err) {
			// retry connecting to the server on secured channel
			fmt.Fprintf(os.Stderr, "Failed to connect on insecure channel\n(%s)\n", err)
			fmt.Fprint(os.Stderr, "Trying to reconnect using secured channel...\n")
			o.Secure = true
			logClient, err = NewClient(gRPC, o, c.K8sClientset)
			if err != nil {
				return fmt.Errorf("unable to create log client, error=%s", err)
			}
		} else {
			return fmt.Errorf("unable to create log client, error=%s", err)
		}
	}

	fmt.Fprintf(os.Stderr, "Created a gRPC client (%s)\n", gRPC)

	// do healthcheck
	if ok := logClient.DoHealthCheck(); !ok {
		return errors.New("failed to check the liveness of the gRPC server")
	}
	fmt.Fprintln(os.Stderr, "Checked the liveness of the gRPC server")

	if o.MsgPath != "none" {
		// watch messages
		go logClient.WatchMessages(o.MsgPath, o.JSON)
		fmt.Fprintln(os.Stderr, "Started to watch messages")
	}

	err = regexCompile(o)
	if err != nil {
		fmt.Print(err)
		return err
	}

	Limitchan = make(chan bool, 2)
	if o.LogPath != "none" {
		if o.LogFilter == "all" || o.LogFilter == "policy" {
			// watch alerts
			go logClient.WatchAlerts(o)
			fmt.Fprintln(os.Stderr, "Started to watch alerts")
		}

		if o.LogFilter == "all" || o.LogFilter == "system" {
			// watch logs
			go logClient.WatchLogs(o)
			fmt.Fprintln(os.Stderr, "Started to watch logs")
		}
	}

	ctrlc := false
	if o.Limit != 0 {
		<-Limitchan
		if o.LogFilter == "all" {
			<-Limitchan
		}
	} else {
		// listen for interrupt signals
		UnblockSignal = nil
		sigChan = GetOSSigChannel()
		for UnblockSignal == nil && !ctrlc {
			time.Sleep(50 * time.Millisecond)
			select {
			case <-sigChan:
				ctrlc = true
			default:
			}
		}
	}
	fmt.Fprintln(os.Stderr, "releasing grpc client")
	closeStopChan()

	logClient.Running = false

	// destroy the client
	_ = logClient.DestroyClient()
	if ctrlc {
		return nil
	}
	return UnblockSignal
}
