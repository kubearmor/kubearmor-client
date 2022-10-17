package utils

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kubearmor/kubearmor-client/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// Port forward details for a pod
type PortForwardOpt struct {
	LocalPort   int
	RemotePort  int
	MatchLabels map[string]string
	Namespace   string
	PodName     string
}

// Initialize port forwarding for kubearmor-relay pod
func InitiatePortForward(c *k8s.Client, localPort int, remotePort int, matchLabels map[string]string) (PortForwardOpt, error) {
	pf := PortForwardOpt{
		LocalPort:   localPort,
		RemotePort:  remotePort,
		Namespace:   "",
		MatchLabels: matchLabels,
	}

	// handle port forward
	err := pf.HandlePortForward(c)
	if err != nil {
		return pf, err
	}
	return pf, nil
}

// handle port forward to allow grpc to connect at localhost:32767
func (pf *PortForwardOpt) HandlePortForward(c *k8s.Client) error {
	if err := pf.getPodName(c); err != nil {
		return err
	}

	// local port
	pf.LocalPort = pf.GetLocalPort()

	err := k8sPortForward(c, *pf)
	if err != nil {
		fmt.Printf("could not do kubearmor portforward, Error=%s", err.Error())
		return err
	}
	return nil

}

// k8s port forward
func k8sPortForward(c *k8s.Client, pf PortForwardOpt) error {
	roundTripper, upgrader, err := spdy.RoundTripperFor(c.Config)
	if err != nil {
		fmt.Printf("unable to spdy.RoundTripperFor error=%s", err.Error())
		return err
	}

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", pf.Namespace, pf.PodName)
	hostIP := strings.TrimLeft(c.Config.Host, "https:/")
	serverURL := url.URL{Scheme: "https", Path: path, Host: hostIP}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, &serverURL)

	StopChan, readyChan := make(chan struct{}, 1), make(chan struct{}, 1)
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)

	forwarder, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", pf.LocalPort, pf.RemotePort)},
		StopChan, readyChan, out, errOut)
	if err != nil {
		fmt.Printf("unable to portforward. error=%s", err.Error())
		return err
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
		fmt.Println("Kubearmor Pod not found")
		err = fmt.Errorf("kubearmor pod not found")
		return err
	}
	pf.PodName = podList.Items[0].GetObjectMeta().GetName()
	pf.Namespace = podList.Items[0].GetObjectMeta().GetNamespace()
	return nil
}

// Returns the local port for the port forwarder
func (pf *PortForwardOpt) GetLocalPort() int {
	port := pf.LocalPort

	for {
		listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
		if err == nil {
			listener.Close()
			fmt.Printf("local port to be used for port forwarding %s: %d \n", pf.PodName, port)
			return port
		}
		rand.Seed(time.Now().UnixNano())
		port = rand.Intn(32900-32768+1) + 32768
	}
}
