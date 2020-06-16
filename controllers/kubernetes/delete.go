package kubernetes

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func DeletePod(c client.Client, pod corev1.Pod) error {
	ctx := context.Background()
	err := c.Delete(ctx, &pod)
	return err
}
