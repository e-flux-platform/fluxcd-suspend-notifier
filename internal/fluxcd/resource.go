package fluxcd

type Resource struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Suspend bool `json:"suspend"`
	} `json:"spec"`
}

type ResourceList struct {
	Items []Resource `json:"items"`
}
