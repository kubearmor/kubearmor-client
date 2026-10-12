// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

// Package log connects and observes telemetry from KubeArmor
package log

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/kubearmor/KubeArmor/KubeArmor/cert"
	"github.com/kubearmor/kubearmor-client/internal/transport"
	"github.com/kubearmor/kubearmor-client/k8s"
	"google.golang.org/grpc/credentials"
	"k8s.io/client-go/kubernetes"
)

func loadTLSCredentials(client kubernetes.Interface, o Options) (credentials.TransportCredentials, error) {
	// Prefer the log plane when the configured directory contains split PKI.
	logCertPath := ResolveLogCertPath(o.TlsCertPath)
	if !o.ReadCAFromSecret && o.TlsCertProvider == ExternalCertLoader {
		return transport.ClientCredentials(logCertPath)
	}
	var secret, namespace string
	var clientCertCfg cert.CertConfig
	var clientset *kubernetes.Clientset
	if o.ReadCAFromSecret {
		var ok bool
		clientset, ok = client.(*kubernetes.Clientset)
		if !ok || clientset == nil {
			return nil, fmt.Errorf("a Kubernetes clientset is required to read the CA secret")
		}
		secret, namespace = k8s.GetKubeArmorCaSecret(client)
		if secret == "" || namespace == "" {
			return credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12}), fmt.Errorf("error getting kubearmor ca secret")
		}
	}
	if o.TlsCertProvider == SelfCertProvider {
		// create certificate configurations
		clientCertCfg = cert.DefaultKubeArmorClientConfig
		clientCertCfg.NotAfter = time.Now().AddDate(1, 0, 0) // valid for 1 year
	}
	tlsConfig := cert.TlsConfig{
		CertCfg:              clientCertCfg,
		ReadCACertFromSecret: o.ReadCAFromSecret,
		SecretName:           secret,
		Namespace:            namespace,
		K8sClient:            clientset,
		CertPath:             cert.GetClientCertPath(logCertPath),
		CertProvider:         o.TlsCertProvider,
		CACertPath:           cert.GetCACertPath(logCertPath),
	}
	manager := cert.NewTlsCredentialManager(&tlsConfig)
	if manager == nil {
		return nil, fmt.Errorf("unsupported TLS certificate provider %q", o.TlsCertProvider)
	}
	creds, err := manager.CreateTlsClientCredentials()
	if err != nil {
		fmt.Println(err.Error())
	}
	return creds, err
}
