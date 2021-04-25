package collectors

import (
	"errors"
	"fmt"
	"strings"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/kube-state-metrics/pkg/metric"
)

const (
	CLUSTER_CPU_UTILIZATION_PSQL    = `1-avg(rate(node_cpu_seconds_total{mode="idle"}[5m]))`
	CLUSTER_MEMORY_UTILIZATION_PSQL = `1 - sum(:node_memory_MemAvailable_bytes:sum{}) / sum(kube_node_status_allocatable_memory_bytes{})`
)

var (
	clusterResourceMetricFamilies = []metric.FamilyGenerator{
		metric.FamilyGenerator{
			Name: "board_cluster_cpu_utilization",
			Type: metric.MetricTypeGauge,
			Help: "Cluster CPU Utilization",
			GenerateFunc: func(obj interface{}) metric.Family {
				c := obj.(*ClusterValue)
				f := metric.Family{}
				f.Metrics = append(f.Metrics, c.CPU)

				return f
			},
		},
		metric.FamilyGenerator{
			Name: "board_cluster_memory_utilization",
			Type: metric.MetricTypeGauge,
			Help: "Cluster Memory Utilization",
			GenerateFunc: func(obj interface{}) metric.Family {
				c := obj.(*ClusterValue)
				f := metric.Family{}
				f.Metrics = append(f.Metrics, c.Memory)

				return f
			},
		},
	}
)

type ClusterValue struct {
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	CPU               *metric.Metric
	Memory            *metric.Metric
}

func getClusterResourceMetrics(c context.Context, apiserver string, kubeconfig string, namespaces []string, url string) []interface{} {
	papi, err := getPrometheusAPI(url)
	if err != nil {
		klog.Infof("New prometheus client error: %+v", err)
		return nil
	}
	cpu, err := getSingleSampleValue(c, papi, CLUSTER_CPU_UTILIZATION_PSQL)
	if err != nil {
		cpu = 0
	}
	//cluster cpu metric
	cpumetric := metric.Metric{
		Value: cpu,
	}
	memory, err := getSingleSampleValue(c, papi, CLUSTER_MEMORY_UTILIZATION_PSQL)
	if err != nil {
		memory = 0
	}
	//cluster memory metric
	memorymetric := metric.Metric{
		Value: memory,
	}

	value := ClusterValue{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID("cluster_utilization"),
		},
		CPU:    &cpumetric,
		Memory: &memorymetric,
	}
	return []interface{}{&value}
}

func getSingleSampleValue(c context.Context, papi v1.API, psql string) (float64, error) {
	values, warns, err := papi.Query(c, psql, time.Now())
	if err != nil {
		klog.Infof("Query prometheus metric %s error: %+v", psql, err)
		return 0, err
	}
	if len(warns) != 0 {
		klog.Infof("Query prometheus metric %s warns: %s", psql, strings.Join(warns, " "))
	}
	if values.Type() != model.ValVector {
		klog.Infof("Query prometheus metric %s type %s is not vector", psql, values.Type().String())
		return 0, errors.New(fmt.Sprintf("Query prometheus metric %s type %s is not vector", psql, values.Type().String()))
	}
	vec := values.(model.Vector)
	if len(vec) == 0 {
		klog.Infof("Query prometheus metric %s has no result.", psql)
		return 0, nil
	}

	if len(vec) > 1 {
		for _, sample := range vec {
			klog.Infof("Query prometheus metric %s result multi result: %+v", psql, sample)
		}
	}
	return float64(vec[0].Value), nil
}
