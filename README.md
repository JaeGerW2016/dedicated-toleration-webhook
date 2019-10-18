# dedicated-toleration-webhook
Webhook implementation of DedicatedToleration plugin

## Prerequisites

Please enable the admission webhook feature [doc](https://kubernetes.io/docs/admin/extensible-admission-controllers/#enable-external-admission-webhooks).

## What is it?

It automatically adds tolerations if Pod or Deployment definition has labels

pod

```yaml
apiVersion: v1
kind: Pod
metadata:
 name: gpu-pod
 labels:
   component: nvidia-gpu
spec:
 containers:
   - name: gpu-container
     image: mirrorgcrio/pause:2.0
 nodeSelector:
   component: nvidia-gpu
```

into

```yaml
apiVersion: v1
kind: Pod
metadata:
 name: gpu-pod
 labels:
   component: nvidia-gpu
spec:
 containers:
   - name: gpu-container
     image: mirrorgcrio/pause:2.0
 nodeSelector:
   component: nvidia-gpu
 tolerations:
  - effect: NoSchedule
    key: dedicated
    operator: Equal
    value: gpu
```

deployment

```yaml
kind: Deployment
metadata:
  name: gpu-container-deployment
  labels:
    component: nvidia-gpu
spec:
  replicas: 1
  template:
    metadata:
      labels:
        component: nvidia-gpu
    spec:
      containers:
      - name: gpu-container
        image: mirrorgcrio/pause:2.0
      nodeSelector:
        component: nvidia-gpu
```

into

```yaml
kind: Deployment
metadata:
  name: gpu-container-deployment
  labels:
    component: nvidia-gpu
spec:
  replicas: 1
  template:
    metadata:
      labels:
        component: nvidia-gpu
    spec:
      containers:
      - name: gpu-container
        image: mirrorgcrio/pause:2.0
      nodeSelector:
        component: nvidia-gpu
      tolerations:
      - effect: NoSchedule
    	key: dedicated
    	operator: Equal
    	value: gpu
```

dynamically when scheduling to create a pod or deployment.

## Dedicated Node

```shell
git clone git@github.com:JaeGerW2016/dedicated-toleration-webhook.git
cd deployment
bash create-signed-cert.sh ##create secrets
bash patch-ca-bundle.sh  ## build caBundle
```



```
kubectl taint nodes nodename dedicated=nvidia-gpu:NoSchedule
```


#### `dedicated-toleration-webhook-deployment`

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  labels:
    app: dedicated-toleration-webhook
  name: dedicated-toleration-webhook-deployment
spec:
  selector:
    matchLabels:
      app: dedicated-toleration-webhook
  template:
    metadata:
      labels:
        app: dedicated-toleration-webhook
    spec:
      containers:
      - env:
        - name: MATCH_LABEL_KEY
          value: component
        - name: MATCH_LABEL_VALUE
          value: nvidia-gpu
        - name: TOLERATION_KEY
          value: dedicated
        - name: TOLERATION_OPERATOR
          value: Equal
        - name: TOLERATION_VALUE
          value: gpu
        - name: TOLERATION_EFFECT
          value: NoSchedule
        image: 314315960/dedicated-toleration-webhook:v1.3
        imagePullPolicy: Always
        name: dedicated-toleration-webhook
        ports:
        - containerPort: 443
        volumeMounts:
        - mountPath: /etc/certs
          name: cert
          readOnly: true
      volumes:
      - name: cert
        secret:
          secretName: dedicated-toleration-webhook
```
#### `dedicated-toleration-webhook-service`

```
apiVersion: v1
kind: Service
metadata:
  labels:
    app: dedicated-toleration-webhook
  name: dedicated-toleration-webhook-service
spec:
  ports:
  - port: 443
    targetPort: 443
  selector:
    app: dedicated-toleration-webhook
```

### `dedicated-toleration-webhook-mutating-webhook-configuration`

```
apiVersion: admissionregistration.k8s.io/v1beta1
kind: MutatingWebhookConfiguration
metadata:
  labels:
    app: dedicated-toleration-webhook
  name: dedicated-toleration-webhook-mutating-webhook-configuration
webhooks:
- clientConfig:
    caBundle: "up to your cluster caBundle" ## need to edit it
    service:
      name: dedicated-toleration-webhook-service
      namespace: default
      path: /apply-dtw
  name: dtw.webhook.io
  rules:
  - apiGroups:
    - ""
    apiVersions:
    - v1
    operations:
    - CREATE
    resources:
    - pods
  - apiGroups:
    - apps
    apiVersions:
    - v1
    operations:
    - CREATE
    resources:
    - deployments
```

### `example yaml`

```yaml
apiVersion: v1
kind: Pod
metadata:
 name: gpu-pod
 labels:
   component: nvidia-gpu
spec:
 containers:
   - name: gpu-container
     image: mirrorgcrio/pause:2.0
 nodeSelector:  
   component: nvidia-gpu   ##need equal MATCH_LABEL_KEY and MATCH_LABEL_VALUE
```



```yaml
apiVersion: apps/v1beta1 # for versions before 1.6.0 use extensions/v1beta1
kind: Deployment
metadata:
  name: gpu-container-deployment
  labels:
    component: nvidia-gpu
spec:
  replicas: 1
  template:
    metadata:
      labels:
        component: nvidia-gpu ##need equal MATCH_LABEL_KEY and MATCH_LABEL_VALUE
    spec:
      containers:
      - name: gpu-container
        image: mirrorgcrio/pause:2.0
      nodeSelector: 
        component: nvidia-gpu   ##need equal MATCH_LABEL_KEY and MATCH_LABEL_VALUE
```


