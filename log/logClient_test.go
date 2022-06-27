package log

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	pb "github.com/kubearmor/KubeArmor/protobuf"
)

var eventChan chan interface{}
var gotAlerts = 0
var gotLogs = 0

const maxEvents = 5

func waitOnEvent(cnt int) {
	for i := 0; i < cnt; i++ {
		evtin := <-eventChan
		switch evt := evtin.(type) {
		case pb.Alert:
			gotAlerts++
		case pb.Log:
			gotLogs++
		default:
			fmt.Printf("unknown event rcvd %v\n", reflect.TypeOf(evt))
		}
	}
}

func TestLogClient(t *testing.T) {
	var res = pb.Alert{
		ClusterName:    "breaking-bad",
		HostName:       "saymyname",
		NamespaceName:  "heisenberg",
		PodName:        "new-mexico",
		Labels:         "substance=meth,currency=usd",
		ContainerID:    "12345678901234567890",
		ContainerName:  "los-polos",
		ContainerImage: "evergreen",
	}
	eventChan = make(chan interface{}, maxEvents)
	var o = Options{
		EventChan: eventChan,
	}
	for i := 0; i < maxEvents; i++ {
		err := watchAlertsHelper(&res, o)
		if err != nil {
			t.Errorf("watchAlertsHelper failed\n")
		}
	}
	go waitOnEvent(maxEvents)
	for i := 0; i < 10 && gotAlerts < maxEvents; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if gotAlerts < maxEvents {
		t.Errorf("did not receive all the events")
	}
}
