package controllers

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetDeployment(c client.Client, namespace string, name string) (appsv1.Deployment, error) {
	ctx := context.Background()
	deployment := appsv1.Deployment{}
	err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &deployment)
	if err != nil {
		return nil, err
	}
	return deployment, err
}

func ListPodMatchLabels(c client.Client, namespace string, labels map[string]string) ([]corev1.Pod, error) {
	ctx := context.Background()
	podList := corev1.PodList{}
	err := c.List(ctx, &podList,
		client.InNamespace(deployment.ObjectMeta.Namespace),
		client.MatchingLabels(deployment.Spec.Template.ObjectMeta.Labels),
	)

	if err != nil {
		return nil, err
	}
	return podList.Items, nil
}

func DeletePod(c client.Client, pod corev1.Pod) error {
	ctx := context.Background()
	err := c.Delete(ctx, pod)
	return err
}

func GetPod(c client.Client, namespace string, name string) (corev1.Pod, error) {
	ctx := context.Background()
	ctx := context.Background()
	pod := corev1.Pod{}
	c.Get(ctx, client.ObjectKey{Namespace: deploymentInfo.Namespace, Name: deploymentInfo.Name}, &pod)
	if err != nil {
		return nil, err
	}
	return pod, nil
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

	err, patchErr := h.PatchOrReplace(c, false)
	if patchErr != nil {
		return patchErr
	}
	if err != nil {
		return err
	}

	return nil
}
