// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

// Package log connects and observes telemetry from KubeArmor
package log

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/kubearmor/KubeArmor/KubeArmor/cert"
	"github.com/kubearmor/kubearmor-client/k8s"
	"google.golang.org/grpc/credentials"
	"k8s.io/client-go/kubernetes"
)

func loadTLSCredentials(client kubernetes.Interface, o Options) (credentials.TransportCredentials, error) {
	var secret, namespace string
	var clientCertCfg cert.CertConfig
	if o.ReadCAFromSecret {
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
	// Resolve the log trust plane: with a split-PKI layout
	// (<base>/log/ca.crt present) log commands use the Log CA + log
	// client identity; otherwise fall back to the configured path
	// (single-CA layouts).
	logCertPath := ResolveLogCertPath(o.TlsCertPath)
	tlsConfig := cert.TlsConfig{
		CertCfg:              clientCertCfg,
		ReadCACertFromSecret: o.ReadCAFromSecret,
		SecretName:           secret,
		Namespace:            namespace,
		K8sClient:            client.(*kubernetes.Clientset),
		CertPath:             cert.GetClientCertPath(logCertPath),
		CertProvider:         o.TlsCertProvider,
		CACertPath:           cert.GetCACertPath(logCertPath),
	}
	creds, err := cert.NewTlsCredentialManager(&tlsConfig).CreateTlsClientCredentials()
	if err != nil {
		fmt.Println(err.Error())
	}
	return creds, err
}
