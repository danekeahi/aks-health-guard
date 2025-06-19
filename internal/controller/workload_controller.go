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
	// will need a managed identity for this
	// assign role to contributor to managed cluster
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

	// 1. Get the resource group and cluster name from the request
	var workload monitoringv1.Workload
	if err := r.Get(ctx, req.NamespacedName, &workload); err != nil {
		log.Error(err, "unable to fetch Workload")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 2. Initialize workload status on first reconciliation
	// Check if this workload has been evaluated before - if not, set default healthy state
	if !workload.Status.Evaluated {
		log.Info("Initializing workload status to healthy", "JobName", workload.Spec.JobName)

		// Set the workload as healthy by default when first created
		workload.Status.Health = true
		// Mark as evaluated so we don't reinitialize on subsequent reconciliations
		workload.Status.Evaluated = true

		// Persist the status changes to the Kubernetes API server
		if err := r.Status().Update(ctx, &workload); err != nil {
			log.Error(err, "Failed to initialize workload status")
			return ctrl.Result{}, err
		}

		// Requeue the reconciliation to process the workload with its new status
		// This ensures the next reconciliation cycle will see the initialized status
		return ctrl.Result{Requeue: true}, nil
	}

	// 3. Log the workload details
	if !workload.Status.Health {
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
