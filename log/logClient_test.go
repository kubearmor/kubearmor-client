package log

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	pb "github.com/kubearmor/KubeArmor/protobuf"
)

var eventChan chan []byte
var done chan bool
var gotAlerts = 0
var gotLogs = 0

const maxEvents = 5

func genericWaitOnEvent(cnt int) {
	for evtin := range eventChan {
		var res map[string]interface{}
		err := json.Unmarshal(evtin, &res)
		if err != nil {
			fmt.Printf("error unmarshling %v\n", err.Error())
		}
		switch res["Type"].(string) {
		case "MatchedPolicy":
			gotAlerts++
		case "ContainerLog", "HostLog":
			gotLogs++
		default:
			fmt.Printf("unknown event\n")
			break
		}

		if gotAlerts+gotLogs >= cnt {
			break
		}
	}
	done <- true
}

func TestLogClient(t *testing.T) {
	var res = &pb.Alert{
		ClusterName:    "breaking-bad",
		HostName:       "saymyname",
		NamespaceName:  "heisenberg",
		PodName:        "new-mexico",
		Labels:         "substance=meth,currency=usd",
		ContainerID:    "12345678901234567890",
		ContainerName:  "los-polos",
		ContainerImage: "evergreen",
		Type:           "MatchedPolicy",
	}
	eventChan = make(chan []byte, maxEvents)
	var o = Options{
		EventChan: eventChan,
	}

	tel, err := json.Marshal(res)
	if err != nil {
		t.Error(err.Error())
		return
	}

	// Handle Telemetry Events
	for i := 0; i < maxEvents; i++ {
		WatchTelemetryHelper(tel, "Alert", o)
	}

	done = make(chan bool, 1)
	go genericWaitOnEvent(maxEvents)

	// Check for timeouts
	select {
	case <-done:
		if gotAlerts < maxEvents {
			t.Errorf("did not receive all the events")
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("timed out")
	}
}
