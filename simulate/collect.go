// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package simulate

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	pb "github.com/kubearmor/KubeArmor/protobuf"
	"github.com/kubearmor/kubearmor-client/k8s"
	klog "github.com/kubearmor/kubearmor-client/log"
)

const defaultCollectLimit = 1000

// CollectOptions configures telemetry gathering.
type CollectOptions struct {
	EventsFile       string
	Namespace        string
	Pod              string
	Last             time.Duration
	CollectTimeout   time.Duration
	Limit            uint32
	GRPC             string
	Secure           bool
	TlsCertPath      string
	TlsCertProvider  string
	ReadCAFromSecret bool
}

// CollectEvents returns system logs for simulation, from a file or live relay.
func CollectEvents(client *k8s.Client, opts CollectOptions) ([]pb.Log, error) {
	if opts.EventsFile != "" {
		return collectFromFile(opts)
	}
	return collectLive(client, opts)
}

func collectFromFile(opts CollectOptions) ([]pb.Log, error) {
	var r io.Reader
	if opts.EventsFile == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(opts.EventsFile)
		if err != nil {
			return nil, fmt.Errorf("open events file: %w", err)
		}
		defer f.Close()
		r = f
	}

	cutoff := time.Now().Add(-opts.Last)
	var events []pb.Log
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var evt pb.Log
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			continue
		}
		if !eventInWindow(evt, cutoff) {
			continue
		}
		if !eventMatchesFilters(evt, opts.Namespace, opts.Pod) {
			continue
		}
		events = append(events, evt)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read events file: %w", err)
	}
	return events, nil
}

func collectLive(client *k8s.Client, opts CollectOptions) ([]pb.Log, error) {
	limit := opts.Limit
	if limit == 0 {
		limit = defaultCollectLimit
	}

	eventChan := make(chan klog.EventInfo, 256)
	errChan := make(chan error, 1)

	logOpts := klog.Options{
		GRPC:             opts.GRPC,
		Secure:           opts.Secure,
		TlsCertPath:      opts.TlsCertPath,
		TlsCertProvider:  opts.TlsCertProvider,
		ReadCAFromSecret: opts.ReadCAFromSecret,
		MsgPath:          "none",
		LogPath:          "stdout",
		LogFilter:        "system",
		EventChan:        eventChan,
		Namespace:        opts.Namespace,
		PodName:          opts.Pod,
		Limit:            limit,
	}

	go func() {
		errChan <- klog.StartObserver(client, logOpts)
	}()

	cutoff := time.Now().Add(-opts.Last)
	timeout := opts.CollectTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	deadline := time.After(timeout)

	var events []pb.Log
collectLoop:
	for {
		select {
		case <-deadline:
			break collectLoop
		case err := <-errChan:
			if err != nil && len(events) == 0 {
				return nil, fmt.Errorf("collect telemetry: %w", err)
			}
			break collectLoop
		case evtin, ok := <-eventChan:
			if !ok {
				break collectLoop
			}
			if evtin.Type != "Log" {
				continue
			}
			var evt pb.Log
			if err := json.Unmarshal(evtin.Data, &evt); err != nil {
				continue
			}
			if !eventInWindow(evt, cutoff) {
				continue
			}
			if !eventMatchesFilters(evt, opts.Namespace, opts.Pod) {
				continue
			}
			events = append(events, evt)
		}
	}

	return events, nil
}

func eventInWindow(evt pb.Log, cutoff time.Time) bool {
	if evt.UpdatedTime != "" {
		if t, err := time.Parse(time.RFC3339, evt.UpdatedTime); err == nil {
			return !t.Before(cutoff)
		}
		if t, err := time.Parse(time.RFC3339Nano, evt.UpdatedTime); err == nil {
			return !t.Before(cutoff)
		}
	}
	if evt.Timestamp > 0 {
		return time.Unix(0, evt.Timestamp).After(cutoff) || time.Unix(0, evt.Timestamp).Equal(cutoff)
	}
	return true
}

func eventMatchesFilters(evt pb.Log, namespace, pod string) bool {
	if namespace != "" && evt.NamespaceName != namespace {
		return false
	}
	if pod != "" && evt.PodName != pod {
		return false
	}
	return true
}

// ParseDurationFlag parses a duration string like 30m, 1h.
func ParseDurationFlag(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}
