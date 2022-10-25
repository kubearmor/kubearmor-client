package profile

import (
	"errors"
	"sync"

	pb "github.com/kubearmor/KubeArmor/protobuf"
	// . "github.com/kubearmor/KubeArmor/tests/util"
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
	var err error
	// err = KubearmorPortForward()
	KarmorProfileStart("all")
	if err != nil {
		return err
	}
	if eventChan == nil {
		log.Error("event channel not set. Did you call KarmorQueueLog()?")
		return errors.New("event channel not set")
	}

	log.Println("Starting to read 1")
	for eventChan != nil {
		// fmt.Printf("event before\n")
		evtin := <-eventChan
		// fmt.Printf("event after\n")
		if evtin.Type == "Log" {
			log := pb.Log{}
			protojson.Unmarshal(evtin.Data, &log)
			// TelMutex.Lock()
			Telemetry = append(Telemetry, log)
			// TelMutex.Unlock()
			// b, err := json.MarshalIndent(Telemetry, "", "  ")
			// if err != nil {
			// 	fmt.Println("error:", err)
			// }
			// fmt.Printf(string(b))
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
	// fmt.Printf("%v", client.K8sClientset)
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
	log.Println("Ending to log")
	return err
}
