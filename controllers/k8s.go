package controllers

import (
	"context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetDeployment(c client.Client, namespace string, name string) (appsv1.Deployment, error) {
	ctx := context.Background()
	deployment := appsv1.Deployment{}
	err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &deployment)
	if err != nil {
		return appsv1.Deployment{}, err
	}
	return deployment, nil
}

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

func DeletePod(c client.Client, pod corev1.Pod) error {
	ctx := context.Background()
	err := c.Delete(ctx, &pod)
	return err
}

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

func GetNodes(c client.Client) ([]corev1.Node, error) {
	ctx := context.Background()
	nodes := corev1.NodeList{}
	err := c.List(ctx, &nodes)
	if err != nil {
		return nil, err
	}
	return nodes.Items, nil
}

// RunCordonOrUncordon demonstrates the canonical way to cordon or uncordon a Node
func RunCordonOrUncordon(c client.Client, node *corev1.Node, desired bool) error {
	// TODO(justinsb): Ensure we have adequate e2e coverage of this function in library consumers
	h := NewCordonHelper(node)

	if updateRequired := h.UpdateIfRequired(desired); !updateRequired {
		// Already done
		return nil
	}

	err, patchErr := h.PatchOrReplace(c)
	if patchErr != nil {
		return patchErr
	}

	if err != nil {
		return err
	}

	return nil
}

func FilterNodes(nodes []corev1.Node, f func(corev1.Node) bool) []corev1.Node {
	filteredNodes := []corev1.Node{}
	for _, node := range nodes {
		if f(node) {
			filteredNodes = append(filteredNodes, node)
		}
	}
	return filteredNodes
}

func FilterPods(pods []corev1.Pod, f func(corev1.Pod) bool) []corev1.Pod {
	filterdPods := []corev1.Pod{}
	for _, pod := range pods {
		if f(pod) {
			filterdPods = append(filterdPods, pod)
		}
	}
	return filterdPods
}
