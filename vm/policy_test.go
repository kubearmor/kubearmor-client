// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Authors of KubeArmor

package vm

import (
	"bytes"
	"context"
	"crypto/tls"
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
)

type policyRequest struct {
	kind        string
	policy      []byte
	certificate []byte
	deadline    time.Time
}

type policyTestServer struct {
	pb.UnimplementedPolicyServiceServer
	requests chan policyRequest
	rpcError error
}

func (server *policyTestServer) receive(ctx context.Context, kind string, request *pb.Policy) (*pb.Response, error) {
	received := policyRequest{kind: kind, policy: request.Policy}
	received.deadline, _ = ctx.Deadline()
	if remote, ok := peer.FromContext(ctx); ok {
		if info, ok := remote.AuthInfo.(credentials.TLSInfo); ok {
			received.certificate = info.State.PeerCertificates[0].Raw
		}
	}
	server.requests <- received
	return &pb.Response{Status: pb.PolicyStatus_Applied}, server.rpcError
}

func (server *policyTestServer) ContainerPolicy(ctx context.Context, request *pb.Policy) (*pb.Response, error) {
	return server.receive(ctx, KubeArmorPolicy, request)
}

func (server *policyTestServer) HostPolicy(ctx context.Context, request *pb.Policy) (*pb.Response, error) {
	return server.receive(ctx, KubeArmorHostPolicy, request)
}

func (server *policyTestServer) NetworkPolicy(ctx context.Context, request *pb.Policy) (*pb.Response, error) {
	return server.receive(ctx, KubeArmorNetworkPolicy, request)
}

func startPolicyServer(t *testing.T, pki *testutil.PKI, rpcError error) (string, <-chan policyRequest) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var options []grpc.ServerOption
	if pki != nil {
		options = append(options, grpc.Creds(credentials.NewTLS(&tls.Config{
			MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pki.Server},
			ClientCAs: pki.Roots, ClientAuth: tls.RequireAndVerifyClientCert,
		})))
	}
	server := grpc.NewServer(options...)
	service := &policyTestServer{requests: make(chan policyRequest, 3), rpcError: rpcError}
	pb.RegisterPolicyServiceServer(server, service)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	return listener.Addr().String(), service.requests
}

func TestPolicyUsesPersistedManagementIdentity(t *testing.T) {
	pki := testutil.NewPKI(t)
	address, requests := startPolicyServer(t, &pki, nil)
	if _, err := os.Stat(filepath.Join(pki.ClientDir, "ca.key")); !os.IsNotExist(err) {
		t.Fatalf("client fixture must not contain a CA private key: %v", err)
	}
	for _, kind := range []string{KubeArmorPolicy, KubeArmorHostPolicy, KubeArmorNetworkPolicy} {
		data := []byte(`{"type":"ADDED"}`)
		if err := sendPolicyOverGRPC(PolicyOptions{GRPC: address, ManagementTLSCertPath: pki.ClientDir}, data, kind); err != nil {
			t.Fatal(err)
		}
		received := <-requests
		if received.kind != kind || !bytes.Equal(received.policy, data) {
			t.Fatalf("unexpected policy request: %+v", received)
		}
		if !bytes.Equal(received.certificate, pki.Client.Certificate[0]) {
			t.Fatal("policy did not use the persisted management client identity")
		}
		if received.deadline.IsZero() || time.Until(received.deadline) > transport.RPCTimeout {
			t.Fatalf("policy RPC has no bounded deadline: %v", received.deadline)
		}
	}
}

func TestPolicyFailsClosedOnTLSFailures(t *testing.T) {
	for _, name := range []string{"wrong server CA", "log client identity", "plaintext server"} {
		t.Run(name, func(t *testing.T) {
			management := testutil.NewPKI(t)
			log := testutil.NewPKI(t)
			serverPKI := &management
			switch name {
			case "wrong server CA":
				wrongCA, err := os.ReadFile(filepath.Join(log.ClientDir, "ca.crt"))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(management.ClientDir, "ca.crt"), wrongCA, 0600); err != nil {
					t.Fatal(err)
				}
			case "log client identity":
				testutil.WriteClientIdentity(t, management.ClientDir, log.Client)
			case "plaintext server":
				serverPKI = nil
			}
			address, requests := startPolicyServer(t, serverPKI, nil)
			err := sendPolicyOverGRPC(PolicyOptions{GRPC: address, ManagementTLSCertPath: management.ClientDir}, []byte("policy"), KubeArmorHostPolicy)
			if status.Code(err) != codes.Unavailable || !strings.Contains(err.Error(), address) {
				t.Fatalf("expected wrapped TLS transport failure at %s, got %v", address, err)
			}
			if name == "wrong server CA" && !strings.Contains(err.Error(), "unknown authority") {
				t.Fatalf("original server certificate validation error was lost: %v", err)
			}
			if name == "plaintext server" && !strings.Contains(err.Error(), "authentication handshake failed") {
				t.Fatalf("original TLS handshake failure was lost: %v", err)
			}
			select {
			case <-requests:
				t.Fatal("policy was delivered without valid mutual TLS")
			default:
			}
		})
	}
}

func TestPolicyPreservesRPCError(t *testing.T) {
	pki := testutil.NewPKI(t)
	address, _ := startPolicyServer(t, &pki, status.Error(codes.PermissionDenied, "policy denied"))
	err := sendPolicyOverGRPC(PolicyOptions{GRPC: address, ManagementTLSCertPath: pki.ClientDir}, []byte("policy"), KubeArmorHostPolicy)
	if status.Code(err) != codes.PermissionDenied || !strings.Contains(err.Error(), "policy denied") {
		t.Fatalf("expected original RPC error, got %v", err)
	}
}
