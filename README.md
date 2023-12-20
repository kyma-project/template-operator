[![REUSE status](https://api.reuse.software/badge/github.com/kyma-project/template-operator)](https://api.reuse.software/info/github.com/kyma-project/template-operator)

# Template Operator
This documentation and template serve as a reference to implement a module operator for integration with [Lifecycle Manager](https://github.com/kyma-project/lifecycle-manager/tree/main/).
It utilizes the [kubebuilder](https://book.kubebuilder.io/) framework with some modifications to implement Kubernetes APIs for Custom Resource Definitions (CRDs).
Additionally, it hides Kubernetes boilerplate code to develop fast and efficient control loops in Go.


## Contents
- [Template Operator](#template-operator)
  - [Contents](#contents)
  - [Understanding Module Development in Kyma](#understanding-module-development-in-kyma)
    - [Basic Principles](#basic-principles)
    - [Release Channels](#release-channels)
    - [Comparison to Other Established Frameworks](#comparison-to-other-established-frameworks)
      - [Operator Lifecycle Manager (OLM)](#operator-lifecycle-manager-olm)
    - [Crossplane](#crossplane)
  - [Implementation](#implementation)
    - [Prerequisites](#prerequisites)
      - [**Brew**](#brew)
      - [**Fetch sources directly**](#fetch-sources-directly)
    - [Generate kubebuilder operator](#generate-kubebuilder-operator)
      - [Optional: Adjust the default config resources](#optional-adjust-the-default-config-resources)
      - [API Ddefinition Steps](#api-ddefinition-steps)
      - [Controller Implementation Steps](#controller-implementation-steps)
    - [Local Testing](#local-testing)
  - [Bundling and Installation](#bundling-and-installation)
    - [Grafana Dashboard for Simplified Controller Observability](#grafana-dashboard-for-simplified-controller-observability)
    - [Role-Based Access Control (RBAC)](#role-based-access-control-rbac)
    - [Prepare and Build Module Operator Image](#prepare-and-build-module-operator-image)
    - [Build and Push Your Module to the Registry](#build-and-push-your-module-to-the-registry)
  - [Using Your Module in the Lifecycle Manager Ecosystem](#using-your-module-in-the-lifecycle-manager-ecosystem)
    - [Deploying Kyma Infrastructure Operators with `kyma alpha deploy`](#deploying-kyma-infrastructure-operators-with-kyma-alpha-deploy)
    - [Deploying `ModuleTemplate` into the Control Plane](#deploying-moduletemplate-into-the-control-plane)
    - [Debugging the Operator Ecosystem](#debugging-the-operator-ecosystem)
    - [Registering your Module Within the Control Plane](#registering-your-module-within-the-control-plane)

## Understanding Module Development in Kyma 

Before going in-depth, make sure you are familiar with:

- [Modularization in Kyma](https://github.com/kyma-project/community/tree/main/concepts/modularization)
- [Operator Pattern in Kubernetes](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

This Guide serves as a comprehensive step-by-step tutorial on how to properly create a module from scratch by using the operator that installs the Kubernetes YAML resources. 
> **NOTE:** While other approaches are encouraged, there are no dedicated guides available yet. These will follow with sufficient requests and the adoption of Kyma modularization.

### Basic Principles
Every Kyma module using the operator follows five basic principles:

- Is declared as available for use in a release channel through the `ModuleTemplate` custom resource (CR) in Control Plane
- Is declared as the desired state within the `Kyma` CR in runtime or Control Plane
- Is installed or managed in the runtime by [Lifecycle Manager](https://github.com/kyma-project/lifecycle-manager) through the `Manifest` CR in Control Plane
- Owns at least one CRD that defines the contract towards a Runtime Administrator and configures its behavior
- Operates on at most one runtime at any given time

### Release Channels
Release channels let customers try new modules and features early and decide when the updates should be applied. For more info, see the [release channels documentation in our Modularization overview](https://github.com/kyma-project/community/tree/main/concepts/modularization#release-channels).

The following rules apply to the channels:
1. Lower case letters from a to z.
2. The total length is between 3 and 32.

If you are planning to migrate a pre-existing module within Kyma, read the [transition plan for existing modules](https://github.com/kyma-project/community/blob/main/concepts/modularization/transition.md).

### Comparison to Other Established Frameworks

#### Operator Lifecycle Manager (OLM)

Compared to [OLM](https://olm.operatorframework.io/), the Kyma modularization differs in a few aspects.
While OLM is built heavily around a static dependency expression, Kyma Modules are expected to resolve dependencies dynamically. This means that while in OLM, a module has to declare CRDs and APIs that it depends on. In Kyma, all modules can depend on each other without declaring it in advance.
This makes it harder to understand compared to a strict dependency graph, but it comes with a few key advantages:

- Concurrent optimization on the controller level: every controller in Kyma is installed simultaneously and is not blocked from installation until other operators are available.
This makes it easy to, for example, create or configure resources that do not need to wait for the dependency (for example, ConfigMap can be created even before a deployment that has to wait for an API to be present).
While this forces controllers to consider a case where the dependency is absent, we encourage eventual consistency and do not enforce a strict lifecycle model on the modules.
- Discoverability is handled not through a registry or server but through a declarative configuration.
Every Module is installed through `ModuleTemplate`, which is semantically the same as registering an operator in an OLM registry or `CatalogSource`. 
`ModuleTemplate` however is a normal CR and can be installed on Control Plane. 
This allows multiple Control Planes to offer differing modules simply at configuration time.
Also, we do not use file-based catalogs to maintain our catalog but maintain every `ModuleTemplate` through [Open Component Model](https://ocm.software/), an open standard to describe software artifact delivery.

Regarding release channels for operators, Lifecycle Manager operates at the same level as OLM. However, with `Kyma`, we ensure the bundling of `ModuleTemplate` to a specific release channel.
We are heavily inspired by the way that OLM handles release channels, but we do not have an intermediary `Subscription` that assigns the catalog to the channel. Instead, every module is already delivered in `ModuleTemplate` in a channel.

There is a distinct difference in the `ModuleTemplate` parts. 
`ModuleTemplate` contains not only a specification of the operator to be installed through a dedicated Layer but it also a set of default values for a given channel when installed for the first time.
When installing an operator from scratch through Kyma, the module will already be initialized with a default set of values.
However, when upgrading, Lifecycle Manager is not expected to update the values to new defaults. Instead, it is a way for module developers to prefill their Operators with instructions based on a given environment (the channel).
Note that these default values are static once installed, and they are not be updated unless a new module installation occurs, even when the content of `ModuleTemplate` changes. 
This is because a customer is expected to be able to change the settings of the module CR at any time without the Kyma ecosystem overriding it.
Thus, the module CR can also be treated as a customer or runtime-facing API that allows us to offer typed configuration for multiple parts of Kyma.

### Crossplane

With [Crossplane](https://crossplane.io/), you are fundamentally allowing providers to interact with your Control Plance.
The most similar aspect of the Crossplane lifecycle is that we also use opinionated OCI Images to bundle our modules. 
We use `ModuleTemplate` to reference our layers containing the necessary metadata to deploy our controllers, just like Crossplane.
However, we do not speculate on the permissions of controllers and enforce stricter versioning guarantees, only allowing `semver` to be used for modules and `Digest` for the `sha` digests for individual layers of modules.

Fundamentally different is also the way that `Providers` and `Composite Resources` work compared to the Kyma ecosystem.
While Kyma allows any module to bring an individual CR into the cluster for configuration, a `Provider` in Crossplane is located in Control Plane and only directs installation targets.
We handle this kind of data centrally through acquisition strategies for credentials and other centrally managed data in the `Kyma` CR. 
Thus, it is most fitting to consider the Kyma ecosystem as a heavily opinionated `Composite Resource` from Crossplane, with the `Managed Resource` being tracked with the Lifecycle Manager `Manifest`. 

Compared to Crossplane, we also encourage the creation of our CRDs in place of the concept of the `Managed Resource`, and in the case of configuration, we can synchronize not only a desired state for all modules from Control Plance but also from the runtime.
Similarly, we make the runtime module catalog discoverable from inside the runtime with a dedicated synchronization mechanism.

Lastly, compared to Crossplane, we have fewer choices regarding revision management and dependency resolution.
While in Crossplane, it is possible to define custom package, revision, and dependency policies. 
However, in Kyma, managed use cases usually require unified revision handling, and we do not target a generic solution for revision management of different module ecosystems.

## Implementation

### Prerequisites

  **WARNING:** For all use cases in the guide, you will need a cluster for end-to-end testing outside your [envtest](https://book.kubebuilder.io/reference/envtest.html) integration test suite.
  This guide is HIGHLY RECOMMENDED to be followed for a smooth development process.
  This is a good alternative if you do not want to use an entire control-plane infrastructure and still want to test your operators properly.

* A provisioned Kubernetes cluster and OCI registry
* [kubectl](https://kubernetes.io/docs/tasks/tools/)
* [kubebuilder](https://book.kubebuilder.io/)

Use one of the following options to install kubebuilder:

<!-- tabs:start -->
#### **Brew**
    ```bash
    brew install kubebuilder
    ```
#### **Fetch sources directly**
    ```bash
    curl -L -o kubebuilder https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)
    chmod +x kubebuilder && mv kubebuilder /usr/local/bin/
    ```
<!-- tabs:end -->
* [Kyma CLI](https://storage.googleapis.com/kyma-cli-stable/kyma-darwin)
* An OCI registry to host OCI Image
  * Follow our [Provision cluster and OCI registry](https://github.com/kyma-project/lifecycle-manager/blob/main/docs/developer-tutorials/provision-cluster-and-registry.md) guide to create a local registry provided by k3d or use the [Google Container Registry (GCR)](https://github.com/kyma-project/lifecycle-manager/blob/main/docs/developer-tutorials/prepare-gcr-registry.md) guide for a remote registry.

### Generate kubebuilder operator

1. Initialize the `kubebuilder` project. Make sure the domain is set to `kyma-project.io`. Execute the following command in the `test-operator` folder.
    ```shell
    kubebuilder init --domain kyma-project.io --repo github.com/kyma-project/test-operator --project-name=test-operator --plugins=go/v4-alpha
    ```

2. Create the API group version and kind for the intended CR(s). Please make sure the `group` is set to `operator`.
    ```shell
    kubebuilder create api --group operator --version v1alpha1 --kind Sample --resource --controller --make
    ```

3. Run `make manifests` to generate respective CRDs.

Set up a basic kubebuilder operator with appropriate scaffolding.

#### Optional: Adjust the default config resources
If the module operator is deployed under same namespace with other operators, differentiate your resources by adding common labels.

1. Add `commonLabels` to default `kustomization.yaml`, [reference implementation](config/default/kustomization.yaml).

2. Include all resources (for example, [manager.yaml](config/manager/manager.yaml)) that contain label selectors by using `commonLabels`.

Further reading: [Kustomize built-in commonLabels](https://github.com/kubernetes-sigs/kustomize/blob/master/api/internal/konfig/builtinpluginconsts/commonlabels.go)

#### API Ddefinition Steps

1. Refer to [State requirements](api/v1alpha1/status.go) and similarly include them in your `Status` sub-resource.

This `Status` sub-resource must contain all valid `State` (`.status.state`) values to be compliant with the Kyma ecosystem.

   ```go
   package v1alpha1
   // Status defines the observed state of Module CR.
   type Status struct {
       // State signifies current state of Module CR.
       // Value can be one of ("Ready", "Processing", "Error", "Deleting").
       // +kubebuilder:validation:Required
       // +kubebuilder:validation:Enum=Processing;Deleting;Ready;Error
       State State `json:"state"`
   }
   ```

Include the `State` values in your `Status` sub-resource, either through inline reference or direct inclusion. These values have literal meaning behind them, so use them appropriately.

1. Optionally, you can add additional fields to your `Status` sub-resource. 
2. For instance, `Conditions` are added to `SampleCR` in the [API definition](api/v1alpha1/sample_types.go).
This also includes the required `State` values, using an inline reference.
See the following SampleCR reference implementation.
    
    ```go
    package v1alpha1
    // Sample is the Schema for the samples API
    type Sample struct {
        metav1.TypeMeta   `json:",inline"`
        metav1.ObjectMeta `json:"metadata,omitempty"`
    
        Spec   SampleSpec   `json:"spec,omitempty"`
        Status SampleStatus `json:"status,omitempty"`
    }
    
    type SampleStatus struct {
        Status `json:",inline"`
    
        // Conditions contain a set of conditionals to determine the State of Status.
        // If all Conditions are met, State is expected to be in StateReady.
        Conditions []metav1.Condition `json:"conditions,omitempty"`
    
        // add other fields to status subresource here
    }
    ```

3. Run `make generate manifests`, to generate boilerplate code and manifests.

#### Controller Implementation Steps

**Warning**: This sample implementation is only for reference. You can copy parts of it but do not add this repository as a dependency to your project.

1. Implement `State` handling to represent the corresponding state of the reconciled resource by following [kubebuilder](https://book.kubebuilder.io/) guidelines on how to implement controllers.

2. Refer to `SampleCR` [controller implementation](controllers/sample_controller_rendered_resources.go) for setting the appropriate `State` and `Conditions` values to your `Status` sub-resource.

`SampleCR` is reconciled to install or uninstall a list of rendered resources from a YAML file on the file system.
    
   ```go   
   r.setStatusForObjectInstance(ctx, objectInstance, status.
   WithState(v1alpha1.StateReady).
   WithInstallConditionStatus(metav1.ConditionTrue, objectInstance.GetGeneration()))
   ```
    
3. The reference controller implementations listed above use [Server-Side Apply](https://kubernetes.io/docs/reference/using-api/server-side-apply/) instead of conventional methods to process resources on the target cluster.
Parts of this logic can be leveraged to implement your own controller logic. Check out functions inside these controllers for state management and other implementation details.

### Local Testing
* Connect to your cluster and ensure `kubectl` is pointing to the desired cluster.
* Install CRDs with `make install`
  **WARNING:** This installs a CRD on your cluster, so create your cluster before running the `install` command. See [Prerequisites](#prerequisites) for details on the cluster setup.
* _Local setup_: install your module CR on a cluster and execute `make run` to start your operator locally.

**WARNING:** Note that while `make run` fully runs your controller against the cluster, it is not feasible to compare it to a productive operator.
This is mainly because it runs with a client configured with privileges derived from your `KUBECONFIG` environment variable. For in-cluster configuration, see [Guide on RBAC Management](#rbac).

## Bundling and Installation

### Grafana Dashboard for Simplified Controller Observability

You can extend the operator further by using automated dashboard generation for Grafana.

Use the following command to generate two grafana dashboard files with controller related metrics in the `/grafana` folder:

```shell
kubebuilder edit --plugins grafana.kubebuilder.io/v1-alpha
```

To import the Grafana dashboard, read the [official Grafana guide](https://grafana.com/docs/grafana/latest/dashboards/export-import/#import-dashboard).
This feature is supported by the [kubebuilder Grafana plugin](https://book.kubebuilder.io/plugins/grafana-v1-alpha.html).

### Role-Based Access Control (RBAC)
Ensure you have appropriate authorizations assigned to your controller binary before running it inside a cluster (not locally with `make run`).
The Sample CR [controller implementation](controllers/sample_controller_rendered_resources.go) includes RBAC generation (via kubebuilder) for all resources across all API groups.
This should be adjusted according to the chart manifest resources and reconciliation types.

Towards the earlier stages of your operator development, RBACs can accommodate all resource types and adjust them later per your requirements.

   ```go
   package controllers
   // TODO: dynamically create RBACs! Remove line below.
   //+kubebuilder:rbac:groups="*",resources="*",verbs="*"
   ```

**REMEMBER:** Run `make manifests` after this adjustment for it to take effect.

### Prepare and Build Module Operator Image

**WARNING:** This step requires the working OCI registry. See [Prerequisites](#prerequisites).

1. Include the static module data in your Dockerfile:
    ```dockerfile
    FROM gcr.io/distroless/static:nonroot
    WORKDIR /
    COPY module-data/ module-data/
    COPY --from=builder /workspace/manager .
    USER 65532:65532
    
    ENTRYPOINT ["/manager"]
    ``` 

The sample module data in this repository includes a YAML manifest in the `module-data/yaml` directories.
Reference the YAML manifest directory with the `spec.resourceFilePath` attribute of the `Sample` CR.
The example CRs in the `config/samples` directory already reference the mentioned directories.
Feel free to organize the static data differently. The included `module-data` directory serves just as an example.
You may also decide not to include any static data at all. In that case, you must provide the controller with the YAML data at runtime using other techniques, such as Kubernetes volume mounting.

2. Build and push your module operator binary by adjusting `IMG`, if necessary, and running the inbuilt kubebuilder commands.
Assuming your operator image has the following base settings:
* is hosted at `op-kcp-registry.localhost:8888/unsigned/operator-images` 
* controller image name is `sample-operator`
* controller image has version `0.0.1`

You can run the following command
   ```sh
   make docker-build docker-push IMG="op-kcp-registry.localhost:8888/unsigned/operator-images/sample-operator:0.0.1"
   ```
   
This builds the controller image and then pushes it as the image defined in `IMG` based on the kubebuilder targets.

### Build and Push Your Module to the Registry

**WARNING:** This step requires the working OCI Registry, cluster and Kyma CLI. See [Prerequisites](#prerequisites).

1. Generate the CRDs and resources for the module from the `default` kustomization into a manifest file using the following command:
   ```shell
   make build-manifests
   ```
This file will be used as a manifest for the module configuration in the next step.
   
Furthermore, make sure the settings from [Prepare and Build Module Operator Image](#prepare-and-build-module-operator-image) for single-cluster mode, and the following module settings are applid:
* is hosted at `op-kcp-registry.localhost:8888/unsigned`
* for a k3d registry the `insecure` flag (`http` instead of `https` for registry communication) is enabled
* Kyma CLI in `$PATH` under `kyma` is used
* the default sample under `config/samples/operator.kyma-project.io_v1alpha1_sample.yaml` has been adjusted to be a valid CR.

**WARNING:** The settings above reflect your default configuration for a module. If you want to change this, you must manually adjust it to a different configuration. 
You can also define multiple files in `config/samples`, but you must specify the correct file during the bundling.

* `.gitignore` has been adjusted and the following ignores have been added

   ```gitignore
   # generated dummy charts
     charts
   # template generated by kyma create module
     template.yaml
   ```
   
2. To configure the module, adjust the file `module-config.yaml`, located at the root of the repository.
 
   The following fields are available for the configuration of the module:
   - `name`: (Required) The name of the module.
   - `version`: (Required) The version of the module.
   - `channel`: (Required) The channel that must be used in the ModuleTemplate. Must be a valid Kyma state.
   - `manifest`: (Required) The relative path to the manifest file (generated in the first step).
   - `defaultCR`: (Optional) The relative path to a YAML file containing the default CR for the module.
   - `resourceName`: (Optional) The name for the ModuleTemplate that will be created.
   - `namespace`: (Optional) The namespace where the ModuleTemplate will be deployed.
   - `security`: (Optional) The name of the security scanners configuration file.
   - `internal`: (Optional) Determines whether the ModuleTemplate must have the internal flag or not. Type is bool.
   - `beta`: (Optional) Determines whether the ModuleTemplate must have the beta flag or not. Type is bool.
   - `labels`: (Optional) Additional labels for the ModuleTemplate.
   - `annotations`: (Optional) Additional annotations for the ModuleTemplate.
   - `customStateCheck`: (Optional) Specifies custom state checks for the module.

   An example configuration:

   ```yaml
   name: kyma-project.io/module/template-operator
   version: v1.0.0
   channel: regular
   manifest: template-operator.yaml
   ```
3. Run the following command to create the module configured in `module-config.yaml` and push your module operator image to the specified registry:

   ```sh
   kyma alpha create module --insecure --registry op-kcp-registry.localhost:8888/unsigned --module-config-file module-config.yaml 
   ```
   
**WARNING:** For external registries (for example, Google Container/Artifact Registry) never use insecure. Instead, specify credentials. You can find more details in the CLI help documentation.
   
4. Verify that the module creation succeeded and observe the generated `template.yaml` file. It will contain the ModuleTemplate CR and descriptor of the component under `spec.descriptor.component`.   

   ```yaml
   component:
     componentReferences: []
     labels:
     - name: security.kyma-project.io/scan
       value: enabled
       version: v1
     name: kyma-project.io/module/template-operator
     provider: internal
     repositoryContexts:
     - baseUrl: http://op-kcp-registry.localhost:8888/unsigned
       componentNameMapping: urlPath
       type: ociRegistry
     resources:
     - access:
         digest: sha256:d008309948bd08312016731a9c528438e904a71c05a110743f5a151f0c3c4a9e
         type: localOciBlob
       name: raw-manifest
       relation: local
       type: yaml
       version: v1.0.0
     sources:
     - access:
         commit: 4f2ae6474ea7ababf9be246abe74b40f1baf1121
         repoUrl: https://github.com/LeelaChacha/kyma-cli.git
         type: gitHub
       labels:
       - name: git.kyma-project.io/ref
         value: refs/heads/feature/#90-update-build-instructions
         version: v1
       - name: scan.security.kyma-project.io/language
         value: golang-mod
         version: v1
       - name: scan.security.kyma-project.io/subprojects
         value: "false"
         version: v1
       - name: scan.security.kyma-project.io/exclude
         value: '**/test/**,**/*_test.go'
         version: v1
       name: module-sources
       type: git
       version: v1.0.0
     version: v1.0.0
   ```
   
The CLI created various layers that are referenced in the `blobs` directory. For more information on layer structure, check the module creation with `kyma alpha mod create --help`.

## Using Your Module in the Lifecycle Manager Ecosystem

### Deploying Kyma Infrastructure Operators with `kyma alpha deploy`

**WARNING:** This step requires the working OCI registry and cluster. See [Prerequisites](#prerequisites).

Now that everything is prepared in a cluster of your choice, you can reference the module within any `Kyma` CR in your Control Plane cluster.

Deploy the [Lifecycle Manager](https://github.com/kyma-project/lifecycle-manager/tree/main) to the Control Plane cluster with:

   ```shell
   kyma alpha deploy
   ```

### Deploying `ModuleTemplate` into the Control Plane

Run the command for creating the `ModuleTemplate` in your cluster.
After this the module will be available for consumption based on the module name configured with the label `operator.kyma-project.io/module-name` in the `ModuleTemplate`.

**WARNING:** Depending on your setup against either a k3d cluster or registry, you must run the script `/scripts/patch_local_template.sh` before pushing the ModuleTemplate to have the proper registry setup.
(This is necessary for k3d clusters due to port-mapping issues in the cluster that the operators cannot reuse. Take a look at the [relevant issue for more details](https://github.com/kyma-project/module-manager/issues/136#issuecomment-1279542587)).

   ```sh
   kubectl apply -f template.yaml
   ```

You can use the following command to enable the module you created:

   ```shell
   kyma alpha enable module <module-identifier> -c <channel>
   ```

This adds your module to `.spec.modules` with a name originally based on the `"operator.kyma-project.io/module-name": "sample"` label generated in `template.yaml`:

   ```yaml
   spec:
     modules:
     - name: sample
   ```

You can adjust this Kyma CR based on your testing scenario, if required. For example, if you are running a dual-cluster setup, you might want to enable the synchronization of the Kyma CR into the runtime cluster for a full E2E setup.
When creating this Kyma CR in your Control Plane cluster, installation of the specified modules should start immediately.

### Debugging the Operator Ecosystem

The operator ecosystem around Kyma is complex, and it might become troublesome to debug issues in case your module is not installed correctly.
For this reason here are some best practices on how to debug modules developed through this guide.

1. Verify the Kyma installation state is ready by verifying all conditions.
   ```shell
    JSONPATH='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.reason}:{@.status};{end}{end}' \
    && kubectl get kyma -o jsonpath="$JSONPATH" -n kcp-system
   ```
2. Verify the Manifest installation state is ready by verifying all conditions.
   ```shell
    JSONPATH='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}' \
    && kubectl get manifest -o jsonpath="$JSONPATH"-n kcp-system
   ```
3. Depending on your issue, either observe the deployment logs from either Lifecycle Manager or Module Manager. Make sure that no errors have occurred.

Usually, the issue is related to either RBAC configuration (for troubleshooting minimum privileges for the controllers, see our dedicated [RBAC](#rbac) section), misconfigured image, module registry or `ModuleTemplate`.
As a last resort, make sure that you are running within a single-cluster or a dual-cluster setup, watch out for any steps with a `WARNING` specified and retry with a freshly provisioned cluster.
For cluster provisioning, make sure to follow our recommendations for clusters mentioned in our [Prerequisites](#prerequisites) for this guide.

Lastly, if you are still unsure, open an issue with a description and steps to reproduce. We will be happy to help you with a solution.

### Registering your Module Within the Control Plane

For global usage of your module, the generated `template.yaml` from [Build and Push your Module to the Registry](#build-and-push-your-module-to-the-registry) must be registered in our Control Plane.
This relates to [Phase 2 of the module transition plane](https://github.com/kyma-project/community/blob/main/concepts/modularization/transition.md#phase-2---first-module-managed-by-kyma-operator-integrated-with-keb). Please be patient until we provide you with a stable guide on integrating your `template.yaml` properly with an automated test flow into the central Control Plane offering.