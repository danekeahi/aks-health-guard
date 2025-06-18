package detector

import (
	"context"
	"fmt"
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

// look into managed identity
// service principal
// specify which permission it has access to

// service principal id and password (as a CRD property?)

// helm chart
// block storage
// extension RP to enable extension
// fetch helm chart
// deploy it
// there'll be an extension manager to handle things in background

const (
	subscriptionID    = "8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8"
	tenantID          = "72f988bf-86f1-41af-91ab-2d7cd011db47"
	resourceGroupName = "aks-health-rg"
	resourceName      = "aks-health-cluster"

	MaxRestartCount     = 10
	MaxPendingDuration  = 1 * time.Minute
	MaxCPUUsageNano     = 200_000_000       // 200m = 0.2 cores
	MaxMemoryUsageBytes = 200 * 1024 * 1024 // 200Mi
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
	CPUUsage        string // e.g. "50m"
	MemoryUsage     string // e.g. "128Mi"
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

	// Get Kubernetes kubeClient and restConfig
	kubeClient, restConfig, err := getKubeClient()
	if err != nil {
		fmt.Println("Failed to get Kubernetes client:", err)
		return
	}
	fmt.Println("Successfully connected to Kubernetes cluster")

	// Create a metrics client (for CPU and memory usage)
	// Note: Ensure the metrics server is running in your cluster
	// You can run "kubectl get deployment metrics-server -n kube-system" to check if the metrics server is running"
	metricsClient, err := getMetricsClient(restConfig)
	metrics := []PodMetrics{}
	if err != nil {
		fmt.Println("Failed to create metrics client:", err)
		return
	} else {
		metrics, err = getPodMetrics(kubeClient, metricsClient, "default")
		if err != nil {
			fmt.Println("Error gathering pod metrics:", err)
		} else {
			// I think change later
			for _, m := range metrics {
				fmt.Printf("%+v\n", m)
			}
		}
	}

	// List all workloads
	if err := c.List(ctx, &workloadList); err != nil {
		fmt.Println("Failed to list workloads:", err)
		return
	}

	// Iterate through each workload and check its health
	for _, wl := range workloadList.Items {
		isHealthy := checkPodHealth(metrics, wl.Spec.JobName)

		// Update the workload health status if it has changed
		if wl.Spec.Health != isHealthy {
			wl.Spec.Health = isHealthy
			if err := c.Update(ctx, &wl); err != nil {
				fmt.Println("Failed to update workload health:", err)
			} else {
				fmt.Printf("Updated %s health to %v\n", wl.Name, isHealthy)
			}
		}
	}
}

// Get a metrics client from the Kubernetes rest.Config
func getMetricsClient(config *rest.Config) (*metricsclient.Clientset, error) {
	return metricsclient.NewForConfig(config)
}

// getKubeClient initializes and returns a Kubernetes clientset
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

		restConfig, e = clientConfig.ClientConfig() // <-- set global here
		if e != nil {
			err = e
			return
		}

		kubeClient, err = kubernetes.NewForConfig(restConfig)
	})

	return kubeClient, restConfig, err
}

// getPodMetrics retrieves metrics for all pods in a given namespace
func getPodMetrics(kubeClient *kubernetes.Clientset, metricsClient *metricsclient.Clientset, namespace string) ([]PodMetrics, error) {
	ctx := context.Background()
	var results []PodMetrics

	// List all pods in the specified namespace
	podList, err := kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// List all pod metrics in the specified namespace
	metricsList, err := metricsClient.MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Create a map for quick lookup of pod metrics by pod name
	metricsMap := make(map[string]metricsv1beta1.PodMetrics)
	for _, m := range metricsList.Items {
		metricsMap[m.Name] = m
	}

	// Iterate through each pod and gather metrics
	now := time.Now()
	for _, pod := range podList.Items {
		// Skip pods that are not in a running or pending state
		m := PodMetrics{
			PodName:      pod.Name,
			Namespace:    pod.Namespace,
			IsCrashed:    pod.Status.Phase == corev1.PodFailed,
			RestartCount: 0,
		}

		// Count the total number of restarts for all containers in the pod
		for _, cs := range pod.Status.ContainerStatuses {
			// Add the restart count for each container
			m.RestartCount += cs.RestartCount

			// Detect CrashLoopBackOff state
			if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
				m.IsCrashed = true
			}
		}

		// If the pod is in a failed state, set IsCrashed to true
		if pod.Status.Phase == corev1.PodPending {
			m.PendingDuration = now.Sub(pod.CreationTimestamp.Time)
		}

		// If the pod is not running or pending, skip it
		if podMetrics, exists := metricsMap[pod.Name]; exists {
			// If pod metrics exist, gather CPU and memory usage
			for _, container := range podMetrics.Containers {
				m.CPUUsage = container.Usage.Cpu().String()
				m.MemoryUsage = container.Usage.Memory().String()
				break // Assuming single container per pod; adjust if needed
			}
		}

		results = append(results, m)
	}

	return results, nil
}

// // checkPodHealth checks the health of pods associated with a specific job
// func checkPodHealth(metrics []PodMetrics, jobName string) bool {
// 	fmt.Println("Checking health for job:", jobName)
// 	// List all pods in the cluster
// 	pods, err := client.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
// 	if err != nil {
// 		fmt.Println("Error listing pods:", err)
// 		return true // Assume healthy if we can't check
// 	}

// 	// Check each pod for health status
// 	for _, pod := range pods.Items {
// 		fmt.Println("Checking pod:", pod.Name, "with labels:", pod.Labels)
// 		// Check if the pod belongs to the specified job
// 		// Assuming the job name is stored in the "job-name" label
// 		if pod.Labels["job-name"] != jobName {
// 			continue
// 		}

// 		// Check if the pod is in a healthy state
// 		if pod.Status.Phase == "Failed" || pod.Status.Phase == "Unknown" {
// 			fmt.Println("Pod is in Failed or Unknown state:", pod.Name)
// 			return false
// 		}

// 		// Check if the pod is pending for too long
// 		for _, cs := range pod.Status.ContainerStatuses {
// 			if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
// 				fmt.Println("Container is in CrashLoopBackOff state:", cs.Name, "in pod", pod.Name)
// 				return false
// 			}
// 		}
// 	}

// 	return true
// }

// checkPodHealth evaluates the health of a workload based on collected metrics
func checkPodHealth(metrics []PodMetrics, jobName string) bool {
	fmt.Println("Checking health for job:", jobName)
	for _, m := range metrics {
		// Match pods by job-name label convention (you can refine this)
		if m.PodName == "" || m.Namespace == "" {
			continue
		}

		// Parse and check CPU usage
		if m.CPUUsage != "" {
			cpuQty, err := resource.ParseQuantity(m.CPUUsage)
			if err == nil {
				fmt.Printf("Parsed CPU quantity for pod %s: %dm\n", m.PodName, cpuQty.MilliValue())
				if cpuQty.MilliValue()*1_000_000 > MaxCPUUsageNano {
					fmt.Printf("Pod %s CPU usage too high: %s\n", m.PodName, m.CPUUsage)
					return false
				}
			} else {
				fmt.Printf("Failed to parse CPU usage for pod %s: %v\n", m.PodName, err)
			}
		} else {
			fmt.Printf("No CPU usage reported for pod %s\n", m.PodName)
		}

		// Parse and check memory usage
		if m.MemoryUsage != "" {
			memQty, err := resource.ParseQuantity(m.MemoryUsage)
			if err == nil {
				fmt.Printf("Parsed memory quantity for pod %s: %d bytes\n", m.PodName, memQty.Value())
				if memQty.Value() > MaxMemoryUsageBytes {
					fmt.Printf("Pod %s memory usage too high: %s\n", m.PodName, m.MemoryUsage)
					return false
				}
			} else {
				fmt.Printf("Failed to parse memory usage for pod %s: %v\n", m.PodName, err)
			}
		} else {
			fmt.Printf("No memory usage reported for pod %s\n", m.PodName)
		}

		if m.RestartCount > MaxRestartCount {
			fmt.Printf("Pod %s has too many restarts: %d\n", m.PodName, m.RestartCount)
			return false
		}
		if m.PendingDuration > MaxPendingDuration {
			fmt.Printf("Pod %s has been pending too long: %v\n", m.PodName, m.PendingDuration)
			return false
		}
		if m.IsCrashed {
			fmt.Printf("Pod %s is crashed\n", m.PodName)
			return false
		}
	}
	return true
}
