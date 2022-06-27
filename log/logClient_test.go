package log

import (
	"fmt"
	"testing"
	"time"

	pb "github.com/kubearmor/KubeArmor/protobuf"
)

var eventChan chan []byte
var gotEvent = false

const maxEvents = 5

func waitOnEvent(cnt int) {
	for i := 0; i < cnt; i++ {
		evt := <-eventChan
		fmt.Printf("Event: %s\n", string(evt))
	}
	gotEvent = true
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
	eventChan = make(chan []byte, maxEvents)
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
	for i := 0; i < 10 && !gotEvent; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if !gotEvent {
		t.Errorf("did not receive the event")
	}
}
