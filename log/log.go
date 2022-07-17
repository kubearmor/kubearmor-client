// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package log

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"
)

type regexType *regexp.Regexp

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
	GRPC          string
	MsgPath       string
	LogPath       string
	LogFilter     string
	JSON          bool
	Namespace     string
	LogType       string
	Operation     string
	ContainerName string
	PodName       string
	Source        string
	Resource      string
	Limit         uint32
	Selector      []string
	EventChan     chan EventInfo // channel to send events on
}

// StopChan Channel
var StopChan chan struct{}
var sigChan chan os.Signal
var unblockSignal = false

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
func StartObserver(o Options) error {
	gRPC := "localhost:32767"

	if o.GRPC != "" {
		gRPC = o.GRPC
	} else if val, ok := os.LookupEnv("KUBEARMOR_SERVICE"); ok {
		gRPC = val
	}

	fmt.Fprintln(os.Stderr, "gRPC server: "+gRPC)

	if o.MsgPath == "none" && o.LogPath == "none" {
		flag.PrintDefaults()
		return nil
	}

	if o.LogFilter != "all" && o.LogFilter != "policy" && o.LogFilter != "system" {
		flag.PrintDefaults()
		return nil
	}

	// create a client
	logClient := NewClient(gRPC, o.MsgPath, o.LogPath, o.LogFilter, o.Limit)
	if logClient == nil {
		return errors.New("failed to connect to the gRPC server\nPossible troubleshooting:\n- Check if Kubearmor is running\n- Create a portforward to KubeArmor relay service using\n\t\033[1mkubectl -n kube-system port-forward service/kubearmor --address 0.0.0.0 --address :: 32767:32767\033[0m\n- Configure grpc server information using\n\t\033[1mkarmor log --grpc <info>\033[0m")
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

	err := regexCompile(o)
	if err != nil {
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

	if o.Limit != 0 {
		if o.LogFilter == "all" {
			<-Limitchan
			<-Limitchan
		} else {
			<-Limitchan
		}
	} else {
		// listen for interrupt signals
		unblockSignal = false
		sigChan = GetOSSigChannel()
		for !unblockSignal {
			time.Sleep(50 * time.Millisecond)
			select {
			case <-sigChan:
				unblockSignal = true
			default:
			}
		}
	}
	fmt.Fprintln(os.Stderr, "releasing grpc client")
	closeStopChan()

	logClient.Running = false

	// destroy the client
	return logClient.DestroyClient()
}

// StopObserver unblocks signal
func StopObserver() {
	unblockSignal = true
}
