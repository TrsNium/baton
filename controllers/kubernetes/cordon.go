// hommage from https://github.com/kubernetes/kubectl/blob/master/pkg/drain/cordon.go

package kubernetes

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
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
func (r *CordonHelper) UpdateIfRequired(desired bool) bool {
	r.desired = desired

	return r.node.Spec.Unschedulable != r.desired
}

// PatchOrReplace uses given clientset to update the node status, either by patching or
// updating the given node object; it may return error if the object cannot be encoded as
// JSON, or if either patch or update calls fail; it will also return a second error
// whenever creating a patch has failed
func (r *CordonHelper) PatchOrReplace(c client.Client) (error, error) {
	oldData, err := json.Marshal(r.node)
	if err != nil {
		return err, nil
	}

	r.node.Spec.Unschedulable = r.desired

	newData, err := json.Marshal(r.node)
	if err != nil {
		return err, nil
	}

	patchBytes, patchErr := strategicpatch.CreateTwoWayMergePatch(oldData, newData, r.node)
	if patchErr != nil {
		err = c.Patch(context.TODO(), &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: r.node.Namespace,
				Name:      r.node.Name,
			},
		}, client.RawPatch(types.StrategicMergePatchType, patchBytes))
	} else {
		err = c.Update(context.TODO(), r.node)
	}
	return err, patchErr
}
