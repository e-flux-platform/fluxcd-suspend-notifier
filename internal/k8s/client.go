package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	client *kubernetes.Clientset
}

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

	return &Client{
		client: clientSet,
	}, nil
}

func (c *Client) GetRawResource(ctx context.Context, relativePath string) (map[string]any, error) {
	body, err := c.client.RESTClient().Get().AbsPath(path.Join("apis", relativePath)).DoRaw(ctx)
	if err != nil {
		slog.Warn("failed to fetch resource", slog.Any("error", err), slog.Any("path", path.Join("apis", relativePath)))
		return nil, err
	}

	var res map[string]any
	if err = json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("invalid response: %w", err)
	}
	return res, nil
}
