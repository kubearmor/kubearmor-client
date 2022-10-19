package profile

import (
	"errors"

	pb "github.com/kubearmor/KubeArmor/protobuf"
	. "github.com/kubearmor/KubeArmor/tests/util"
	klog "github.com/kubearmor/kubearmor-client/log"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/encoding/protojson"
)

var eventChan chan klog.EventInfo

func GetLogs() ([]pb.Log, error) {
	if eventChan == nil {
		log.Error("event channel not set. Did you call KarmorQueueLog()?")
		return nil, errors.New("event channel not set")
	}
	logs := []pb.Log{}

	for eventChan != nil {
		evtin := <-eventChan
		if evtin.Type == "Log" {
			log := pb.Log{}
			protojson.Unmarshal(evtin.Data, &log)
			logs = append(logs, log)
			// b, err := json.MarshalIndent(logs, "", "  ")
			// if err != nil {
			// 	fmt.Println("error:", err)
			// }
		} else {
			log.Errorf("UNKNOWN EVT type %s", evtin.Type)
		}
		// }
	}
	return logs, nil
}

func KarmorProfileStart(logFilter string) ([]pb.Log, error) {
	if eventChan == nil {
		eventChan = make(chan klog.EventInfo)
	}
	KubearmorPortForward()
	logs := []pb.Log{}
	go func() {
		err := klog.StartObserver(klog.Options{
			LogFilter: logFilter,
			MsgPath:   "none",
			EventChan: eventChan,
		})
		if err != nil {
			log.Errorf("failed to start observer. Error=%s", err.Error())
		}
		logs, _ = GetLogs()
	}()
	return logs, nil
}

// Stops the Observer
func KarmorProfileStop() {
	klog.StopObserver()
}
