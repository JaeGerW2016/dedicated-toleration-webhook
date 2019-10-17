package main

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"os"
	"strings"
)

var (
	matchLabelKey      = os.Getenv("MATCH_LABEL_KEY")
	matchLabelValue    = os.Getenv("MATCH_LABEL_VALUE")
	tolerationKey      = os.Getenv("TOLERATION_KEY")
	tolerationOperator = corev1.TolerationOperator(os.Getenv("TOLERATION_OPERATOR"))
	tolerationValue    = os.Getenv("TOLERATION_VALUE")
	tolerationEffect   = corev1.TaintEffect(os.Getenv("TOLERATION_EFFECT"))
)

// Semantic can do semantic deep equality checks for core objects.
// Example: apiequality.Semantic.DeepEqual(aPod, aPodWithNonNilButEmptyMaps) == true
var Semantic = conversion.EqualitiesOrDie(
	func(a, b resource.Quantity) bool {
		// Ignore formatting, only care that numeric value stayed the same.
		// TODO: if we decide it's important, it should be safe to start comparing the format.
		//
		// Uninitialized quantities are equivalent to 0 quantities.
		return a.Cmp(b) == 0
	},
	func(a, b metav1.MicroTime) bool {
		return a.UTC() == b.UTC()
	},
	func(a, b metav1.Time) bool {
		return a.UTC() == b.UTC()
	},
	func(a, b labels.Selector) bool {
		return a.String() == b.String()
	},
	func(a, b fields.Selector) bool {
		return a.String() == b.String()
	},
)

func isMatchMetadataLabel(key string, value string) bool {
	return strings.EqualFold(key, matchLabelKey) && strings.EqualFold(value, matchLabelValue)
}

func addOrUpdateTolerationInPod(pod *corev1.Pod, toleration *corev1.Toleration) bool {
	podTolerations := pod.Spec.Tolerations

	var newTolerations []corev1.Toleration
	updated := false
	for i := range podTolerations {
		if toleration.MatchToleration(&podTolerations[i]) {
			if Semantic.DeepEqual(toleration, podTolerations[i]) {
				return false
			}
			newTolerations = append(newTolerations, *toleration)
			updated = true
			continue
		}

		newTolerations = append(newTolerations, podTolerations[i])
	}

	if !updated {
		newTolerations = append(newTolerations, *toleration)
	}

	pod.Spec.Tolerations = newTolerations
	return true
}

func addOrUpdateTolerationInDeployment(d *appsv1.Deployment, toleration *corev1.Toleration) bool {
	deploymentTolerations := d.Spec.Template.Spec.Tolerations

	var newTolerations []corev1.Toleration
	updated := false
	for i := range deploymentTolerations {
		if toleration.MatchToleration(&deploymentTolerations[i]) {
			if Semantic.DeepEqual(toleration, deploymentTolerations[i]) {
				return false
			}
			newTolerations = append(newTolerations, *toleration)
			updated = true
			continue
		}

		newTolerations = append(newTolerations, deploymentTolerations[i])
	}

	if !updated {
		newTolerations = append(newTolerations, *toleration)
	}

	d.Spec.Template.Spec.Tolerations = newTolerations
	return true
}
