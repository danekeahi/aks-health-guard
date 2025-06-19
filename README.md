# aks-health-guard
// TODO(user): Add simple overview of use/purpose

## Steps for Demo (June 19, 2025)
1.	“az login”
2.	Create a resource group and cluster, and run “az aks get-credentials”
    a.	The following is what I write:
        az group create --name aks-health-rg --location westus
        az aks create --resource-group aks-health-rg --name aks-health-cluster --node-count 1 --generate-ssh-keys
        az aks get-credentials --resource-group aks-health-rg --name aks-health-cluster
3.	Change constant variables “subscriptionID,” “tenantID,” “resourceGroupName,” and “clusterName” (in workload_controller.go) / “resourceName” (in detector.go) to your IDs and names of the RG and cluster you just created
4.  Add your thresholds to the metrics in the workload spec. If you don't want to use a certain metric, simply delete it.
5.	Then apply the workload yaml file and the pods (found under config/samples/).
    a.	These pods are made to be unhealthy so the detector can catch this. 
    b.	Here’s what I write:
        kubectl apply -f config/samples/monitoring_v1_workload.yaml
        kubectl apply -f config/samples/crash-pod.yaml
        kubectl apply -f config/samples/pending-pod.yaml
        kubectl apply -f config/samples/active-metrics-pod.yaml
6.	Make sure the health is “true” (meaning it’s healthy)
    a.	You can run this command to patch the health:
        kubectl patch workload workload-sample --type=merge -p '{"status":{"health":true,"evaluated":false}}' --subresource=status
7.	Run “make install”
8.	Run a long-running operation and wait until it says “running” with the spinning line
    a.	I use this command, since I have ran out of upgrades:
        az aks update --resource-group aks-health-rg --name aks-health-cluster --tags testRun=$(date +%s)
    b.	You can check if it’s currently running if you run this command:
        az aks show --resource-group aks-health-rg --name aks-health-cluster --query "provisioningState" --output tsv
9.	Run “make run” while the long-running operation is running
    a.	You should see the workload as healthy first
    b.	Then, every thirty seconds, it should detect if any of the pods are unhealthy (either CPU/Memory storage is too high or a pod is crashed or the pending state is too long)
    c.	Next, it will attempt to abort your current operation
    d.	It should say “Successfully aborted latest AKS operation”


## Description
// TODO(user): An in-depth paragraph about your project and overview of use

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

