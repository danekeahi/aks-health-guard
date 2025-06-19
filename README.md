# AKS Health Guard

A Kubernetes controller that monitors AKS cluster health and automatically aborts long-running operations when unhealthy conditions are detected.

## Overview

AKS Health Guard continuously monitors your Azure Kubernetes Service (AKS) cluster for unhealthy conditions such as:
- High CPU/Memory usage
- Crashed pods
- Pods stuck in pending state

When unhealthy conditions are detected during long-running AKS operations, the controller automatically aborts the operation to prevent further issues.

## Demo Instructions (June 19, 2025)

### Step 1: Login to Azure
```sh
az login
```

### Step 2: Create AKS Resources
Create a resource group and cluster, then configure kubectl:

```sh
# Create resource group
az group create --name aks-health-rg --location westus

# Create AKS cluster
az aks create --resource-group aks-health-rg --name aks-health-cluster --node-count 1 --generate-ssh-keys

# Get cluster credentials
az aks get-credentials --resource-group aks-health-rg --name aks-health-cluster
```

### Step 3: Configure Application Settings
Update the following constants in your code with your Azure resource details:
- In `workload_controller.go`: `subscriptionID`, `tenantID`, `resourceGroupName`, and `clusterName`
- In `detector.go`: `subscriptionID`, `tenantID`, `resourceGroupName`, and `resourceName`

### Step 4: Configure Health Thresholds
Add your desired thresholds to the metrics in the workload spec. Remove any metrics you don't want to monitor.

### Step 5: Deploy Test Resources
Apply the workload configuration and deploy test pods (designed to be unhealthy for demonstration):

```sh
# Deploy workload configuration
kubectl apply -f config/samples/monitoring_v1_workload.yaml

# Deploy test pods
kubectl apply -f config/samples/crash-pod.yaml
kubectl apply -f config/samples/pending-pod.yaml
kubectl apply -f config/samples/active-metrics-pod.yaml
```

### Step 6: Set Initial Health Status
Ensure the workload health is set to "true" (healthy):

```sh
kubectl patch workload workload-sample --type=merge -p '{"status":{"health":true,"evaluated":false}}' --subresource=status
```

### Step 7: Install the Controller
```sh
make install
```

### Step 8: Start a Long-Running Operation
Begin a long-running AKS operation and wait for it to show "running" status:

```sh
# Example: Update cluster with timestamp tag
az aks update --resource-group aks-health-rg --name aks-health-cluster --tags testRun=$(date +%s)
```

Check if the operation is running:
```sh
az aks show --resource-group aks-health-rg --name aks-health-cluster --query "provisioningState" --output tsv
```

### Step 9: Run the Health Monitor
While the long-running operation is active:

```sh
make run
```

**Expected behavior:**
1. Workload initially shows as healthy
2. Every 30 seconds, detector checks for unhealthy conditions
3. When unhealthy pods are detected, attempts to abort the current operation
4. Should display "Successfully aborted latest AKS operation"

## Getting Started

### Prerequisites
- go version v1.24.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/aks-health-guard-clean:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/aks-health-guard-clean:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/aks-health-guard-clean:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/aks-health-guard-clean/<tag or branch>/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v1-alpha
```

2. See that a chart was generated under 'dist/chart', and users
can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

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

