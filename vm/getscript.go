package vm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/kubearmor/kubearmor-client/k8s"
	pb "github.com/kubearmor/kubearmor-client/vm/protobuf"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Options for karmor vm getscript
type ScriptOptions struct {
	Port   string
	VMName string
	File   string
}

var (
	serviceAccountName = "kvmservice"
	pbClient           pb.HandleCliClient
	namespace          string
)

func initGrpcClient(ip string, port string) error {
	// Connect to gRPC server
	grpcClientConn, err := grpc.DialContext(context.Background(), net.JoinHostPort(ip, port), grpc.WithInsecure())
	if err != nil {
		return err
	}

	pbClient = pb.NewHandleCliClient(grpcClientConn)
	if pbClient == nil {
		return errors.New("invalid grpc client handle")
	}
	return nil
}

func writeScriptDataToFile(options ScriptOptions, scriptData string) error {

	var filename string

	if options.File == "none" {
		filename = options.VMName + ".sh"
	} else {
		filename = options.File
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	_, err = file.WriteString(scriptData)
	if err != nil {
		return err
	}

	fmt.Printf("VM installation script copied to %s\n", filename)
	return nil
}

func getClusterIP(c *k8s.Client, options ScriptOptions) (string, error) {

	var externalIP string

	svcInfo, err := c.K8sClientset.CoreV1().Services(namespace).Get(context.Background(), serviceAccountName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	for _, lbIngress := range svcInfo.Status.LoadBalancer.Ingress {
		externalIP = lbIngress.IP
		break
	}

	return externalIP, err
}

// Function to handle script download for vm option
func GetScript(c *k8s.Client, options ScriptOptions) error {

	var clusterIP string

	// Get the list of namespaces in kubernetes context
	namespaces, err := c.K8sClientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		// Consider as kubectl is not configured
		clusterIP = "127.0.0.1"
	} else {
		for _, ns := range namespaces.Items {
			// Fetch the namespace of kvmservice
			if _, err := c.K8sClientset.CoreV1().ServiceAccounts(ns.Name).Get(context.Background(), serviceAccountName, metav1.GetOptions{}); err != nil {
				continue
			}
			namespace = ns.Name
			break
		}

		clusterIP, err := getClusterIP(c, options)
		if err != nil || clusterIP == "" {
			return err
		}
	}

	err = initGrpcClient(clusterIP, options.Port)
	if err != nil {
		log.Error().Msgf("unable to connect to grpc server: %s", err.Error())
		return err
	}

	response, err := pbClient.HandleCliRequest(context.Background(), &pb.CliRequest{KvmName: options.VMName})
	if err != nil {
		return err
	}

	if response.Status == 0 {
		err = writeScriptDataToFile(options, response.ScriptData)
	} else {
		return errors.New(response.StatusMsg)
	}

	return err
}
