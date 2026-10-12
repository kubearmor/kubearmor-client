// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Authors of KubeArmor

package probe

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pb "github.com/kubearmor/KubeArmor/protobuf"
	"github.com/kubearmor/kubearmor-client/internal/transport"
	"github.com/kubearmor/kubearmor-client/internal/transport/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type probeRequest struct {
	certificate []byte
	deadline    time.Time
}

type standaloneTestServer struct {
	pb.UnimplementedProbeServiceServer
	requests chan probeRequest
}

func (server *standaloneTestServer) GetProbeData(ctx context.Context, _ *emptypb.Empty) (*pb.ProbeResponse, error) {
	remote, _ := peer.FromContext(ctx)
	info := remote.AuthInfo.(credentials.TLSInfo)
	deadline, _ := ctx.Deadline()
	server.requests <- probeRequest{certificate: info.State.PeerCertificates[0].Raw, deadline: deadline}
	return &pb.ProbeResponse{
		HostMap:       map[string]*pb.HostSecurityPolicies{"remote-host": {PolicyList: []string{"host-policy"}}},
		ContainerList: []string{"container"},
		ContainerMap:  map[string]*pb.ContainerData{"container": {PolicyEnabled: 1, PolicyList: []string{"container-policy"}}},
	}, nil
}

func startProbeServer(t *testing.T, pki testutil.PKI) (string, <-chan probeRequest) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{
		MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pki.Server},
		ClientCAs: pki.Roots, ClientAuth: tls.RequireAndVerifyClientCert,
	})))
	service := &standaloneTestServer{requests: make(chan probeRequest, 1)}
	pb.RegisterProbeServiceServer(server, service)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	return listener.Addr().String(), service.requests
}

func TestStandaloneProbeWithoutKubernetesOrLocalHostData(t *testing.T) {
	pki := testutil.NewPKI(t)
	address, requests := startProbeServer(t, pki)
	var output bytes.Buffer
	if err := PrintProbeResultCmd(nil, Options{
		NonK8s: true, GRPC: address, ManagementTLSCertPath: pki.ClientDir, Output: "json", Writer: &output,
	}); err != nil {
		t.Fatal(err)
	}
	var result map[string]map[string]json.RawMessage
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output %q: %v", output.String(), err)
	}
	data := result["Probe Data"]
	if _, exists := data["Host"]; exists {
		t.Fatal("remote probe must not report this machine's host metadata")
	}
	if !bytes.Contains(data["HostPolicies"], []byte("host-policy")) || !bytes.Contains(data["ArmoredContainers"], []byte("container-policy")) {
		t.Fatalf("missing management RPC policies: %s", output.String())
	}
	received := <-requests
	if !bytes.Equal(received.certificate, pki.Client.Certificate[0]) {
		t.Fatal("probe did not use the persisted management client identity")
	}
	if received.deadline.IsZero() || time.Until(received.deadline) > transport.RPCTimeout {
		t.Fatalf("probe RPC has no bounded deadline: %v", received.deadline)
	}
}

func TestStandaloneProbeRejectsWrongTrustPlane(t *testing.T) {
	for _, name := range []string{"wrong server CA", "log client identity"} {
		t.Run(name, func(t *testing.T) {
			management := testutil.NewPKI(t)
			log := testutil.NewPKI(t)
			address, requests := startProbeServer(t, management)
			if name == "log client identity" {
				testutil.WriteClientIdentity(t, management.ClientDir, log.Client)
			} else {
				wrongCA, err := os.ReadFile(filepath.Join(log.ClientDir, "ca.crt"))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(management.ClientDir, "ca.crt"), wrongCA, 0600); err != nil {
					t.Fatal(err)
				}
			}
			_, err := getPolicyData(Options{GRPC: address, ManagementTLSCertPath: management.ClientDir})
			if status.Code(err) != codes.Unavailable {
				t.Fatalf("expected TLS rejection, got %v", err)
			}
			if name == "wrong server CA" && !strings.Contains(err.Error(), "unknown authority") {
				t.Fatalf("original server certificate validation error was lost: %v", err)
			}
			select {
			case <-requests:
				t.Fatal("probe accepted the wrong trust plane")
			default:
			}
		})
	}
}
