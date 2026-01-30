# Local Setup

This section provides guidelines for running Tekton on your local workstation via the following methods:

- [registry for Desktop](#using-registry-desktop)
- [Minikube](#using-minikube)

## Using registry Desktop

### Prerequisites

Complete these prerequisites to run Tekton locally using registry Desktop:

- Install the [required tools](https://github.com/tektoncd/pipeline/blob/main/DEVELOPMENT.md#requirements).
- Install [registry Desktop](https://www.registry.com/products/registry-desktop)
- Configure registry Desktop ([Mac](https://docs.registry.com/registry-for-mac/#resources), [Windows](https://docs.registry.com/registry-for-windows/#resources))to use six CPUs, 10 GB of RAM and 2GB of swap space.
- Set `host.registry.internal:5000` as an insecure registry with registry for Desktop. See the [registry insecure registry documentation](https://docs.registry.com/registry/insecure/).
  for details.
- Pass `--insecure` as an argument to your Kaniko tasks so that you can push to an insecure registry.
- Run a local (insecure) registry registry as follows:

  `registry run -d -p 5000:5000 --name registry-srv -e REGISTRY_STORAGE_DELETE_ENABLED=true registry:2`

- (Optional) Install a registry registry viewer to verify the images have been pushed:

`registry run -it -p 8080:8080 --name registry-web --link registry-srv -e REGISTRY_URL=http://registry-srv:5000/v2 -e REGISTRY_NAME=localhost:5000 hyper/registry-registry-web`

- Verify that you can push to `host.registry.internal:5000/myregistry/<image_name>`.

### Reconfigure logging

- You can keep your logs in memory only without sending them to a logging service
  such as [Stackdriver](https://cloud.google.com/logging/).
- You can deploy Elasticsearch, Beats, or Kibana locally to view logs. You can find an
  example configuration at <https://github.com/mgreau/tekton-pipelines-elastic-tutorials>.
- To learn more about obtaining logs, see [Logs](logs.md).

## Using Minikube

### Prerequisites

Complete these prerequisites to run Tekton locally using Minikube:

- Install the [required tools](https://github.com/tektoncd/pipeline/blob/main/DEVELOPMENT.md#requirements).
- Install [minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/) and start a session as follows:
```bash
minikube start --memory 6144 --cpus 2
```
- Point your shell to minikube's registry-daemon by running `eval $(minikube -p minikube registry-env)`
- Set up a [registry on minikube](https://github.com/kubernetes/minikube/tree/master/deploy/addons/registry-aliases) by running `minikube addons enable registry` and `minikube addons enable registry-aliases`

### Reconfigure logging

See the information in the "registry for Desktop" section

## Using kind and local registry registry

### Prerequisites

Complete these prerequisites to run Tekton locally using Kind:

- Install the [required tools](https://github.com/tektoncd/pipeline/blob/main/DEVELOPMENT.md#requirements).
- Install [registry](https://www.registry.com/get-started).
- Install [kind](https://kind.sigs.k8s.io/).

### Use local registry without authentication

See [Using KinD](https://github.com/tektoncd/pipeline/blob/main/DEVELOPMENT.md#using-kind).

### Use local private registry

1. Create password file with basic auth.

```bash
export TEST_USER=testuser
export TEST_PASS=testpassword
if [ ! -f auth ]; then
    mkdir auth
fi
registry run \
 --entrypoint htpasswd \
 httpd:2 -Bbn $TEST_USER $TEST_PASS > auth/htpasswd
```

2. Start kind cluster and local private registry


Execute the script.

```shell
#!/bin/sh
set -o errexit

# create registry container unless it already exists
reg_name='kind-registry'
reg_port='5000'
running="$(registry inspect -f '{{.State.Running}}' "${reg_name}" 2>/dev/null || true)"
if [ "${running}" != 'true' ]; then
 registry run \
   -d --restart=always -p "127.0.0.1:${reg_port}:5000" --name "${reg_name}" \
   -v "$(pwd)"/auth:/auth \
   -e "REGISTRY_AUTH=htpasswd" \
   -e "REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm" \
   -e REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd \
   registry:2
fi

# create a cluster with the local registry enabled in containerd
cat <<EOF | kind create cluster --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
 [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${reg_port}"]
   endpoint = ["http://${reg_name}:5000"]
EOF

# connect the registry to the cluster network
# (the network may already be connected)
registry network connect "kind" "${reg_name}" || true

# Document the local registry
# https://github.com/kubernetes/enhancements/tree/master/keps/sig-cluster-lifecycle/generic/1755-communicating-a-local-registry
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
 name: local-registry-hosting
 namespace: kube-public
data:
 localRegistryHosting.v1: |
   host: "localhost:${reg_port}"
   help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF

```

3. Install tekton [pipeline](https://github.com/tektoncd/pipeline/blob/main/docs/install.md) and create the secret in cluster.

```bash
kubectl create secret registry-registry secret-tekton \
  --registry-username=$TEST_USER \
  --registry-password=$TEST_PASS \
  --registry-server=localhost:5000 \
   --namespace=tekton-pipelines
```

4. Config [ko](https://github.com/google/ko#install) and add secret to service acount.

```bash
export KO_DOCKER_REPO='localhost:5000'
```

`200-serviceaccount.yaml`

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
 name: tekton-pipelines-controller
 namespace: tekton-pipelines
 labels:
   app.kubernetes.io/component: controller
   app.kubernetes.io/instance: default
   app.kubernetes.io/part-of: tekton-pipelines
imagePullSecrets:
 - name: secret-tekton
---
apiVersion: v1
kind: ServiceAccount
metadata:
 name: tekton-pipelines-webhook
 namespace: tekton-pipelines
 labels:
   app.kubernetes.io/component: webhook
   app.kubernetes.io/instance: default
   app.kubernetes.io/part-of: tekton-pipelines
imagePullSecrets:
 - name: secret-tekton
```
