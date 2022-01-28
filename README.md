# OpenShift cluster-api-provider-powervs

This repository hosts an implementation of a provider for PowerVS for the
OpenShift [machine-api](https://github.com/openshift/cluster-api).

This provider runs as a machine-controller deployed by the
[machine-api-operator](https://github.com/openshift/machine-api-operator)

### How to build the images in the RH infrastructure
The Dockerfiles use `as builder` in the `FROM` instruction which is not currently supported
by the RH's docker fork (see [https://github.com/kubernetes-sigs/kubebuilder/issues/268](https://github.com/kubernetes-sigs/kubebuilder/issues/268)).
One needs to run the `imagebuilder` command instead of the `docker build`.

Note: this info is RH only, it needs to be backported every time the `README.md` is synced with the upstream one.

## Deploy machine API plane with kubernetes cluster

1. **Deploying the cluster**

    Use any existing mechanism for deploying the kubernetes cluster, e.g: kubeadm https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/

2. **Deploying machine API controllers**

    For development purposes the powervs machine controller itself will run out of the machine API stack.
    Otherwise, docker images needs to be built, pushed into a docker registry and deployed within the stack.

    To deploy the stack:
    ```
    kustomize build config | kubectl apply -f -
    ```

3. **Deploy secret with Power VS credentials**

   Power VS nodeupdate controller assumes existence of a secret file:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: powervs-credentials
     namespace: openshift-machine-api
   type: Opaque
   data:
     ibmcloud_api_key: FILLIN
   ```

   You can use `examples/render-powervs-secrets.sh` script to generate the secret:
   ```sh
   # Set the IBMCLOUD_API_KEY with a proper IBM Cloud API Key
   $ IBMCLOUD_API_KEY=<API_KEY> ./examples/render-powervs-secrets.sh examples/addons.yaml | kubectl apply -f -
   ```

   Go to [How to create IBM Cloud API Key](#How-to-create-IBM-Cloud-API-Key) for creating API Key

4. **Test by creating example machine**

   ```shell
   # Update the relevant fields like serviceInstanceID, imageID, subnets, keyName etc.. 
   $ kubectl create -f examples/machine-with-user-data.yaml
   $ kubectl create -f examples/userdata.yml
   ```
## Test locally built powervs actuator

1. **Tear down machine-controller**

   Deployed machine API plane (`machine-api-controllers` deployment) is (among other
   controllers) running `machine-controller`. In order to run locally built one,
   simply edit `machine-api-controllers` deployment and remove `machine-controller` container from it.


2. **Build and run powervs actuator from outside the cluster**

   ```sh
   $ go build -o bin/machine-controller-manager github.com/openshift/cluster-api-provider-powervs/cmd/manager
   ```

   ```sh
   $ .bin/machine-controller-manager --kubeconfig ~/.kube/config --logtostderr -v 5 -alsologtostderr
   ```
      If running in container with `podman`, or locally without `docker` installed, and encountering issues, see [hacking-guide](https://github.com/openshift/machine-api-operator/blob/master/docs/dev/hacking-guide.md#troubleshooting-make-targets).

## How to create IBM Cloud API Key

### Method 1: Create an API Key via IAM
https://cloud.ibm.com/docs/account?topic=account-userapikey&locale=en#create_user_key

### Method 2: Create an API Key via Service Account
<!TODO: Add information>
