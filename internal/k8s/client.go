package k8s

import (
	"context"
	"log/slog"
	"path"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client is a thing wrapper around the kubernetes client. It exposes functions relevant for checking what fluxcd
// resources exist, and what their underlying state look like.
type Client struct {
	client       *kubernetes.Clientset
	apiExtClient *clientset.Clientset
}

// NewClient instantiates and returns a Client
func NewClient(configPath string) (*Client, error) {
	var (
		config *rest.Config
		err    error
	)
	if configPath == "" {
		config, err = rest.InClusterConfig()
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", configPath)
	}
	if err != nil {
		return nil, err
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	apiExtClientSet, err := clientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Client{
		client:       clientSet,
		apiExtClient: apiExtClientSet,
	}, nil
}

// GetRawResource retrieves a raw resource from the kubernetes API. This is used to fetch fluxcd custom resources.
func (c *Client) GetRawResource(ctx context.Context, resource ResourceReference) ([]byte, error) {
	absPath := path.Join(
		"apis",
		resource.Type.Group,
		resource.Type.Version,
		"namespaces",
		resource.Namespace,
		resource.Type.Kind,
		resource.Name,
	)
	body, err := c.client.RESTClient().Get().AbsPath(absPath).DoRaw(ctx)
	if err != nil {
		slog.Warn("failed to fetch resource", slog.Any("error", err), slog.Any("path", absPath))
		return nil, err
	}
	return body, nil
}

// GetRawResources retrieves a list of raw resources
func (c *Client) GetRawResources(ctx context.Context, group ResourceType) ([]byte, error) {
	absPath := path.Join(
		"apis",
		group.Group,
		group.Version,
		group.Kind,
	)
	body, err := c.client.RESTClient().Get().AbsPath(absPath).DoRaw(ctx)
	if err != nil {
		slog.Warn("failed to fetch resources", slog.Any("error", err), slog.Any("path", absPath))
		return nil, err
	}
	return body, nil
}

// GetCustomResourceDefinitions fetches all custom resource definitions registered with the cluster.
func (c *Client) GetCustomResourceDefinitions(ctx context.Context, listOptions metav1.ListOptions) (*v1.CustomResourceDefinitionList, error) {
	return c.apiExtClient.
		ApiextensionsV1().
		CustomResourceDefinitions().
		List(ctx, listOptions)
}
