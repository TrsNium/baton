package kubernetes

import (
	"context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetPod(c client.Client, namespace string, name string) (corev1.Pod, error) {
	ctx := context.Background()
	pod := corev1.Pod{}
	err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &pod)
	if err != nil {
		return corev1.Pod{}, err
	}
	return pod, nil
}

func GetNode(c client.Client, name string) (corev1.Node, error) {
	ctx := context.Background()
	node := corev1.Node{}
	err := c.Get(ctx, client.ObjectKey{Name: name}, &node)
	if err != nil {
		return corev1.Node{}, err
	}
	return node, nil
}

func GetDeployment(c client.Client, namespace string, name string) (appsv1.Deployment, error) {
	ctx := context.Background()
	deployment := appsv1.Deployment{}
	err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &deployment)
	if err != nil {
		return appsv1.Deployment{}, err
	}
	return deployment, nil
}
