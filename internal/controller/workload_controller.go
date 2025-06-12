/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	// "net/http"
	// "os"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// logf "sigs.k8s.io/controller-runtime/pkg/log"

	monitoringv1 "github.com/danekeahi/aks-health-guard/api/v1"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
)

// WorkloadReconciler reconciles a Workload object
type WorkloadReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	subscriptionID = "8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8"
	// tenantID          = "72f988bf-86f1-41af-91ab-2d7cd011db47"
	resourceGroupName = "aks-health-rg"
	clusterName       = "aks-health-cluster"
)

// func abortLatestAKSOperation(subscriptionID, resourceGroup, clusterName string) error {
// 	apiVersion := "2025-03-01"
// 	url := fmt.Sprintf(
// 		"https://management.azure.com/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ContainerService/managedClusters/%s/abortLatestOperation?api-version=%s",
// 		subscriptionID, resourceGroup, clusterName, apiVersion,
// 	)

// 	fmt.Println("[DEBUG] abort URL:", url)

// 	token := "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiIsIng1dCI6IkNOdjBPSTNSd3FsSEZFVm5hb01Bc2hDSDJYRSIsImtpZCI6IkNOdjBPSTNSd3FsSEZFVm5hb01Bc2hDSDJYRSJ9.eyJhdWQiOiJodHRwczovL21hbmFnZW1lbnQuY29yZS53aW5kb3dzLm5ldC8iLCJpc3MiOiJodHRwczovL3N0cy53aW5kb3dzLm5ldC83MmY5ODhiZi04NmYxLTQxYWYtOTFhYi0yZDdjZDAxMWRiNDcvIiwiaWF0IjoxNzQ5NTkyMjk5LCJuYmYiOjE3NDk1OTIyOTksImV4cCI6MTc0OTU5NjU4MiwiX2NsYWltX25hbWVzIjp7Imdyb3VwcyI6InNyYzEifSwiX2NsYWltX3NvdXJjZXMiOnsic3JjMSI6eyJlbmRwb2ludCI6Imh0dHBzOi8vZ3JhcGgud2luZG93cy5uZXQvNzJmOTg4YmYtODZmMS00MWFmLTkxYWItMmQ3Y2QwMTFkYjQ3L3VzZXJzL2Q3MzQzZDg0LTNmNzQtNDZiMS1iZmM0LWY2Y2I3MzE5MWRlYS9nZXRNZW1iZXJPYmplY3RzIn19LCJhY3IiOiIxIiwiYWNycyI6WyJwMSIsImMxMCJdLCJhaW8iOiJBWlFBYS84WkFBQUF3djE1YXljZlRzK2syWGVLYXdWL0hManRxN21iU3NKbkpMc0dRR0ZkUVJnMFV4cjN4cWFrZVNrd2FmM2wybFhmWlpLWkxwOFIyZDMrNndKSHJiREZxbWRRVG1JZ09YR0JzSExzS01EVkViSVU5VGdqM1BISmJMMENURVNjczU0b1Q1KzJJQWhhRnIzVllSdzNRRUFOOFJCQmowTnBUWmRSWHNHVGY1cWQyQitlS1licE9TSktxYWJNWTJQbmFuR2YiLCJhbXIiOlsiZmlkbyIsInJzYSIsIm1mYSJdLCJhcHBpZCI6ImM0NGI0MDgzLTNiYjAtNDljMS1iNDdkLTk3NGU1M2NiZGYzYyIsImFwcGlkYWNyIjoiMCIsImRldmljZWlkIjoiMjUxODQwNzItOTRkNS00OTdjLWI5OWQtNjY5OTdjNjAwZTIxIiwiZmFtaWx5X25hbWUiOiJLZWFoaSIsImdpdmVuX25hbWUiOiJEYW5lIiwiaWR0eXAiOiJ1c2VyIiwiaXBhZGRyIjoiMjAwMTo0ODk4OjgwZTg6Mzg6ZTA0NzpiYmRhOjRkMTI6MmVlOSIsIm5hbWUiOiJEYW5lIEtlYWhpIiwib2lkIjoiZDczNDNkODQtM2Y3NC00NmIxLWJmYzQtZjZjYjczMTkxZGVhIiwib25wcmVtX3NpZCI6IlMtMS01LTIxLTIxMjc1MjExODQtMTYwNDAxMjkyMC0xODg3OTI3NTI3LTgzMTU4NDQ4IiwicHVpZCI6IjEwMDMyMDA0OTc2QTQ5MzAiLCJyaCI6IjEuQVFFQXY0ajVjdkdHcjBHUnF5MTgwQkhiUjBaSWYza0F1dGRQdWtQYXdmajJNQk1hQUNnYUFBLiIsInNjcCI6InVzZXJfaW1wZXJzb25hdGlvbiIsInNpZCI6IjAwNWNmNGY5LWJjMjUtNTJiNy03NzcyLWM0NDU3NGFjZWQyZiIsInN1YiI6ImZ5UllrRzc2UDY5YXBIUEhYWmpoTmxhQXZjYVlxQ1lGYWM3Z3N0N0JJSmMiLCJ0aWQiOiI3MmY5ODhiZi04NmYxLTQxYWYtOTFhYi0yZDdjZDAxMWRiNDciLCJ1bmlxdWVfbmFtZSI6InQtZGFuZWtlYWhpQG1pY3Jvc29mdC5jb20iLCJ1cG4iOiJ0LWRhbmVrZWFoaUBtaWNyb3NvZnQuY29tIiwidXRpIjoidHpQbmRRa3ZRRUN0NHo4V0lwQUpBQSIsInZlciI6IjEuMCIsInhtc19mdGQiOiJCWmJTaXJfVUlUVC1raXRndURMeGdELWFETVVjM1AwNUh1V2FvSWd3WFI4QmRYTnpiM1YwYUMxa2MyMXoiLCJ4bXNfaWRyZWwiOiIxIDYiLCJ4bXNfdGNkdCI6MTI4OTI0MTU0N30.F1R21uddtpaLAlxgyRwpCpfADalOi-zipz-CYi0JGH6yd6eeE-ydL28wMq4goOyfbJqYHyhVpO090s4Rfu_awx_kZInv0iTlNTbJk3okJl1zCdrg-RXwsJ9tHQjzJNxzyt9hJxhiGwpQlZ34l600vNolVpbcbqczUA-o4R9DpGLSAf5WYYqlAdVLWXpVNCIgJmVwKuv-dZoDRbkC1iFMfLPZ9EX5F49MurD3eRKm8MUpWM4umO4TQYbll1AyxEWB5Z6uscxqBitoQSSRvIfvLu_WPeXPH-_2_VF05MtGA5R6F472NnrZ2pIsZtne3_97GOSenrJ4QuoDa-xlCPEkaQ"
// 	if token == "" {
// 		return fmt.Errorf("AZURE_TOKEN environment variable not set")
// 	}
// 	fmt.Println("[DEBUG] token length:", len(token))
// 	if len(token) < 100 {
// 		fmt.Println("[WARN] Azure token seems suspiciously short")
// 	}

// 	req, err := http.NewRequest("POST", url, nil)
// 	if err != nil {
// 		return fmt.Errorf("failed to create HTTP request: %w", err)

// 	}
// 	req.Header.Add("Authorization", "Bearer "+token)
// 	req.Header.Add("Content-Type", "application/json")

// 	fmt.Println("[DEBUG] Request Headers:")
// 	for k, v := range req.Header {
// 		fmt.Printf("  %s: %s\n", k, v)
// 	}

// 	client := &http.Client{}
// 	resp, err := client.Do(req)
// 	if err != nil {
// 		return fmt.Errorf("HTTP request failed: %w", err)
// 	}
// 	defer resp.Body.Close()

// 	fmt.Println("[DEBUG] Response status:", resp.Status)

// 	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
// 		return fmt.Errorf("abort failed: %s", resp.Status)
// 	}

// 	return nil
// }

func abortLatestAKSOperation(ctx context.Context, resourceGroupName, clusterName string) error {
	// 1. Read the subscription ID from the environment
	subID := subscriptionID
	if subID == "" {
		return fmt.Errorf("AZURE_SUBSCRIPTION_ID not set")
	}

	//rg := os.Getenv("AZURE_RESOURCE_GROUP")
	//cluster := os.Getenv("AZURE_CLUSTER_NAME")
	//if rg == "" || cluster == "" {
	//  return fmt.Errorf("missing RG or CLUSTER env")
	//}

	// 2. Acquire a credential using 'az login' or env vars
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return fmt.Errorf("failed to get Azure credential: %w", err)
	}

	// 3. Build the AKS client
	client, err := armcontainerservice.NewManagedClustersClient(subID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create AKS client: %w", err)
	}

	// 4. Kick off the abort operation
	poller, err := client.BeginAbortLatestOperation(ctx, resourceGroupName, clusterName, nil)
	if err != nil {
		return fmt.Errorf("abort operation failed: %w", err)
	}

	// 5. Wait for the abort call to complete (optional)
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("polling abort operation: %w", err)
	}

	return nil
}

// +kubebuilder:rbac:groups=monitoring.healthcheck.dev,resources=workloads,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.healthcheck.dev,resources=workloads/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monitoring.healthcheck.dev,resources=workloads/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Workload object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *WorkloadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// TODO(user): your logic here
	var workload monitoringv1.Workload
	if err := r.Get(ctx, req.NamespacedName, &workload); err != nil {
		log.Error(err, "unable to fetch Workload")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !workload.Spec.Health {
		log.Info("Workload is UNHEALTHY", "JobName", workload.Spec.JobName)

		// call azure abort
		err := abortLatestAKSOperation(ctx, resourceGroupName, clusterName)
		if err != nil {
			log.Error(err, "Failed to abort latest AKS operation")
		} else {
			log.Info("Successfully aborted latest AKS operation")
		}
	} else {
		log.Info("Workload is healthy", "JobName", workload.Spec.JobName)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkloadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitoringv1.Workload{}).
		Named("workload").
		Complete(r)
}
