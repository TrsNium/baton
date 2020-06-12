// hommage from https://github.com/kubernetes/kubectl/blob/master/pkg/drain/cordon.go

package controllers

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CordonHelper wraps functionality to cordon/uncordon nodes
type CordonHelper struct {
	node    *corev1.Node
	desired bool
}

func NewCordonHelper(node *corev1.Node) *CordonHelper {
	return &CordonHelper{
		node: node,
	}
}

// UpdateIfRequired returns true if c.node.Spec.Unschedulable isn't already set,
// or false when no change is needed
func (c *CordonHelper) UpdateIfRequired(desired bool) bool {
	c.desired = desired

	return c.node.Spec.Unschedulable != c.desired
}

// PatchOrReplace uses given clientset to update the node status, either by patching or
// updating the given node object; it may return error if the object cannot be encoded as
// JSON, or if either patch or update calls fail; it will also return a second error
// whenever creating a patch has failed
func (c *CordonHelper) PatchOrReplace(client client.Client, serverDryRun bool) (error, error) {
	oldData, err := json.Marshal(c.node)
	if err != nil {
		return err, nil
	}

	c.node.Spec.Unschedulable = c.desired

	newData, err := json.Marshal(c.node)
	if err != nil {
		return err, nil
	}

	patchBytes, patchErr := strategicpatch.CreateTwoWayMergePatch(oldData, newData, c.node)
	if patchErr == nil {
		patchOptions := metav1.PatchOptions{}
		if serverDryRun {
			patchOptions.DryRun = []string{metav1.DryRunAll}
		}
		_, err = client.Patch(context.TODO(), c.node.Name, types.StrategicMergePatchType, patchBytes, patchOptions)
	} else {
		updateOptions := metav1.UpdateOptions{}
		if serverDryRun {
			updateOptions.DryRun = []string{metav1.DryRunAll}
		}
		_, err = client.Update(context.TODO(), c.node, updateOptions)
	}
	return err, patchErr
}
