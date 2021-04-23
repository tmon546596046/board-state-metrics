package collectors

import (
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kube-state-metrics/pkg/collector"
	"k8s.io/kube-state-metrics/pkg/metric"
	metricsstore "k8s.io/kube-state-metrics/pkg/metrics_store"
	"k8s.io/kube-state-metrics/pkg/options"

	"k8s.io/client-go/tools/cache"

	"golang.org/x/net/context"
	"k8s.io/klog/v2"
)

type whiteBlackLister interface {
	IsIncluded(string) bool
	IsExcluded(string) bool
}

// Builder helps to build collectors. It follows the builder pattern
// (https://en.wikipedia.org/wiki/Builder_pattern).
type Builder struct {
	apiserver         string
	kubeconfig        string
	namespaces        options.NamespaceList
	prometheus        string
	ctx               context.Context
	enabledCollectors []string
	whiteBlackList    whiteBlackLister
}

// NewBuilder returns a new builder.
func NewBuilder(
	ctx context.Context,
) *Builder {
	return &Builder{
		ctx: ctx,
	}
}

func (b *Builder) WithApiserver(apiserver string) *Builder {
	b.apiserver = apiserver
	return b
}

func (b *Builder) WithKubeConfig(kubeconfig string) *Builder {
	b.kubeconfig = kubeconfig
	return b
}

// WithEnabledCollectors sets the enabledCollectors property of a Builder.
func (b *Builder) WithEnabledCollectors(c []string) *Builder {
	copy := []string{}
	for _, s := range c {
		copy = append(copy, s)
	}

	sort.Strings(copy)

	b.enabledCollectors = copy
	return b
}

// WithNamespaces sets the namespaces property of a Builder.
func (b *Builder) WithNamespaces(n options.NamespaceList) *Builder {
	b.namespaces = n
	return b
}

// WithPrometheus sets the prometheus property of a Builder.
func (b *Builder) WithPrometheus(prom string) *Builder {
	b.prometheus = prom
	return b
}

// WithWhiteBlackList configures the white or blacklisted metrics to be exposed
// by the collectors build by the Builder
func (b *Builder) WithWhiteBlackList(l whiteBlackLister) *Builder {
	b.whiteBlackList = l
	return b
}

// Build initializes and registers all enabled collectors.
func (b *Builder) Build() []*collector.Collector {
	if b.whiteBlackList == nil {
		panic("whiteBlackList should not be nil")
	}

	collectors := []*collector.Collector{}
	activeCollectorNames := []string{}

	// for _, c := range b.enabledCollectors {
	// 	constructor, ok := availableCollectors[c]
	// 	if !ok {
	// 		klog.Fatalf("collector %s is not correct", c)
	// 	}

	// 	collector := constructor(b)
	// 	activeCollectorNames = append(activeCollectorNames, c)
	// 	collectors = append(collectors, collector)

	// }

	//ignore the configuration, we enable all the metrics.
	for c, constructor := range availableCollectors {
		collector := constructor(b)
		activeCollectorNames = append(activeCollectorNames, c)
		collectors = append(collectors, collector)

	}

	klog.Infof("Active collectors: %s", strings.Join(activeCollectorNames, ","))

	return collectors
}

var availableCollectors = map[string]func(f *Builder) *collector.Collector{
	"clusterresource": func(b *Builder) *collector.Collector { return b.buildClusterResourceCollector() },
	"noderesource":    func(b *Builder) *collector.Collector { return b.buildNodeResourceCollector() },
}

func (b *Builder) buildClusterResourceCollector() *collector.Collector {
	filteredMetricFamilies := metric.FilterMetricFamilies(b.whiteBlackList, clusterResourceMetricFamilies)
	composedMetricGenFuncs := metric.ComposeMetricGenFuncs(filteredMetricFamilies)

	familyHeaders := metric.ExtractMetricFamilyHeaders(filteredMetricFamilies)

	store := metricsstore.NewMetricsStore(
		familyHeaders,
		composedMetricGenFuncs,
	)
	retriveFromProm(b.ctx, store, b.apiserver, b.kubeconfig, b.namespaces, b.prometheus, getClusterResourceMetrics)

	return collector.NewCollector(store)
}

func (b *Builder) buildNodeResourceCollector() *collector.Collector {
	filteredMetricFamilies := metric.FilterMetricFamilies(b.whiteBlackList, nodeResourceMetricFamilies)
	composedMetricGenFuncs := metric.ComposeMetricGenFuncs(filteredMetricFamilies)

	familyHeaders := metric.ExtractMetricFamilyHeaders(filteredMetricFamilies)

	store := metricsstore.NewMetricsStore(
		familyHeaders,
		composedMetricGenFuncs,
	)
	retriveFromProm(b.ctx, store, b.apiserver, b.kubeconfig, b.namespaces, b.prometheus, getNodeResourceMetrics)

	return collector.NewCollector(store)
}

// reflectorPerNamespace creates a Kubernetes client-go reflector with the given
// listWatchFunc for each given namespace and registers it with the given store.
func reflectorPerNamespace(
	ctx context.Context,
	expectedType interface{},
	store cache.Store,
	apiserver string,
	kubeconfig string,
	namespaces []string,
	listWatchFunc func(apiserver string, kubeconfig string, ns string) cache.ListWatch,
) {
	for _, ns := range namespaces {
		lw := listWatchFunc(apiserver, kubeconfig, ns)
		reflector := cache.NewReflector(&lw, expectedType, store, 0)
		go reflector.Run(ctx.Done())
	}
}

// retriveFromProm query the psql from prometheus and then add the result to store.
func retriveFromProm(ctx context.Context, store cache.Store, apiserver string, kubeconfig string, namespaces []string, promurl string, query func(c context.Context, apiserver string, kubeconfig string, namespaces []string, url string) []interface{}) {
	go wait.Forever(func() {
		objs := query(ctx, apiserver, kubeconfig, namespaces, promurl)
		for _, obj := range objs {
			store.Update(obj)
		}
	}, 15*time.Second)
}
