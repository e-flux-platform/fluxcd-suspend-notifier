package fluxcd

// Resource represents an abstract suspendable fluxcd resource. Only the fields relevant to this application are
// covered here
type Resource struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Suspend bool `json:"suspend"`
	} `json:"spec"`
}

// ResourceList represents a list of resources, aligned to how this would be presented by the kubernetes API
type ResourceList struct {
	Items []Resource `json:"items"`
}
