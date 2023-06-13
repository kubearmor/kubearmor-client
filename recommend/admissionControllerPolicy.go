package recommend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/accuknox/auto-policy-discovery/src/libs"
	"github.com/accuknox/auto-policy-discovery/src/protobuf/v1/worker"
	"github.com/accuknox/auto-policy-discovery/src/types"
	"github.com/clarketm/json"
	"github.com/fatih/color"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/kubearmor/kubearmor-client/utils"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"sigs.k8s.io/yaml"
)

var connection *grpc.ClientConn

func initClientConnection(c *k8s.Client) error {
	if connection != nil {
		return nil
	}
	var err error
	connection, err = getClientConnection(c)
	if err != nil {
		return err
	}
	log.Info("Connected to discovery engine")
	return nil
}

func closeConnectionToDiscoveryEngine() {
	if connection != nil {
		err := connection.Close()
		if err != nil {
			log.Println("Error while closing connection")
		} else {
			log.Info("Connection to discovery engine closed successfully!")
		}
	}
}

func getClientConnection(c *k8s.Client) (*grpc.ClientConn, error) {
	gRPC := ""
	targetSvc := "discovery-engine"
	var port int64 = 9089
	mtchLabels := map[string]string{"app": "discovery-engine"}
	if val, ok := os.LookupEnv("DISCOVERY_SERVICE"); ok {
		gRPC = val
	} else {
		pf, err := utils.InitiatePortForward(c, port, port, mtchLabels, targetSvc)
		if err != nil {
			return nil, err
		}
		gRPC = "localhost:" + strconv.FormatInt(pf.LocalPort, 10)
	}
	// create a client
	conn, err := grpc.Dial(gRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, errors.New("could not connect to the server. Possible troubleshooting:\n- Check if discovery engine is running\n- Create a portforward to discovery engine service using\n\t\033[1mkubectl port-forward -n explorer service/knoxautopolicy --address 0.0.0.0 --address :: 9089:9089\033[0m\n[0m")
	}
	return conn, nil
}

func recommendAdmissionControllerPolicies(img ImageInfo) error {
	client := worker.NewWorkerClient(connection)
	labels := libs.LabelMapToString(img.Labels)
	resp, err := client.Convert(context.Background(), &worker.WorkerRequest{
		Labels:     labels,
		Namespace:  img.Namespace,
		Policytype: types.PolicyTypeAdmissionController,
	})
	if err != nil {
		color.Red(err.Error())
		return err
	}
	if resp.AdmissionControllerPolicy != nil {
		for _, policy := range resp.AdmissionControllerPolicy {
			var kyvernoPolicyInterface kyvernov1.PolicyInterface
			kyvernoPolicyInterface, err = getKyvernoPolicy(policy.Data)
			if err != nil {
				return err
			}
			if namespaceMatches(kyvernoPolicyInterface.GetNamespace()) && matchAdmissionControllerPolicyTags(kyvernoPolicyInterface.GetAnnotations()) {
				img.writeAdmissionControllerPolicy(kyvernoPolicyInterface)
			}
		}
	}
	return nil
}

func recommendGenericAdmissionControllerPolicies() error {
	client := worker.NewWorkerClient(connection)
	resp, err := client.Convert(context.Background(), &worker.WorkerRequest{
		Policytype: types.PolicyTypeAdmissionControllerGeneric,
	})
	if err != nil {
		color.Red(err.Error())
		return err
	}
	if resp.AdmissionControllerPolicy != nil {
		reportStarted := false
		for _, policy := range resp.AdmissionControllerPolicy {
			var kyvernoPolicyInterface kyvernov1.PolicyInterface
			kyvernoPolicyInterface, err = getKyvernoPolicy(policy.Data)
			if err != nil {
				if reportStarted {
					err := ReportSectEnd()
					if err != nil {
						return err
					}
				}
				return err
			}
			if matchAdmissionControllerPolicyTags(kyvernoPolicyInterface.GetAnnotations()) {
				if !reportStarted {
					err := ReportStartGenericAdmissionControllerPolicies()
					if err != nil {
						return err
					}
					reportStarted = true
				}
				writeGenericAdmissionControllerPolicy(kyvernoPolicyInterface)
			}
		}
		if reportStarted {
			err := ReportSectEnd()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func matchAdmissionControllerPolicyTags(policyAnnotations map[string]string) bool {
	policyTags := strings.Split(policyAnnotations[types.RecommendedPolicyTagsAnnotation], ",")
	if len(options.Tags) <= 0 {
		return true
	}
	for _, t := range options.Tags {
		if slices.Contains(policyTags, t) {
			return true
		}
	}
	return false
}

func namespaceMatches(policyNamespace string) bool {
	return options.Namespace == "" || options.Namespace == policyNamespace
}

func getKyvernoPolicy(policyYaml []byte) (kyvernov1.PolicyInterface, error) {
	var policy map[string]interface{}
	err := yaml.Unmarshal(policyYaml, &policy)
	if err != nil {
		return nil, err
	}
	policyKind := policy["kind"].(string)

	var kyvernoPolicyInterface kyvernov1.PolicyInterface
	switch policyKind {
	case "Policy":
		var kyvernoPolicy kyvernov1.Policy
		err = yaml.Unmarshal(policyYaml, &kyvernoPolicy)
		if err != nil {
			return nil, err
		}
		kyvernoPolicyInterface = &kyvernoPolicy
	case "ClusterPolicy":
		var kyvernoClusterPolicy kyvernov1.ClusterPolicy
		err = yaml.Unmarshal(policyYaml, &kyvernoClusterPolicy)
		if err != nil {
			return nil, err
		}
		kyvernoPolicyInterface = &kyvernoClusterPolicy
	default:
		return nil, fmt.Errorf("unexpected policy kind: %s", policyKind)
	}
	return kyvernoPolicyInterface, nil
}

func convertKyvernoPolicyInterfaceToJSON(policyInterface kyvernov1.PolicyInterface) ([]byte, error) {
	var jsonBytes []byte
	var err error
	switch policyInterface.(type) {
	case *kyvernov1.ClusterPolicy:
		kyvernoClusterPolicy := policyInterface.(*kyvernov1.ClusterPolicy)
		jsonBytes, err = json.Marshal(*kyvernoClusterPolicy)
		if err != nil {
			log.WithError(err).Error("json marshal failed")
			return nil, err
		}
	case *kyvernov1.Policy:
		kyvernoPolicy := policyInterface.(*kyvernov1.Policy)
		jsonBytes, err = json.Marshal(*kyvernoPolicy)
		if err != nil {
			log.WithError(err).Error("json marshal failed")
			return nil, err
		}
	}
	return jsonBytes, nil
}
