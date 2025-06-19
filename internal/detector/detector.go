package detector

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	monitoringv1 "github.com/danekeahi/aks-health-guard/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	containerservice "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kubeclientcmd "k8s.io/client-go/tools/clientcmd"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

const (
	subscriptionID    = "8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8"
	tenantID          = "72f988bf-86f1-41af-91ab-2d7cd011db47"
	resourceGroupName = "aks-health-rg"
	resourceName      = "aks-health-cluster"
)

var (
	kubeClient     *kubernetes.Clientset
	restConfig     *rest.Config
	kubeClientOnce sync.Once
)

// PodMetrics holds the metrics for a pod, including CPU and memory usage, restart count, and health status
type PodMetrics struct {
	PodName         string
	Namespace       string
	CPUUsage        string
	MemoryUsage     string
	RestartCount    int32
	IsCrashed       bool
	PendingDuration time.Duration
}

// NewDetectorRunnable creates a new Runnable that periodically checks the health of workloads
func NewDetectorRunnable(c client.Client) manager.Runnable {
	return manager.RunnableFunc(func(ctx context.Context) error {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				checkAndUpdateWorkloadHealth(c)
			}
		}
	})
}

// checkAndUpdateWorkloadHealth checks the health of workloads and updates their status in the cluster
func checkAndUpdateWorkloadHealth(c client.Client) {
	ctx := context.Background()
	var workloadList monitoringv1.WorkloadList

	// Initialize Kubernetes and Metrics clients
	kubeClient, restConfig, err := getKubeClient()
	if err != nil {
		fmt.Println("Failed to get Kubernetes client:", err)
		return
	}
	fmt.Println("Successfully connected to Kubernetes cluster")

	metricsClient, err := getMetricsClient(restConfig)
	if err != nil {
		fmt.Println("Failed to create metrics client:", err)
		return
	}

	metrics, err := getPodMetrics(kubeClient, metricsClient, "default")
	if err != nil {
		fmt.Println("Error gathering pod metrics:", err)
		return
	}

	// List all workloads of kind Workload
	if err := c.List(ctx, &workloadList); err != nil {
		fmt.Println("Failed to list workloads:", err)
		return
	}

	// Check each workload and update its health status
	for _, wl := range workloadList.Items {
		isHealthy := checkPodHealth(metrics, wl.Spec.JobName, wl.Spec.Thresholds)
		if wl.Status.Health != isHealthy {
			wl.Status.Health = isHealthy
			if err := c.Status().Update(ctx, &wl); err != nil {
				fmt.Println("Failed to update workload health:", err)
			} else {
				fmt.Printf("Updated %s health to %v\n", wl.Name, isHealthy)
			}
		}
	}
}

// getMetricsClient returns a new metrics client
func getMetricsClient(config *rest.Config) (*metricsclient.Clientset, error) {
	return metricsclient.NewForConfig(config)
}

// getKubeClient initializes and returns a Kubernetes clientset using Azure credentials
func getKubeClient() (*kubernetes.Clientset, *rest.Config, error) {
	var err error
	kubeClientOnce.Do(func() {
		cred, e := azidentity.NewAzureCLICredential(&azidentity.AzureCLICredentialOptions{TenantID: tenantID})
		if e != nil {
			err = e
			return
		}

		mcClient, e := containerservice.NewManagedClustersClient(subscriptionID, cred, nil)
		if e != nil {
			err = e
			return
		}

		credentials, e := mcClient.ListClusterAdminCredentials(context.Background(), resourceGroupName, resourceName, nil)
		if e != nil {
			err = e
			return
		}

		clientConfig, e := kubeclientcmd.NewClientConfigFromBytes(credentials.Kubeconfigs[0].Value)
		if e != nil {
			err = e
			return
		}

		restConfig, e = clientConfig.ClientConfig()
		if e != nil {
			err = e
			return
		}

		kubeClient, err = kubernetes.NewForConfig(restConfig)
	})

	return kubeClient, restConfig, err
}

// getPodMetrics collects metrics and basic health data for all pods in a namespace
func getPodMetrics(kubeClient *kubernetes.Clientset, metricsClient *metricsclient.Clientset, namespace string) ([]PodMetrics, error) {
	ctx := context.Background()
	var results []PodMetrics

	podList, err := kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	metricsList, err := metricsClient.MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	metricsMap := make(map[string]metricsv1beta1.PodMetrics)
	for _, m := range metricsList.Items {
		metricsMap[m.Name] = m
	}

	now := time.Now()
	for _, pod := range podList.Items {
		m := PodMetrics{
			PodName:      pod.Name,
			Namespace:    pod.Namespace,
			IsCrashed:    pod.Status.Phase == corev1.PodFailed,
			RestartCount: 0,
		}

		// Aggregate restart count and check for crash-loop
		for _, cs := range pod.Status.ContainerStatuses {
			m.RestartCount += cs.RestartCount
			if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
				m.IsCrashed = true
			}
		}

		// Capture pending duration
		if pod.Status.Phase == corev1.PodPending {
			m.PendingDuration = now.Sub(pod.CreationTimestamp.Time)
		}

		// Extract CPU and memory usage
		if podMetrics, exists := metricsMap[pod.Name]; exists {
			for _, container := range podMetrics.Containers {
				m.CPUUsage = container.Usage.Cpu().String()
				m.MemoryUsage = container.Usage.Memory().String()
				break
			}
		}

		results = append(results, m)
	}

	return results, nil
}

// checkPodHealth evaluates the workload health by checking metrics against thresholds
func checkPodHealth(metrics []PodMetrics, jobName string, thresholds monitoringv1.Thresholds) bool {
	fmt.Println("Checking health for job:", jobName)

	totalCrashed := 0

	for _, m := range metrics {
		// Only check pods that match the given job name (inferred by name substring)
		if !strings.Contains(m.PodName, jobName) {
			continue
		}

		// CPU usage check
		if m.CPUUsage != "" && thresholds.CPUUsageNano > 0 {
			cpuQty, err := resource.ParseQuantity(m.CPUUsage)
			if err == nil && cpuQty.MilliValue()*1_000_000 > thresholds.CPUUsageNano {
				fmt.Printf("Pod %s CPU usage too high: %s\n", m.PodName, m.CPUUsage)
				return false
			}
		}

		// Memory usage check
		if m.MemoryUsage != "" && thresholds.MemoryUsageBytes > 0 {
			memQty, err := resource.ParseQuantity(m.MemoryUsage)
			if err == nil && memQty.Value() > thresholds.MemoryUsageBytes {
				fmt.Printf("Pod %s memory usage too high: %s\n", m.PodName, m.MemoryUsage)
				return false
			}
		}

		// Restart count check
		if thresholds.MaxRestartCount > 0 && m.RestartCount > thresholds.MaxRestartCount {
			fmt.Printf("Pod %s has too many restarts: %d\n", m.PodName, m.RestartCount)
			return false
		}

		// Pending duration check
		if m.PendingDuration > 0 && thresholds.MaxPendingTime.Duration > 0 && m.PendingDuration > thresholds.MaxPendingTime.Duration {
			fmt.Printf("Pod %s has been pending too long: %v\n", m.PodName, m.PendingDuration)
			return false
		}

		// Crashed pods number check
		if m.IsCrashed {
			totalCrashed++
			if thresholds.MaxCrashedPods > 0 && totalCrashed > thresholds.MaxCrashedPods {
				fmt.Printf("Too many crashed pods: %d > %d\n", totalCrashed, thresholds.MaxCrashedPods)
				return false
			}
		}
	}

	return true
}
