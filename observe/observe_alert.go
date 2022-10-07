package observe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	proto "github.com/kubearmor/koach/protobuf"
	"google.golang.org/grpc"
)

type AlertOptions struct {
	Namespace string
	Pod       string
	Container string
	JSON      bool
	GRPC      string
}

func StartObserveAlert(args []string, options AlertOptions) error {
	gRPC := "localhost:3001"

	if options.GRPC != "" {
		gRPC = options.GRPC
	}

	conn, err := grpc.Dial(gRPC, grpc.WithInsecure())
	if err != nil {
		return errors.New("could not connect to the server. Possible troubleshooting:\n- Check if koach server is running\n- Create a portforward to koach service using\n\t\033[1mkubectl port-forward -n kube-system service/koach --address 0.0.0.0 --address :: 3001:3001\033[0m\n- Configure grpc server information using\n\t\033[1mkarmor observe alert --grpc <info>\033[0m")
	}

	client := proto.NewObservabilityServiceClient(conn)

	stream, err := client.ListenAlert(context.Background(), &proto.ListenAlertRequest{
		NamespaceId: options.Namespace,
		PodId:       options.Pod,
		ContainerId: options.Container,
	})
	if err != nil {
		return errors.New("could not connect to the server. Possible troubleshooting:\n- Check if koach server is running\n- Create a portforward to koach service using\n\t\033[1mkubectl port-forward -n kube-system service/koach --address 0.0.0.0 --address :: 3001:3001\033[0m\n- Configure grpc server information using\n\t\033[1mkarmor observe --grpc <info>\033[0m")
	}

	for {
		resp, err := stream.Recv()
		if err != nil {
			break
		}

		timestamp, _ := time.Parse("2006-01-02 15:04:05.999999 -0700 MST", resp.Observability.CreatedAt)

		if options.JSON {
			arr, _ := json.Marshal(resp)
			fmt.Println(string(arr))
		} else {
			fmt.Printf("=== Alert / %s ===\n", timestamp.Format("2006-01-02 15:04:05"))
			fmt.Println("Message:", resp.Message)
			fmt.Println("Severity:", resp.Severity)
			fmt.Println("ClusterName:", resp.Observability.ClusterName)
			fmt.Println("HostName:", resp.Observability.HostName)
			fmt.Println("NamespaceName:", resp.Observability.NamespaceName)
			fmt.Println("PodName:", resp.Observability.PodName)
			fmt.Println("Labels:", resp.Observability.Labels)
			fmt.Println("ContainerID:", resp.Observability.ContainerId)
			fmt.Println("ContainerName:", resp.Observability.ContainerName)
			fmt.Println("ContainerImage:", resp.Observability.ContainerImage)
			fmt.Println("ParentProcessName:", resp.Observability.ParentProcessName)
			fmt.Println("ProcessName:", resp.Observability.ProcessName)
			fmt.Println("HostPPID:", resp.Observability.HostPpid)
			fmt.Println("HostPID:", resp.Observability.HostPid)
			fmt.Println("PPID:", resp.Observability.Ppid)
			fmt.Println("PID:", resp.Observability.Pid)
			fmt.Println("UID:", resp.Observability.Uid)
			fmt.Println("Type:", resp.Observability.Type)
			fmt.Println("Source:", resp.Observability.Source)
			fmt.Println("Operation:", resp.Observability.Operation)
			fmt.Println("Resource:", resp.Observability.Resource)
			fmt.Println("Data:", resp.Observability.Data)
			fmt.Println("Result:", resp.Observability.Result)
		}
	}

	return nil
}
