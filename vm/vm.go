package vm

import (
	"context"
	"errors"
	"net"
	"os"

	"github.com/kubearmor/kubearmor-client/k8s"
	pb "github.com/kubearmor/kubearmor-client/protobuf"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type VmOptions struct {
	IP        string
	Port      string
	VMName    string
	File      string
	Namespace string
}

var (
	serviceAccountName = "kvmsoperator"
	pbClient           pb.HandleCliClient
)

func initGrpcClient(ip string) error {
	// Connect to gRPC server
	grpcClientConn, err := grpc.DialContext(context.Background(), net.JoinHostPort(ip, "32770"), grpc.WithInsecure())
	if err != nil {
		return err
	}

	pbClient = pb.NewHandleCliClient(grpcClientConn)
	if pbClient == nil {
		return errors.New("invalid grpc client handle")
	}
	return nil
}

func writeScriptDataToFile(options VmOptions, scriptData string) error {

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

	log.Printf("VM installation script copied to %s\n", filename)
	return nil
}

func getClusterIP(c *k8s.Client, options VmOptions) (string, error) {

	var clusterIP string

	svcList, err := c.K8sClientset.CoreV1().Services("all").List(context.Background(), metav1.ListOptions{
		FieldSelector: "metadata.name=" + serviceAccountName})
	if err != nil {
		return "", err
	}

	for _, svc := range svcList.Items {
		clusterIP = svc.Spec.ClusterIP
		break
	}

	if options.IP != "none" {
		return options.IP, err
	}

	return clusterIP, err
}

func validateInputParameters(options VmOptions) bool {

	if options.Namespace == "none" {
		log.Error().Msgf("provide a valid kvmsoperator service namespace")
		return false
	}

	if options.IP == "none" {
		log.Error().Msgf("provide a valid kvmsoperator service IP")
		return false
	}

	if options.Port == "none" {
		log.Error().Msgf("provide a valid kvmsoperator service port")
		return false
	}

	if options.VMName == "none" {
		log.Error().Msgf("provide a valid vm name")
		return false
	}

	return true
}

func FileDownload(c *k8s.Client, options VmOptions) error {

	if !validateInputParameters(options) {
		return errors.New("check input parameters")
	}

	// Check if kvmsoperator is up and running
	if _, err := c.K8sClientset.CoreV1().ServiceAccounts(options.Namespace).Get(context.Background(), serviceAccountName, metav1.GetOptions{}); err != nil {
		return err
	}

	clusterIP, err := getClusterIP(c, options)
	if err != nil || clusterIP == "" {
		return err
	}

	err = initGrpcClient(clusterIP)
	if err != nil {
		log.Error().Msgf("unable to connect to grpc server: %s", err.Error())
		return err
	}

	response, err := pbClient.HandleCliRequest(context.Background(), &pb.CliRequest{KvmName: options.VMName})
	if err != nil {
		return err
	} else {
		if response.Status == 0 {
			err = writeScriptDataToFile(options, response.ScriptData)
		} else {
			return errors.New(response.StatusMsg)
		}
	}

	return err
}
