// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

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
var Telemetry []pb.Log
var TelMutex sync.RWMutex

func GetLogs() error {
	err := KarmorProfileStart("all")
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
			protojson.Unmarshal(evtin.Data, &log)
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

func KarmorProfileStart(logFilter string) error {
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
		})
		if err != nil {
			return
		}
	}()
	return err
}
