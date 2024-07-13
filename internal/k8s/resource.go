package k8s

import (
	"fmt"
	"strings"
)

type Resource struct {
	Namespace string
	Kind      string
	Name      string
	Path      string
}

func ResourceFromPath(path string) (Resource, error) {
	parts := strings.Split(path, "/")
	if len(parts) != 6 {
		return Resource{}, fmt.Errorf("unexpected path format: %s", path)
	}
	return Resource{
		Namespace: parts[3],
		Kind:      strings.TrimSuffix(parts[4], "s"),
		Name:      parts[5],
		Path:      path,
	}, nil
}
