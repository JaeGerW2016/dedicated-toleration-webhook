#!/bin/bash

ROOT=$(cd $(dirname $0)/../../; pwd)

set -o errexit
set -o nounset
set -o pipefail

export CA_BUNDLE=$(kubectl get configmap -n kube-system extension-apiserver-authentication -o=jsonpath='{.data.client-ca-file}' | base64 | tr -d '\n')

cat <<EOF  >> k8s/mutatingwebhook-ca-bundle.yaml
apiVersion: admissionregistration.k8s.io/v1beta1
kind: MutatingWebhookConfiguration
metadata:
  name: mutating-webhook-configuration
webhooks:
  - name: dtw.webhook.io
    clientConfig:
      service:
        name: dedicated-toleration-webhook-service
        namespace: default
        path: "/apply-dtw"
      caBundle: ${CA_BUNDLE}
    rules:
      - operations: [ "CREATE" ]
        apiGroups: [""]
        apiVersions: ["v1"]
        resources: ["pods"]
      - operations: [ "CREATE" ]
        apiGroups: ["apps"]
        apiVersions: ["v1"]
        resources: ["deployments"]
EOF