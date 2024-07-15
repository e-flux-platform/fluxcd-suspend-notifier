package k8s

import (
	"fmt"
	"strings"
)

type ResourceType struct {
	Group   string `json:"group"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
}

type Resource struct {
	Type      ResourceType `json:"type"`
	Namespace string       `json:"namespace"`
	Name      string       `json:"name"`
}

func ResourceFromPath(path string) (Resource, error) {
	parts := strings.Split(path, "/")
	if len(parts) != 6 {
		return Resource{}, fmt.Errorf("unexpected path format: %s", path)
	}
	return Resource{
		Type: ResourceType{
			Group:   parts[0],
			Version: parts[1],
			Kind:    parts[4],
		},
		Namespace: parts[3],
		Name:      parts[5],
	}, nil
}
