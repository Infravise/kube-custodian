package main

import (
	"context"
	"log"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Resource struct {
	ApiGroup          string
	ApiVersion        string
	ApiName           string
	Name              string
	Namespace         string
	CreationTimestamp metav1.Time
	Labels            map[string]string
}

func fetchDanglingPods(kubeClient *kubernetes.Clientset) []v1.Pod {
	// Get pods across all namespaces
	log.Println("INFO: Fetching all pods")
	pods, err := kubeClient.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Fatalf("ERROR: Failed to retrieve pods: %v", err)
	} else {
		log.Printf("INFO: Fetched all pods")
	}
	// Initialize Slice
	podsToDelete := []v1.Pod{}
	// Iterate over array of pods and retrieve name/status
	for _, pod := range pods.Items {
		if pod.Status.Phase == "Failed" || pod.Status.Phase == "Succeeded" {
			podsToDelete = append(podsToDelete, pod)
		}
	}
	// Return Slice
	return podsToDelete
}

func deleteDanglingPods(kubeClient *kubernetes.Clientset, danglingPods []v1.Pod) {
	// Iterate over list of dangling pods and delete them 1 by 1
	for _, pod := range danglingPods {
		log.Printf("Deleting Pod: [%s]...", pod.Name)
		err := kubeClient.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
		if err != nil {
			log.Printf("ERROR: Failed to delete pod [%s]: %v", pod.Name, err)
		}
	}
}

func fetchEphemeralResources(discoveryClient *discovery.DiscoveryClient, dynamicClient *dynamic.DynamicClient, targetLabel string) []Resource {
	// Fetch list of all API resources
	log.Println("INFO: Fetching API resources")
	apiResourceLists, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		log.Fatalf("ERROR: Failed to retrieve API resources, %v", err)
	}
	// Initialize slice
	resources := []Resource{}
	log.Printf("INFO: Fetching ephemeral resources [%s]", targetLabel)
	// Iterate over list of api resource lists
	for _, apiResourceList := range apiResourceLists {
		groupVersion := strings.Split(apiResourceList.GroupVersion, "/")
		// Interate over all possible resources within those lists
		for _, apiResource := range apiResourceList.APIResources {
			// Set resource schema
			gvr := schema.GroupVersionResource{
				// Use inline function to determine what value we use for Group and Version (No ternary operator in Go)
				Group: func() string {
					// In event resource is apart of CoreV1 group
					if len(groupVersion) == 1 {
						return apiResource.Group
					}
					return groupVersion[0]
				}(),
				Version: func() string {
					if len(groupVersion) == 1 {
						return groupVersion[0]
					}
					return groupVersion[1]
				}(),
				Resource: apiResource.Name,
			}
			// Fetch resources with given rbac verbs and label selector (Returns list of ephemeral resources)
			if slices.Contains(apiResource.Verbs, "get") && slices.Contains(apiResource.Verbs, "list") && slices.Contains(apiResource.Verbs, "delete") {
				resourceList, err := dynamicClient.Resource(gvr).Namespace("").List(context.TODO(), metav1.ListOptions{LabelSelector: targetLabel})
				if err != nil {
					log.Printf("WARN: Failed to retrieve ephemeral resources [%s.%s], %v", apiResourceList.GroupVersion, apiResource.Name, err)
				}
				// Iterate over found resources and append them to slice
				if resourceList != nil {
					for _, resource := range resourceList.Items {
						kubeResource := Resource{
							ApiGroup:          gvr.Group,
							ApiVersion:        gvr.Version,
							ApiName:           gvr.Resource,
							Name:              resource.GetName(),
							Namespace:         resource.GetNamespace(),
							CreationTimestamp: resource.GetCreationTimestamp(),
							Labels:            resource.GetLabels(),
						}
						resources = append(resources, kubeResource)
					}
				}
			}
		}
	}
	// Return list of ephemeral resources
	return resources
}

func fetchExpiredResources(ephemeralResources []Resource) []Resource {
	// Set current time and initialize resource slice
	currentTime := time.Now()
	resoucesForDeletion := []Resource{}
	log.Print("INFO: Processing ephemeral resources")
	// Iterate over list of resources and mark them for deletion if timestamp logic is true
	for _, resource := range ephemeralResources {
		creationTime := resource.CreationTimestamp
		if value, exists := resource.Labels["kube-custodian/ttl"]; exists {
			re := regexp.MustCompile(`(\d+)([wdhm])`)
			matches := re.FindAllStringSubmatch(value, -1)
			var weeks, days, hours, minutes int
			for _, match := range matches {
				// Convert the number to an integer
				value, _ := strconv.Atoi(match[1])
				// Extract the unit (w, d, h, m, s)
				unit := match[2]
				// Assign the value to the correct unit
				switch unit {
				case "w":
					weeks = value
				case "d":
					days = value
				case "h":
					hours = value
				case "m":
					minutes = value
				}
			}
			// Calculate the total duration
			duration := time.Duration(weeks*7*24)*time.Hour +
				time.Duration(days*24)*time.Hour +
				time.Duration(hours)*time.Hour +
				time.Duration(minutes)*time.Minute
			// Add given value to resources creation time
			destructionTime := creationTime.Add(duration)
			// If destruction time is < current time, mark resource for deletion
			if destructionTime.Before(currentTime) {
				resoucesForDeletion = append(resoucesForDeletion, resource)
			}
		} else if value, exists := resource.Labels["kube-custodian/expires"]; exists {
			expireTime, err := time.Parse(time.RFC3339, value)
			if err != nil {
				log.Printf("ERROR: Failed to convert expires string to time.Time [%s/%s.%s], %v", resource.ApiGroup, resource.ApiVersion, resource.Name, err)
			}
			if expireTime.Before(currentTime) {
				resoucesForDeletion = append(resoucesForDeletion, resource)
			}
		} else {
			log.Print("INFO: No expired resources found")
		}
	}
	return resoucesForDeletion
}

func deleteEphemeralResources(dynamicClient *dynamic.DynamicClient, expiredResources []Resource) {
	log.Print("INFO: ")
	for _, resource := range expiredResources {
		gvr := schema.GroupVersionResource{
			Group:    resource.ApiGroup,
			Version:  resource.ApiVersion,
			Resource: resource.ApiName,
		}
		err := dynamicClient.Resource(gvr).Namespace(resource.Namespace).Delete(context.TODO(), resource.Name, metav1.DeleteOptions{})
		if err != nil {
			log.Printf("ERROR: Failed to destroy ephemeral resource [%s/%s:%s], %v", resource.ApiGroup, resource.ApiVersion, resource.Name, err)
		}
	}
}

func main() {
	// Create tmp/health file for probes
	file, err := os.Create("/tmp/health")
	if err == nil {
		file.WriteString("healthy")
		file.Close()
	} else {
		log.Fatalf("ERROR: Failed to generate file for health check, %v", err)
	}
	// Create the in-cluster config
	log.Println("INFO: Creating kubernetes client config")
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("ERROR: Failed to create client config, %v", err)
	}
	// Create the Discovery Client
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		log.Fatalf("ERROR: Failed to create client [Discovery], %v", err)
	}
	// Create the Dynamic Client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Fatalf("ERROR: Failed to create client [Dynamic], %v", err)
	}
	// Create the Kubernetes Client
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("ERROR: Failed to create client [Kubernetes], %v", err)
	}

	for {
		// Fetch pods (Failed/Succeeded)
		pods := fetchDanglingPods(kubeClient)
		if len(pods) == 0 {
			log.Print("INFO: No pods need to be cleaned")
		} else {
			// Delete pods
			deleteDanglingPods(kubeClient, pods)
		}
		ttlResources := fetchEphemeralResources(discoveryClient, dynamicClient, "kube-custodian/ttl")
		ttlToDelete := fetchExpiredResources(ttlResources)
		if len(ttlToDelete) == 0 {
			log.Print("INFO: No ephemeral resources need to be cleaned")
		} else {
			deleteEphemeralResources(dynamicClient, ttlToDelete)
		}
		expiresResources := fetchEphemeralResources(discoveryClient, dynamicClient, "kube-custodian/expires")
		expiresToDelete := fetchExpiredResources(expiresResources)
		if len(expiresToDelete) == 0 {
			log.Print("INFO: No ephemeral resources need to be cleaned")
		} else {
			deleteEphemeralResources(dynamicClient, expiresToDelete)
		}

		// Sleep 30 seconds then loop again
		time.Sleep(30 * time.Second)
	}
}
