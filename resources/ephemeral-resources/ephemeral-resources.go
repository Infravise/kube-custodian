package ephemeralresources

import (
	"context"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
)

func Cleanup(log *logrus.Logger, discoveryClient *discovery.DiscoveryClient, dynamicClient *dynamic.DynamicClient) {
	labels := []string{"kube-custodian/ttl", "kube-custodian/expires"}
	ephemeralResources := []EphemeralResource{}
	for _, label := range labels {
		ephemeralResources = append(ephemeralResources, fetchEphemeralResources(log, discoveryClient, dynamicClient, label)...)
	}
	if len(ephemeralResources) == 0 {
		return
	}
	expiredResources := processEphemeralResources(log, ephemeralResources)
	if len(expiredResources) == 0 {
		return
	}
	deleteEphemeralResources(log, dynamicClient, expiredResources)
}

func fetchEphemeralResources(log *logrus.Logger, discoveryClient *discovery.DiscoveryClient, dynamicClient *dynamic.DynamicClient, targetLabel string) []EphemeralResource {
	// Fetch list of all API resources
	log.Infoln("Fetching API resources")
	apiResourceLists, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		log.Fatalf("Failed to retrieve API resources, %v", err)
	}
	// Initialize slice
	ephemeralResources := []EphemeralResource{}
	log.Infof("Fetching ephemeral resources [%s]", targetLabel)
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
					log.Errorf("Failed to retrieve ephemeral resources [%s.%s], %v", apiResourceList.GroupVersion, apiResource.Name, err)
				}
				// Iterate over found resources and append them to slice
				if resourceList != nil {
					for _, resource := range resourceList.Items {
						kubeResource := EphemeralResource{
							Group:             gvr.Group,
							Version:           gvr.Version,
							Resource:          gvr.Resource,
							Kind:              resource.GetKind(),
							Name:              resource.GetName(),
							Namespace:         resource.GetNamespace(),
							CreationTimestamp: resource.GetCreationTimestamp(),
							Labels:            resource.GetLabels(),
						}
						ephemeralResources = append(ephemeralResources, kubeResource)
					}
				}
			}
		}
	}
	log.Infof("Found %d ephemeral reources", len(ephemeralResources))
	log.Debugf("Ephemeral resources: %v", ephemeralResources)
	return ephemeralResources
}

func processEphemeralResources(log *logrus.Logger, ephemeralResources []EphemeralResource) []EphemeralResource {
	// Set current time and initialize resource slice
	currentTime := time.Now()
	expiredResources := []EphemeralResource{}
	log.Infoln("Processing ephemeral resources")
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
				expiredResources = append(expiredResources, resource)
			}
		} else if value, exists := resource.Labels["kube-custodian/expires"]; exists {
			expireTime, err := time.Parse(time.RFC3339, value)
			if err != nil {
				log.Errorf("Failed to convert expires string to time.Time [%s/%s.%s], %v", resource.Group, resource.Version, resource.Name, err)
			}
			if expireTime.Before(currentTime) {
				expiredResources = append(expiredResources, resource)
			}
		}
	}
	log.Infof("Found %d expired resources. Preparing for deletion", len(expiredResources))
	log.Debugf("Expired resources: %v", expiredResources)
	return expiredResources
}

func deleteEphemeralResources(log *logrus.Logger, dynamicClient *dynamic.DynamicClient, expiredResources []EphemeralResource) {
	log.Infoln("Deleting expired resources")
	for _, resource := range expiredResources {
		gvr := schema.GroupVersionResource{
			Group:    resource.Group,
			Version:  resource.Version,
			Resource: resource.Resource,
		}
		log.Debugf("Deleting expired resource: %v", resource)
		err := dynamicClient.Resource(gvr).Namespace(resource.Namespace).Delete(context.TODO(), resource.Name, metav1.DeleteOptions{})
		if err != nil {
			log.Errorf("Failed to destroy ephemeral resource [%s/%s:%s], %v", resource.Group, resource.Version, resource.Name, err)
		}
	}
	log.Infof("Deleted %d expired resources", len(expiredResources))
}
