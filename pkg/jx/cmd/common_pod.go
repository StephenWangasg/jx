package cmd

import (
	"fmt"
	"time"

	"github.com/jenkins-x/jx/pkg/builds"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// WaitForReadyPodForDeployment waits for a pod of a deployment to be ready
func (o *CommonOptions) WaitForReadyPodForDeployment(c kubernetes.Interface, ns string, name string, names []string, readyOnly bool) (string, error) {
	deployment, err := c.AppsV1beta1().Deployments(ns).Get(name, metav1.GetOptions{})
	if err != nil || deployment == nil {
		return "", util.InvalidArg(name, names)
	}
	selector := deployment.Spec.Selector
	if selector == nil {
		return "", fmt.Errorf("No selector defined on Deployment %s in namespace %s", name, ns)
	}
	labels := selector.MatchLabels
	if labels == nil {
		return "", fmt.Errorf("No MatchLabels defined on the Selector of Deployment %s in namespace %s", name, ns)
	}
	return o.WaitForReadyPodForSelectorLabels(c, ns, labels, readyOnly)
}

// WaitForReadyPodForSelectorLabels waits for a pod selected by the given labels to be ready
func (o *CommonOptions) WaitForReadyPodForSelectorLabels(c kubernetes.Interface, ns string, labels map[string]string, readyOnly bool) (string, error) {
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: labels})
	if err != nil {
		return "", err
	}
	return o.WaitForReadyPodForSelector(c, ns, selector, readyOnly)
}

// WaitForReadyKnativeBuildPod waits for knative build pod to be ready
func (o *CommonOptions) WaitForReadyKnativeBuildPod(c kubernetes.Interface, ns string, readyOnly bool) (string, error) {
	log.Warnf("Waiting for a running Knative build pod in namespace %s\n", ns)
	lastPod := ""
	for {
		pods, err := builds.GetBuildPods(c, ns)
		if err != nil {
			return "", err
		}
		name := ""
		loggedInitContainerIdx := -1
		var latestPod *corev1.Pod
		lastTime := time.Time{}
		for _, pod := range pods {
			phase := pod.Status.Phase
			if phase == corev1.PodRunning || phase == corev1.PodPending {
				if !readyOnly {
					created := pod.CreationTimestamp
					if name == "" || created.After(lastTime) {
						lastTime = created.Time
						name = pod.Name
						latestPod = pod
					}
				}
			}
		}
		if latestPod != nil && name != "" {
			if name != lastPod {
				lastPod = name
				loggedInitContainerIdx = -1
				log.Warnf("Found newest pod: %s\n", name)
			}
			if kube.IsPodReady(latestPod) {
				return name, nil
			}

			initContainers := latestPod.Status.InitContainerStatuses
			for idx, ic := range initContainers {
				if isContainerStarted(&ic.State) && idx > loggedInitContainerIdx {
					loggedInitContainerIdx = idx
					containerName := ic.Name
					log.Warnf("Init container on pod: %s is: %s\n", name, containerName)
					err = o.TailLogs(ns, name, containerName)
					if err != nil {
						break
					}
				}
			}
		}
		// TODO replace with a watch flavour
		time.Sleep(time.Second)
	}
}

// WaitForReadyPodForSelector waits for a pod to which the selector applies to be ready
func (o *CommonOptions) WaitForReadyPodForSelector(c kubernetes.Interface, ns string, selector labels.Selector, readyOnly bool) (string, error) {
	log.Warnf("Waiting for a running pod in namespace %s with labels %v\n", ns, selector.String())
	lastPod := ""
	for {
		pods, err := c.CoreV1().Pods(ns).List(metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			return "", err
		}
		name := ""
		loggedInitContainerIdx := -1
		var latestPod *corev1.Pod
		lastTime := time.Time{}
		for _, pod := range pods.Items {
			phase := pod.Status.Phase
			if phase == corev1.PodRunning || phase == corev1.PodPending {
				if !readyOnly {
					created := pod.CreationTimestamp
					if name == "" || created.After(lastTime) {
						lastTime = created.Time
						name = pod.Name
						latestPod = &pod
					}
				}
			}
		}
		if latestPod != nil && name != "" {
			if name != lastPod {
				lastPod = name
				loggedInitContainerIdx = -1
				log.Warnf("Found newest pod: %s\n", name)
			}
			if kube.IsPodReady(latestPod) {
				return name, nil
			}

			initContainers := latestPod.Status.InitContainerStatuses
			for idx, ic := range initContainers {
				if isContainerStarted(&ic.State) && idx > loggedInitContainerIdx {
					loggedInitContainerIdx = idx
					containerName := ic.Name
					log.Warnf("Init container on pod: %s is: %s\n", name, containerName)
					err = o.TailLogs(ns, name, containerName)
					if err != nil {
						break
					}
				}
			}
		}
		// TODO replace with a watch flavour
		time.Sleep(time.Second)
	}
}
