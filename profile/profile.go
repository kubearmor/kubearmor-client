// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

// Package profile to fetch logs
package profile

import (
	"errors"
	"sync"

	pb "github.com/kubearmor/KubeArmor/protobuf"

	"github.com/kubearmor/kubearmor-client/k8s"
	klog "github.com/kubearmor/kubearmor-client/log"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/encoding/protojson"
)

var eventChan chan klog.EventInfo
var client *k8s.Client

// Telemetry to store incoming log events
var Telemetry []pb.Log

// TelMutex to prevent deadlock
var TelMutex sync.RWMutex

// GetLogs to fetch logs
func GetLogs(grpc string, limit uint32) error {
	err := KarmorProfileStart("system", grpc, limit)
	if err != nil {
		return err
	}
	if eventChan == nil {
		log.Error("event channel not set. Did you call KarmorQueueLog()?")
		return errors.New("event channel not set")
	}

	for eventChan != nil {
		evtin := <-eventChan
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
	}
	return err
}

// KarmorProfileStart starts observer
func KarmorProfileStart(logFilter string, grpc string, limit uint32) error {
	if eventChan == nil {
		eventChan = make(chan klog.EventInfo)
	}
	var err error
	client, err = k8s.ConnectK8sClient()
	if err != nil {

		return err
	}
	go func() {
		err = klog.StartObserver(client, klog.Options{
			LogFilter: logFilter,
			MsgPath:   "none",
			EventChan: eventChan,
			GRPC:      grpc,
			Limit:     limit,
		})
		if err != nil {
			return
		}

	}()
	return err
}
