package recommend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/kubearmor/kubearmor-client/recommend/common"
	"github.com/kubearmor/kubearmor-client/recommend/engines"
	"github.com/kubearmor/kubearmor-client/recommend/image"
	"github.com/kubearmor/kubearmor-client/recommend/registry"
	"github.com/kubearmor/kubearmor-client/recommend/report"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var options common.Options

// Deployment contains brief information about a k8s deployment
type Deployment struct {
	Name      string
	Namespace string
	Labels    LabelMap
	Images    []string
}

// LabelMap is an alias for map[string]string
type LabelMap = map[string]string

func labelSplitter(r rune) bool {
	return r == ':' || r == '='
}

func labelArrayToLabelMap(labels []string) LabelMap {
	labelMap := LabelMap{}
	for _, label := range labels {
		kvPair := strings.FieldsFunc(label, labelSplitter)
		if len(kvPair) != 2 {
			continue
		}
		labelMap[kvPair[0]] = kvPair[1]
	}
	return labelMap
}

func matchLabels(filter, selector LabelMap) bool {
	match := true
	for k, v := range filter {
		if selector[k] != v {
			match = false
			break
		}
	}
	return match
}

func unique(s []string) []string {
	inResult := make(map[string]bool)
	var result []string
	for _, str := range s {
		str = strings.Trim(str, " ")
		if _, ok := inResult[str]; !ok {
			inResult[str] = true
			result = append(result, str)
		}
	}
	return result
}

func createOutDir(dir string) error {
	if dir == "" {
		return nil
	}
	_, err := os.Stat(dir)
	if errors.Is(err, os.ErrNotExist) {
		err = os.Mkdir(dir, 0750)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

// Recommend handler for karmor cli tool
func Recommend(c *k8s.Client, o common.Options, policyGenerators ...engines.Engine) error {
	deployments := []Deployment{}

	labelMap := labelArrayToLabelMap(o.Labels)
	if len(o.Images) == 0 {
		// recommendation based on k8s manifest
		dps, err := c.K8sClientset.AppsV1().Deployments(o.Namespace).List(context.TODO(), v1.ListOptions{})
		if err != nil {
			return err
		}
		for _, dp := range dps.Items {

			if matchLabels(labelMap, dp.Spec.Template.Labels) {
				images := []string{}
				for _, container := range dp.Spec.Template.Spec.Containers {
					images = append(images, container.Image)
				}

				deployments = append(deployments, Deployment{
					Name:      dp.Name,
					Namespace: dp.Namespace,
					Labels:    dp.Spec.Template.Labels,
					Images:    images,
				})
			}
		}
	} else {
		deployments = append(deployments, Deployment{
			Namespace: o.Namespace,
			Labels:    labelMap,
			Images:    o.Images,
		})
	}

	o.Tags = unique(o.Tags)
	options = o
	_ = deployments
	reg := registry.New(o.Config)

	if err := createOutDir(o.OutDir); err != nil {
		return err
	}

	for _, gen := range policyGenerators {
		if o.ReportFile != "" {
			report.ReportInit(o.ReportFile)
		}
		gen.Init()
		for _, deployment := range deployments {
			for _, i := range deployment.Images {
				img := image.ImageInfo{
					Name:       i,
					Namespace:  deployment.Namespace,
					Labels:     deployment.Labels,
					Image:      i,
					Deployment: deployment.Name,
				}
				reg.Analyze(&img)
				gen.Scan(&img, o, []string{})
				err := report.ReportSectEnd()
				if err != nil {
					log.WithError(err).Error("report section end failed")
					return err
				}
			}
		}
		finalReport()
	}

	return nil
}

func finalReport() {
	repFile := filepath.Clean(filepath.Join(options.OutDir, options.ReportFile))
	_ = report.ReportRender(repFile)
	color.Green("output report in %s ...", repFile)
	if strings.Contains(repFile, ".html") {
		return
	}
	data, err := os.ReadFile(repFile)
	if err != nil {
		log.WithError(err).Fatal("failed to read report file")
		return
	}
	fmt.Println(string(data))
}
