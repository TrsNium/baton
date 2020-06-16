package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
