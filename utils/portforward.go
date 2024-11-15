// Package utils provides utility for port forwarding.
package utils

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/kubearmor/kubearmor-client/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// PortForwardOpt details for a pod
type PortForwardOpt struct {
	LocalPort   int64
	RemotePort  int64
	MatchLabels map[string]string
	Namespace   string
	PodName     string
	TargetSvc   string
}

// InitiatePortForward : Initiate port forwarding
func InitiatePortForward(c *k8s.Client, localPort int64, remotePort int64, matchLabels map[string]string, targetSvc string) (PortForwardOpt, error) {
	pf := PortForwardOpt{
		LocalPort:   localPort,
		RemotePort:  remotePort,
		Namespace:   "",
		MatchLabels: matchLabels,
		TargetSvc:   targetSvc,
	}

	// handle port forward
	err := pf.handlePortForward(c)
	if err != nil {
		return pf, err
	}
	return pf, nil
}

// handle port forward to allow grpc to connect at localhost:PORT
func (pf *PortForwardOpt) handlePortForward(c *k8s.Client) error {
	if err := pf.getPodName(c); err != nil {
		return err
	}

	// local port
	lp, err := pf.getLocalPort()
	if err != nil {
		return err
	}
	pf.LocalPort = lp

	err = k8sPortForward(c, *pf)
	if err != nil {
		return fmt.Errorf("\ncould not do kubearmor portforward, error=%s", err.Error())
	}
	return nil
}

// k8s port forward
func k8sPortForward(c *k8s.Client, pf PortForwardOpt) error {
	roundTripper, upgrader, err := spdy.RoundTripperFor(c.Config)
	if err != nil {
		return fmt.Errorf("\nunable to create round tripper and upgrader, error=%s", err.Error())
	}

	serverURL, err := url.Parse(c.Config.Host)
	if err != nil {
		return fmt.Errorf("\nfailed to parse apiserver URL from kubeconfig. error=%s", err.Error())
	}
	serverURL.Path = fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", pf.Namespace, pf.PodName)

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, serverURL)

	StopChan, readyChan := make(chan struct{}, 1), make(chan struct{}, 1)
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)

	forwarder, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", pf.LocalPort, pf.RemotePort)},
		StopChan, readyChan, out, errOut)
	if err != nil {
		return fmt.Errorf("\nunable to portforward. error=%s", err.Error())
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- forwarder.ForwardPorts()
	}()

	select {
	case err = <-errChan:
		close(errChan)
		forwarder.Close()
		return fmt.Errorf("could not create port forward %s", err)
	case <-readyChan:
		return nil
	}
}

// Get pod name to enable port forward
func (pf *PortForwardOpt) getPodName(c *k8s.Client) error {
	labelSelector := metav1.LabelSelector{
		MatchLabels: pf.MatchLabels,
	}

	podList, err := c.K8sClientset.CoreV1().Pods(pf.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&labelSelector),
	})
	if err != nil {
		return err
	}
	if len(podList.Items) == 0 {
		return fmt.Errorf("%s svc not found", pf.TargetSvc)
	}
	pf.PodName = podList.Items[0].GetObjectMeta().GetName()
	pf.Namespace = podList.Items[0].GetObjectMeta().GetNamespace()
	return nil
}

// Returns the local port for the port forwarder
func (pf *PortForwardOpt) getLocalPort() (int64, error) {
	for {
		port, err := getRandomPort()
		if err != nil {
			return port, err
		}
		listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.FormatInt(port, 10))
		if err == nil {
			if err := listener.Close(); err != nil {
				return -1, err
			}
			fmt.Fprintf(os.Stderr, "local port to be used for port forwarding %s: %d \n", pf.PodName, port)
			return port, nil
		}
	}
}

// Return a port number > 32767
func getRandomPort() (int64, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(32900-32768))
	if err != nil {
		return -1, errors.New("unable to generate random integer for port")
	}

	portNo := n.Int64() + 32768
	return portNo, nil
}
