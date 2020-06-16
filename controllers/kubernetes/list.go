package kubernetes

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ListPodMatchLabels(c client.Client, namespace string, labels map[string]string) ([]corev1.Pod, error) {
	ctx := context.Background()
	podList := corev1.PodList{}
	err := c.List(ctx, &podList,
		client.InNamespace(namespace),
		client.MatchingLabels(labels),
	)

	if err != nil {
		return nil, err
	}
	return podList.Items, nil
}

func ListNodeMatchLabels(c client.Client, labels map[string]string) ([]corev1.Node, error) {
	ctx := context.Background()
	nodeList := corev1.NodeList{}
	err := c.List(ctx, &nodeList,
		client.MatchingLabels(labels),
	)

	if err != nil {
		return nil, err
	}
	return nodeList.Items, nil
}

func ListNodes(c client.Client) ([]corev1.Node, error) {
	ctx := context.Background()
	nodes := corev1.NodeList{}
	err := c.List(ctx, &nodes)
	if err != nil {
		return nil, err
	}
	return nodes.Items, nil
}
