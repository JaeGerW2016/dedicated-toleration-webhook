package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/mattbaird/jsonpatch"
	"io/ioutil"
	"k8s.io/api/admission/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog"
	"net/http"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

type Config struct {
	CertFile string
	KeyFile  string
}

type admitFunc func(v1beta1.AdmissionReview) *v1beta1.AdmissionResponse

func configTLS(config Config) *tls.Config {
	sCert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		klog.Fatalf("config=%#v Error: %v", config, err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{sCert},
	}
}

func toAdmissionResponse(err error) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
		},
	}
}

func apply(ar v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	klog.Info("Entering apply in DedicatedToleration webhook")
	podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if ar.Request.Resource == podResource {
		//err := fmt.Errorf("expect resource to be %s", podResource)
		//klog.Error(err)
		//return toAdmissionResponse(err)
		raw := ar.Request.Object.Raw
		pod := corev1.Pod{}
		if _, _, err := deserializer.Decode(raw, nil, &pod); err != nil {
			klog.Error(err)
			return toAdmissionResponse(err)
		}
		reviewResponse := v1beta1.AdmissionResponse{}
		reviewResponse.Allowed = true
		podCopy := pod.DeepCopy()
		klog.V(1).Infof("Examining pod: %v\n", pod.GetName())

		if podAnnotations := pod.GetAnnotations(); podAnnotations != nil {
			klog.Info(fmt.Sprintf("Looking at pod annotations, found: %v", podAnnotations))
			if _, isMirrorPod := podAnnotations[corev1.MirrorPodAnnotationKey]; isMirrorPod {
				return &reviewResponse
			}
		}

		for k, v := range pod.ObjectMeta.Labels {
			if isMatchMetadataLabel(k, v) {
				if !addOrUpdateTolerationInPod(&pod, &corev1.Toleration{
					Key:      tolerationKey,
					Value: tolerationValue,
					Operator: tolerationOperator,
					Effect:   tolerationEffect,
				}) {
					return &reviewResponse
				}
				klog.Infof("applied dedicatedtoleration: %s successfully on Pod: %+v ", tolerationKey, pod.GetName())
			}
		}
		podCopyJSON, err := json.Marshal(podCopy)
		if err != nil {
			return toAdmissionResponse(err)
		}
		podJSON, err := json.Marshal(pod)
		if err != nil {
			return toAdmissionResponse(err)
		}
		klog.Infof("PodCopy json: %s ", podCopyJSON)
		klog.Infof("pod json: %s ", podJSON)
		jsonPatch, err := jsonpatch.CreatePatch(podCopyJSON, podJSON)
		if err != nil {
			klog.Infof("patch error: %+v", err)
			return toAdmissionResponse(err)
		}
		jsonPatchBytes, _ := json.Marshal(jsonPatch)
		klog.Infof("jsonPatch json: %s", jsonPatchBytes)

		reviewResponse.Patch = jsonPatchBytes
		pt := v1beta1.PatchTypeJSONPatch
		reviewResponse.PatchType = &pt
		return &reviewResponse
	}

	deploymentResource := metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	if ar.Request.Resource == deploymentResource {
		//err := fmt.Errorf("expect resource to be %s", deploymentResource)
		//klog.Error(err)
		//return toAdmissionResponse(err)
		raw := ar.Request.Object.Raw
		deployment := appsv1.Deployment{}
		if _, _, err := deserializer.Decode(raw, nil, &deployment); err != nil {
			klog.Error(err)
			return toAdmissionResponse(err)
		}

		reviewResponse := v1beta1.AdmissionResponse{}
		reviewResponse.Allowed = true
		deploymentCopy := deployment.DeepCopy()
		klog.V(1).Infof("Examining deployment: %v\n", deployment.GetName())

		for k,v := range deployment.Labels {
			if isMatchMetadataLabel(k, v) {
				if !addOrUpdateTolerationInDeployment(&deployment, &corev1.Toleration{
					Key:      tolerationKey,
					Value: tolerationValue,
					Operator: tolerationOperator,
					Effect:   tolerationEffect,
				}) {
					return &reviewResponse
				}
				klog.Infof("applied dedicatedtoleration: %s successfully on deployment: %+v ", tolerationKey, deployment.GetName())
			}
		}
		deploymentCopyJSON, err := json.Marshal(deploymentCopy)
		if err != nil {
			return toAdmissionResponse(err)
		}
		deploymentJSON, err := json.Marshal(deployment)
		if err != nil {
			return toAdmissionResponse(err)
		}
		klog.Infof("deploymentCopy json: %s ", deploymentCopyJSON)
		klog.Infof("deployment json: %s ", deploymentJSON)
		jsonPatch, err := jsonpatch.CreatePatch(deploymentCopyJSON, deploymentJSON)
		if err != nil {
			klog.Infof("patch error: %+v", err)
			return toAdmissionResponse(err)
		}
		jsonPatchBytes, _ := json.Marshal(jsonPatch)
		klog.Infof("jsonPatch json: %s", jsonPatchBytes)

		reviewResponse.Patch = jsonPatchBytes
		pt := v1beta1.PatchTypeJSONPatch
		reviewResponse.PatchType = &pt
		return &reviewResponse
	}
	return nil
}

func serve(w http.ResponseWriter, r *http.Request, admit admitFunc) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		klog.Errorf("contentType=%s, expect application/json", contentType)
		return
	}

	var reviewRespone *v1beta1.AdmissionResponse
	ar := v1beta1.AdmissionReview{}
	if _,_,err := deserializer.Decode(body, nil,&ar); err != nil {
		klog.Error(err)
		reviewRespone = toAdmissionResponse(err)
	} else {
		reviewRespone = admit(ar)
	}

	response := v1beta1.AdmissionReview{}
	if reviewRespone != nil {
		response.Response = reviewRespone
		response.Response.UID = ar.Request.UID
	}

	ar.Request.Object =  runtime.RawExtension{}
	ar.Request.OldObject = runtime.RawExtension{}

	resp, err := json.Marshal(response)
	if err != nil {
		klog.Error(err)
	}
	if _,err := w.Write(resp); err != nil {
		klog.Error(err)
	}
}

func serveDTW(w http.ResponseWriter, r *http.Request) {
	serve(w, r, apply)
}

func main() {
	var config Config
	flag.StringVar(&config.CertFile, "tlsCertFile", "/etc/certs/cert.pem", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&config.KeyFile, "tlsKeyFile", "/etc/certs/key.pem", "File containing the x509 private key to --tlsCertFile.")
	flag.Parse()
	klog.InitFlags(nil)

	http.HandleFunc("/apply-dtw",serveDTW)

	server := &http.Server{
		Addr: ":443",
		TLSConfig: configTLS(config),
	}

	klog.Info(fmt.Sprintf("About to start serving webhooks: %#v", server))
	if err := server.ListenAndServeTLS("",""); err != nil {
		klog.Errorf("Failed to listen and server webhook server:%v", err)
	}
}
