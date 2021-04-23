package collectors

import (
	"regexp"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kube-state-metrics/pkg/metric"
	"k8s.io/kube-state-metrics/pkg/version"
)

var (
	resyncPeriod = 5 * time.Minute

	ScrapeErrorTotalMetric = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ksm_scrape_error_total",
			Help: "Total scrape errors encountered when scraping a resource",
		},
		[]string{"resource"},
	)

	ResourcesPerScrapeMetric = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "ksm_resources_per_scrape",
			Help: "Number of resources returned per scrape",
		},
		[]string{"resource"},
	)

	invalidLabelCharRE = regexp.MustCompile(`[^a-zA-Z0-9_]`)
)

func boolFloat64(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func kubeLabelsToPrometheusLabels(labels map[string]string) ([]string, []string) {
	labelKeys := make([]string, len(labels))
	labelValues := make([]string, len(labels))
	i := 0
	for k, v := range labels {
		labelKeys[i] = "label_" + sanitizeLabelName(k)
		labelValues[i] = v
		i++
	}
	return labelKeys, labelValues
}

func sanitizeLabelName(s string) string {
	return invalidLabelCharRE.ReplaceAllString(s, "_")
}

func getPrometheusAPI(url string) (v1.API, error) {
	client, err := api.NewClient(api.Config{
		Address: url,
	})
	if err != nil {
		return nil, err
	}
	return v1.NewAPI(client), nil
}

func convertPromMetricToKSMMetric(pm model.Metric) metric.Metric {
	keys := make([]string, len(pm), len(pm))
	values := make([]string, len(pm), len(pm))
	index := 0
	for k, v := range pm {
		keys[index], values[index] = string(k), string(v)
		index++
	}
	return metric.Metric{
		LabelKeys:   keys,
		LabelValues: values,
	}
}

func createk8sClient(apiserver string, kubeconfig string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags(apiserver, kubeconfig)
	if err != nil {
		return nil, err
	}

	config.UserAgent = version.GetVersion().String()
	config.AcceptContentTypes = "application/vnd.kubernetes.protobuf,application/json"
	config.ContentType = "application/vnd.kubernetes.protobuf"

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	return clientset, err

}

// split the ip:port to ip and map the ip to name.
func getMapString(addr string, mapping map[string]string) string {
	if mapping == nil {
		return addr
	}
	addres := strings.Split(addr, ":")
	if len(addres) < 2 {
		return addr
	}
	// map addr[0] ---> name.
	if name, ok := mapping[addres[0]]; ok {
		return name
	}
	return addr
}

func addMetricNodenameBaseOnInstance(source []*metric.Metric, mapping map[string]string) []*metric.Metric {
	if mapping == nil {
		return source
	}

	for _, s := range source {
		index := -1
		for i, label := range s.LabelKeys {
			if label == "instance" {
				index = i
				break
			}
		}
		if index != -1 {
			s.LabelKeys = append(s.LabelKeys, "nodename_for_board")
			s.LabelValues = append(s.LabelValues, getMapString(s.LabelValues[index], mapping))
		}
	}
	return source
}
