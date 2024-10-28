// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

// Package profile to fetch logs
package profile

import (
	"errors"
	pb "github.com/kubearmor/KubeArmor/protobuf"
	"github.com/kubearmor/kubearmor-client/k8s"
	klog "github.com/kubearmor/kubearmor-client/log"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/encoding/protojson"
	"sync"
)

var eventChan chan klog.EventInfo

// ErrChan to make error channels from goroutines
var ErrChan chan error

// Telemetry to store incoming log events
var Telemetry []pb.Log

// TelMutex to prevent deadlock
var TelMutex sync.RWMutex

// Options for filter
type Options struct {
	Namespace string
	Pod       string
	LogFilter string
	LogType   string
	GRPC      string
	Container string
	Save      bool
}

// GetLogs to fetch logs
func GetLogs(o Options) error {
	errCh := KarmorProfileStart(o)
	var err error
	if eventChan == nil {
		log.Error("event channel not set. Did you call KarmorQueueLog()?")
		return errors.New("event channel not set")
	}

	for eventChan != nil {
		select {
		case evtin := <-eventChan:
			if evtin.Type == "Log" {
				log := pb.Log{}
				err := protojson.Unmarshal(evtin.Data, &log)
				if err != nil {
					return err
				}
				TelMutex.Lock()
				Telemetry = append(Telemetry, log)
				TelMutex.Unlock()
			} else {
				log.Errorf("UNKNOWN EVT type %s", evtin.Type)
			}
		// }
		case err := <-errCh:
			return err
		}
	}
	return err
}

// KarmorProfileStart starts observer
func KarmorProfileStart(o Options) <-chan error {
	ErrChan = make(chan error, 1)
	if eventChan == nil {
		eventChan = make(chan klog.EventInfo)
	}
	client, err := k8s.ConnectK8sClient()
	if err != nil {
		ErrChan <- err
	}

	go func() {
		//defer close(ErrChan)
		err = klog.StartObserver(client, klog.Options{
			LogFilter: o.LogFilter,
			LogType:   o.LogType,
			MsgPath:   "none",
			EventChan: eventChan,
			GRPC:      o.GRPC,
		})

		select {
		case ErrChan <- err:
			log.Errorf("failed to start observer. Error=%s", err.Error())
		default:
			break
		}
	}()

	return ErrChan
}
