// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package log

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kubearmor/kubearmor-log-client/core"
	"github.com/rs/zerolog/log"
)

type Options struct {
	GRPC      string
	MsgPath   string
	LogPath   string
	LogFilter string
	JSON      bool
}

// StopChan Channel
var StopChan chan struct{}

// ==================== //
// == Signal Handler == //
// ==================== //

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

func StartObserver(o Options) error {
	gRPC := ""
	if o.GRPC != "" {
		gRPC = o.GRPC
	} else {
		if val, ok := os.LookupEnv("KUBEARMOR_SERVICE"); ok {
			gRPC = val
		} else {
			gRPC = "localhost:32767"
		}
	}

	fmt.Println("gRPC server: " + gRPC)

	if o.MsgPath == "none" && o.LogPath == "none" {
		flag.PrintDefaults()
		return nil
	}

	if o.LogFilter != "all" && o.LogFilter != "policy" && o.LogFilter != "system" {
		flag.PrintDefaults()
		return nil
	}

	// create a client
	logClient := core.NewClient(gRPC, o.MsgPath, o.LogPath, o.LogFilter)
	if logClient == nil {
		return errors.New("failed to connect to the gRPC server\nPossible troubleshooting:\n- Check if Kubearmor is running\n- Create a portforward to KubeArmor relay service using\n\t\033[1mkubectl -n kube-system port-forward service/kubearmor --address 0.0.0.0 --address :: 32767:32767\033[0m\n- Configure grpc server information using\n\t\033[1mkarmor log --grpc <info>\033[0m")
	}
	fmt.Printf("Created a gRPC client (%s)\n", gRPC)

	// do healthcheck
	if ok := logClient.DoHealthCheck(); !ok {
		return errors.New("failed to check the liveness of the gRPC server")
	}
	fmt.Println("Checked the liveness of the gRPC server")

	if o.MsgPath != "none" {
		// watch messages
		go func() {
			err := logClient.WatchMessages(o.MsgPath, o.JSON)
			if err != nil {
				log.Error().Msg(err.Error())
			}
		}()
		fmt.Println("Started to watch messages")
	}

	if o.LogPath != "none" {
		if o.LogFilter == "all" || o.LogFilter == "policy" {
			// watch alerts
			go func() {
				err := logClient.WatchAlerts(o.LogPath, o.JSON)
				if err != nil {
					log.Error().Msg(err.Error())
				}
			}()

			fmt.Println("Started to watch alerts")
		}

		if o.LogFilter == "all" || o.LogFilter == "system" {
			// watch logs
			go func() {
				err := logClient.WatchLogs(o.LogPath, o.JSON)
				if err != nil {
					log.Error().Msg(err.Error())
				}
			}()
			fmt.Println("Started to watch logs")
		}
	}

	// listen for interrupt signals
	sigChan := GetOSSigChannel()
	<-sigChan
	close(StopChan)

	logClient.Running = false
	time.Sleep(time.Second * 1)

	// destroy the client
	if err := logClient.DestroyClient(); err != nil {
		return err
	}
	fmt.Println("Destroyed the gRPC client")

	// == //
	return nil
}
