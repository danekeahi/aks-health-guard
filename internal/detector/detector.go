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
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

const (
	subscriptionID    = "feb5b150-60fe-4441-be73-8c02a524f55a"
	tenantID          = "72f988bf-86f1-41af-91ab-2d7cd011db47"
	resourceGroupName = "tanamutu-rg"
	resourceName      = "test"
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
	if err != nil {
		fmt.Println("Failed to create metrics client:", err)
		return
	} else {
		metrics, err := getPodMetrics(kubeClient, metricsClient, "default")
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
		isHealthy := checkPodHealth(kubeClient, wl.Spec)

		// Update the workload health status if it has changed
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

		for _, cs := range pod.Status.ContainerStatuses {
			m.RestartCount += cs.RestartCount
		}

		if pod.Status.Phase == corev1.PodPending {
			m.PendingDuration = now.Sub(pod.CreationTimestamp.Time)
		}

		if podMetrics, exists := metricsMap[pod.Name]; exists {
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

// checkPodHealth checks the health of pods associated with a specific job
func checkPodHealth(client *kubernetes.Clientset, spec monitoringv1.WorkloadSpec) bool {
	fmt.Println("Checking health for job:", spec.JobName)
	// List all pods in the cluster
	pods, err := client.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{}) // specify namespace if needed
	if err != nil {
		fmt.Println("Error listing pods:", err)
		return true // Assume healthy if we can't check
	}

	now := time.Now()

	// Check each pod for health status
	for _, pod := range pods.Items {
		fmt.Println("Checking pod:", pod.Name, "with labels:", pod.Labels)
		// Only look at pods that belong to the specified job
		// Assuming the job name is stored in the "job-name" label
		if pod.Labels["job-name"] != spec.JobName {
			continue
		}

		//--- Restart Count Check ---
		var restartCount int32
		for _, cs := range pod.Status.ContainerStatuses {
			restartCount += cs.RestartCount
		}
		if spec.MaxRestartCount > 0 && restartCount > spec.MaxRestartCount {
			fmt.Printf("Pod %s exceeded restart threshold: %d > %d\n", pod.Name, restartCount, spec.MaxRestartCount)
			return false
		}

		// --- Crash Loop Check ---
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
				fmt.Printf("Pod %s container %s is in CrashLoopBackOff\n", pod.Name, cs.Name)
				if spec.PodCrashThreshold > 0 {
					fmt.Println("CrashLoopBackOff is considered unhealthy")
					return false
				}
			}
		}

		// --- Pending Time Check ---
		if pod.Status.Phase == corev1.PodPending {
			pendingDuration := now.Sub(pod.CreationTimestamp.Time)
			if spec.MaxPendingDuration > 0 && pendingDuration > spec.MaxPendingDuration {
				fmt.Printf("Pod %s is pending too long: %s > %s\n", pod.Name, pendingDuration, spec.MaxPendingDuration)
				return false
			}
		}

		// --- CPU/Memory Usage Check ---
		// NOTE: This requires metrics client data to be passed in, or fetched earlier
		// If you want to add CPU/Memory checks here, you'd need to pass PodMetrics too.
		// Example:
		//   cpu := resource.MustParse("400m")
		//   maxCPU := resource.MustParse(spec.MaxCPUUsage)
		//   if cpu.Cmp(maxCPU) > 0 { ... }

		// Currently, CPU/Memory checks would need integration with metrics-server metrics,
		// which you already fetch in another function (getPodMetrics).
	}

	return true
}
