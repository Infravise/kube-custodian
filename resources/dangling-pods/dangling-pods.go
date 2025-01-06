package danglingpods

import (
	"context"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func Cleanup(log *logrus.Logger, kubeClient *kubernetes.Clientset) {
	danglingPods := fetchDanglingPods(log, kubeClient)
	if len(danglingPods) == 0 {
		return
	}
	deleteDanglingPods(log, kubeClient, danglingPods)
}

func fetchDanglingPods(log *logrus.Logger, kubeClient *kubernetes.Clientset) []v1.Pod {
	// Fetch pods across all namespaces
	log.Infoln("Fetching all pods")
	pods, err := kubeClient.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Failed to retrieve pods: %v", err)
	}
	log.Infoln("Fetched all pods")
	// Initialize Slice
	danglingPods := []v1.Pod{}
	// Iterate over array of pods and retrieve name/status
	for _, pod := range pods.Items {
		if pod.Status.Phase == "Failed" || pod.Status.Phase == "Succeeded" {
			danglingPods = append(danglingPods, pod)
		}
	}
	log.Infof("Found %d dangling pod(s)", len(danglingPods))
	log.Debugf("Dangling pods: %v", danglingPods)
	return danglingPods
}

func deleteDanglingPods(log *logrus.Logger, kubeClient *kubernetes.Clientset, danglingPods []v1.Pod) {
	// Iterate over list of dangling pods and delete them 1 by 1
	log.Infoln("Deleting dangling pods")
	for _, pod := range danglingPods {
		log.Debugf("Deleting pod: %v", pod)
		err := kubeClient.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
		if err != nil {
			log.Errorf("Failed to delete dangling pod [%s]: %v", pod.Name, err)
		}
	}
	log.Infof("Deleted %d dangling pod(s)", len(danglingPods))
}
