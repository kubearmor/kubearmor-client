// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Authors of KubeArmor

package log

import (
	"bytes"
	"context"
	"crypto/tls"
	"net"
	"os"
	"path/filepath"
	"testing"

	pb "github.com/kubearmor/KubeArmor/protobuf"
	"github.com/kubearmor/kubearmor-client/internal/transport/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

type standaloneLogService struct {
	pb.UnimplementedLogServiceServer
	clientCertificates chan []byte
}

func (server *standaloneLogService) HealthCheck(ctx context.Context, request *pb.NonceMessage) (*pb.ReplyMessage, error) {
	if remote, ok := peer.FromContext(ctx); ok {
		if info, ok := remote.AuthInfo.(credentials.TLSInfo); ok && len(info.State.PeerCertificates) > 0 {
			server.clientCertificates <- info.State.PeerCertificates[0].Raw
		}
	}
	return &pb.ReplyMessage{Retval: request.Nonce}, nil
}

func (*standaloneLogService) WatchAlerts(_ *pb.RequestMessage, stream grpc.ServerStreamingServer[pb.Alert]) error {
	return stream.Send(&pb.Alert{PolicyName: "standalone-test-policy"})
}

func serveStandaloneLogs(t *testing.T, pki *testutil.PKI) (string, *standaloneLogService) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var options []grpc.ServerOption
	if pki != nil {
		options = append(options, grpc.Creds(credentials.NewTLS(&tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{pki.Server},
			ClientCAs:    pki.Roots,
			ClientAuth:   tls.RequireAndVerifyClientCert,
		})))
	}
	server := grpc.NewServer(options...)
	service := &standaloneLogService{clientCertificates: make(chan []byte, 1)}
	pb.RegisterLogServiceServer(server, service)
	t.Cleanup(server.Stop)
	go func() { _ = server.Serve(listener) }()
	return listener.Addr().String(), service
}

func copyLogClientFiles(t *testing.T, source, destination string) {
	t.Helper()
	if err := os.MkdirAll(destination, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ca.crt", "client.crt", "client.key"} {
		data, err := os.ReadFile(filepath.Join(source, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(destination, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func standaloneLogOptions(directory, address string) Options {
	return Options{
		NonK8s: true, GRPC: address, TlsCertPath: directory,
		// Standalone transport must remain secure even for programmatic callers
		// passing the Kubernetes defaults instead of normalized CLI options.
		TlsCertProvider: SelfCertProvider, ReadCAFromSecret: true,
		MsgPath: "none", LogPath: "stdout", LogFilter: "policy", Limit: 1,
		EventChan: make(chan EventInfo, 1),
	}
}

func TestStandaloneLogsUsePersistedIdentity(t *testing.T) {
	for _, split := range []bool{true, false} {
		name := "single CA directory and environment endpoint"
		if split {
			name = "split PKI and explicit endpoint"
		}
		t.Run(name, func(t *testing.T) {
			pki := testutil.NewPKI(t)
			address, service := serveStandaloneLogs(t, &pki)
			directory := pki.ClientDir
			if split {
				directory = t.TempDir()
				copyLogClientFiles(t, pki.ClientDir, filepath.Join(directory, "log"))
				management := testutil.NewPKI(t)
				copyLogClientFiles(t, management.ClientDir, filepath.Join(directory, "management"))
				// Explicit --gRPC takes precedence over an unrelated environment value.
				t.Setenv("KUBEARMOR_SERVICE", "127.0.0.1:1")
			} else {
				t.Setenv("KUBEARMOR_SERVICE", address)
				address = ""
			}
			if _, err := os.Stat(filepath.Join(ResolveLogCertPath(directory), "ca.key")); !os.IsNotExist(err) {
				t.Fatalf("client fixture must not contain ca.key: %v", err)
			}
			options := standaloneLogOptions(directory, address)
			StopChan = make(chan struct{})
			UnblockSignal = nil
			if err := StartObserver(nil, options); err != nil {
				t.Fatalf("standalone observer: %v", err)
			}
			select {
			case certificate := <-service.clientCertificates:
				if !bytes.Equal(certificate, pki.Client.Certificate[0]) {
					t.Fatal("observer did not present the persisted log client identity")
				}
			default:
				t.Fatal("observer did not perform an authenticated health RPC")
			}
			select {
			case event := <-options.EventChan:
				if event.Type != "Alert" || !bytes.Contains(event.Data, []byte("standalone-test-policy")) {
					t.Fatalf("unexpected log event: %+v", event)
				}
			default:
				t.Fatal("observer did not receive its alert")
			}
		})
	}
}

func TestStandaloneLogsRejectManagementIdentity(t *testing.T) {
	logPKI := testutil.NewPKI(t)
	managementPKI := testutil.NewPKI(t)
	address, service := serveStandaloneLogs(t, &logPKI)
	// Trust the correct log server, but offer a client signed by the management CA.
	testutil.WriteClientIdentity(t, logPKI.ClientDir, managementPKI.Client)
	if err := StartObserver(nil, standaloneLogOptions(logPKI.ClientDir, address)); err == nil {
		t.Fatal("log server accepted the management client identity")
	}
	if len(service.clientCertificates) != 0 {
		t.Fatal("management identity reached the log service")
	}
}

func TestStandaloneLogsRejectPlaintextServer(t *testing.T) {
	pki := testutil.NewPKI(t)
	address, _ := serveStandaloneLogs(t, nil)
	if err := StartObserver(nil, standaloneLogOptions(pki.ClientDir, address)); err == nil {
		t.Fatal("standalone observer connected to a plaintext server")
	}
}
