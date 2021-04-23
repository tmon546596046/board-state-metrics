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
	NODE_CPU_UTILIZATION_PSQL     = `sum by (instance) (instance:node_cpu_utilisation:rate1m{job="node-exporter"})`
	NODE_MEMORY_UTILIZATION_PSQL  = `sum by (instance) (instance:node_memory_utilisation:ratio{job="node-exporter"})`
	NODE_STORAGE_UTILIZATION_PSQL = `1 -(sum by (instance) (node_filesystem_avail_bytes{job="node-exporter", fstype!="", device!=""})/sum by(instance) (node_filesystem_size_bytes{job="node-exporter", fstype!="", device!=""}))`
)

var (
	nodeResourceMetricFamilies = []metric.FamilyGenerator{
		metric.FamilyGenerator{
			Name: "board_node_cpu_utilization",
			Type: metric.MetricTypeGauge,
			Help: "Node CPU Utilization",
			GenerateFunc: func(obj interface{}) metric.Family {
				c := obj.(*NodeValues)
				f := metric.Family{}
				for _, m := range c.CPU {
					f.Metrics = append(f.Metrics, m)
				}

				return f
			},
		},
		metric.FamilyGenerator{
			Name: "board_node_memory_utilization",
			Type: metric.MetricTypeGauge,
			Help: "Node Memory Utilization",
			GenerateFunc: func(obj interface{}) metric.Family {
				c := obj.(*NodeValues)
				f := metric.Family{}
				for _, m := range c.Memory {
					f.Metrics = append(f.Metrics, m)
				}

				return f
			},
		},
		metric.FamilyGenerator{
			Name: "board_node_storage_utilization",
			Type: metric.MetricTypeGauge,
			Help: "Node Storage Utilization",
			GenerateFunc: func(obj interface{}) metric.Family {
				c := obj.(*NodeValues)
				f := metric.Family{}
				for _, m := range c.Storage {
					f.Metrics = append(f.Metrics, m)
				}

				return f
			},
		},
	}
)

type NodeValues struct {
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	CPU               []*metric.Metric
	Memory            []*metric.Metric
	Storage           []*metric.Metric
}

func getNodeResourceMetrics(c context.Context, apiserver string, kubeconfig string, namespaces []string, url string) []interface{} {
	papi, err := getPrometheusAPI(url)
	if err != nil {
		klog.Infof("New prometheus client error: %+v", err)
		return nil
	}
	//create k8s client
	k8scli, err := createk8sClient(apiserver, kubeconfig)
	if err != nil {
		klog.Infof("New kubernetes client error: %+v", err)
		return nil
	}
	nodes, err := k8scli.CoreV1().Nodes().List(c, metav1.ListOptions{})
	if err != nil {
		klog.Infof("List kubernetes nodes error: %+v", err)
		return nil
	}
	nodeMap := map[string]string{}
	if nodes != nil {
		for _, n := range nodes.Items {
			for _, addr := range n.Status.Addresses {
				nodeMap[addr.Address] = n.Name
			}
		}
	}

	//ignore the errors.
	cpus, _ := getMultiSampleValues(c, papi, NODE_CPU_UTILIZATION_PSQL)
	//node memory metric
	memorys, _ := getMultiSampleValues(c, papi, NODE_MEMORY_UTILIZATION_PSQL)
	storages, _ := getMultiSampleValues(c, papi, NODE_STORAGE_UTILIZATION_PSQL)
	nodememorys := NodeValues{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID("node_memory_utilization"),
		},
		CPU:     addMetricNodenameBaseOnInstance(cpus, nodeMap),
		Memory:  addMetricNodenameBaseOnInstance(memorys, nodeMap),
		Storage: addMetricNodenameBaseOnInstance(storages, nodeMap),
	}
	return []interface{}{&nodememorys}
}

func getMultiSampleValues(c context.Context, papi v1.API, psql string) ([]*metric.Metric, error) {
	values, warns, err := papi.Query(c, psql, time.Now())
	if err != nil {
		klog.Infof("Query prometheus metric %s error: %+v", psql, err)
		return nil, err
	}
	if len(warns) != 0 {
		klog.Infof("Query prometheus metric %s warns: %s", psql, strings.Join(warns, " "))
	}
	if values.Type() != model.ValVector {
		klog.Infof("Query prometheus metric %s type %s is not vector", psql, values.Type().String())
		return nil, errors.New(fmt.Sprintf("Query prometheus metric %s type %s is not vector", psql, values.Type().String()))
	}
	vec := values.(model.Vector)

	result := make([]*metric.Metric, len(vec), len(vec))
	for i, v := range vec {
		m := convertPromMetricToKSMMetric(v.Metric)
		m.Value = float64(v.Value)
		result[i] = &m
	}
	return result, nil
}
