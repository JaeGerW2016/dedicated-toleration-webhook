#/bin/bash

if [ ! -x "$(command -v openssl)" ]; then
    echo "openssl not found"
    exit 1
fi

KS_DIR="./kustomize"
SECRET_DIR="${KS_DIR}/secrets"
KS_SERVICE="dtw-webhook-service"
: ${KS_NAMESPACE:=default}
: ${CERT_DAYS:=365}

echo "Apply manifests: namespace=${KS_NAMESPACE}"

atexit() {
  [[ -d "${SECRET_DIR}" ]] && rm -rf "${SECRET_DIR}"
  [[ -f "${KS_DIR}/kustomization.yaml" ]] && rm -f "${KS_DIR}/kustomization.yaml"
  [[ -f "${KS_DIR}/mutating-webhook-configuration.yaml" ]] && rm -f "${KS_DIR}/mutating-webhook-configuration.yaml"
}

mkdir -p "${SECRET_DIR}"
echo "creating certs in ${SECRET_DIR}"
trap atexit EXIT


# x509 outputs a self signed certificate instead of certificate request, later used as self signed root CA
openssl req -x509 -newkey rsa:2048 -keyout ${SECRET_DIR}/self_ca.key -out ${SECRET_DIR}/self_ca.crt -days ${CERT_DAYS} -nodes -subj /C=/ST=/L=/O=/OU=/CN=test-certificate-authority

cat <<EOF >> ${SECRET_DIR}/csr.conf
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${KS_SERVICE}
DNS.2 = ${KS_SERVICE}.${KS_NAMESPACE}
DNS.3 = ${KS_SERVICE}.${KS_NAMESPACE}.svc
EOF

openssl genrsa -out ${SECRET_DIR}/server-key.pem 2048

openssl req -new -key ${SECRET_DIR}/server-key.pem -subj "/CN=${KS_SERVICE}.${KS_NAMESPACE}.svc" -out ${SECRET_DIR}/server.csr -config ${SECRET_DIR}/csr.conf

# Self sign
openssl x509 -req -days ${CERT_DAYS} -in ${SECRET_DIR}/server.csr -CA ${SECRET_DIR}/self_ca.crt -CAkey ${SECRET_DIR}/self_ca.key -CAcreateserial -out ${SECRET_DIR}/server-cert.pem

# base64 encoded ca cert
KS_CA_BUNDLE=$(cat ${SECRET_DIR}/self_ca.crt | openssl enc -a -A)
# secrets from file
KS_CERT_PEM="secrets/server-cert.pem"
KS_KEY_PEM="secrets/server-key.pem"

cat <<EOF >> ${KS_DIR}/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
bases:
  - base
namespace: ${KS_NAMESPACE}
secretGenerator:
- name: dtw-webhook-certs
  namespace: ${KS_NAMESPACE}
  files:
    - cert.pem=${KS_CERT_PEM}
    - key.pem=${KS_KEY_PEM}
patches:
  - mutating-webhook-configuration.yaml
EOF

cat <<EOF >> ${KS_DIR}/mutating-webhook-configuration.yaml
apiVersion: admissionregistration.k8s.io/v1beta1
kind: MutatingWebhookConfiguration
metadata:
  name: mutating-webhook-configuration
webhooks:
- name: dtw.webhook.io
  clientConfig:
    caBundle: ${KS_CA_BUNDLE}
    service:
      namespace: ${KS_NAMESPACE}
EOF

echo "Call kustomize and kubectl..."

if [[ "$1" == "--dryrun" ]]; then
    kustomize build ./kustomize
else
    kustomize build ./kustomize | kubectl apply -f -
fi