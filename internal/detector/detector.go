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
	kubeclientcmd "k8s.io/client-go/tools/clientcmd"
)

const (
	subscriptionID    = "8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8"
	tenantID          = "72f988bf-86f1-41af-91ab-2d7cd011db47"
	resourceGroupName = "aks-health-rg"
	resourceName      = "aks-health-cluster"
)

var (
	kubeClient     *kubernetes.Clientset
	kubeClientOnce sync.Once
)

// func StartDetectorLoop(c client.Client) {
// 	ticker := time.NewTicker(30 * time.Second)
// 	defer ticker.Stop()

// 	for {
// 		select {
// 		case <-ticker.C:
// 			checkAndUpdateWorkloadHealth(c)
// 		}
// 	}
// }

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

func checkAndUpdateWorkloadHealth(c client.Client) {
	ctx := context.Background()
	var workloadList monitoringv1.WorkloadList

	client, err := getKubeClient()
	if err != nil {
		fmt.Println("Failed to get Kubernetes client:", err)
		return
	}
	fmt.Println("Successfully connected to Kubernetes cluster")

	if err := c.List(ctx, &workloadList); err != nil {
		fmt.Println("Failed to list workloads:", err)
		return
	}

	for _, wl := range workloadList.Items {
		isHealthy := checkPodHealth(client, wl.Spec.JobName)

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

func getKubeClient() (*kubernetes.Clientset, error) {
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

		restConfig, e := clientConfig.ClientConfig()
		if e != nil {
			err = e
			return
		}

		kubeClient, err = kubernetes.NewForConfig(restConfig)
	})

	return kubeClient, err
}

func checkPodHealth(client *kubernetes.Clientset, jobName string) bool {
	fmt.Println("Checking health for job:", jobName)
	pods, err := client.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Println("Error listing pods:", err)
		return true // Assume healthy if we can't check
	}

	for _, pod := range pods.Items {
		fmt.Println("Checking pod:", pod.Name, "with labels:", pod.Labels)
		if pod.Labels["job-name"] != jobName {
			continue
		}

		if pod.Status.Phase == "Failed" || pod.Status.Phase == "Unknown" {
			fmt.Println("Pod is in Failed or Unknown state:", pod.Name)
			return false
		}

		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
				fmt.Println("Container is in CrashLoopBackOff state:", cs.Name, "in pod", pod.Name)
				return false
			}
		}
	}

	return true
}
