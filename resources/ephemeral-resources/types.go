package ephemeralresources

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EphemeralResource struct {
	Group             string
	Version           string
	Resource          string
	Kind              string
	Name              string
	Namespace         string
	CreationTimestamp metav1.Time
	Labels            map[string]string
}
