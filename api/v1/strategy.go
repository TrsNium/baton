/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"context"
	"errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8s "trsnium.com/baton/controllers/kubernetes"
)

type Strategy struct {
	NodeMatchLabels map[string]string `json:"nodeMatchLabels"`
	// +kubebuilder:validation:Minimum=1
	KeepPods int32 `json:"keepPods,omitempty"`
}

func (r Strategy) GetMatchNodes(c client.Client) ([]corev1.Node, error) {
	nodes, err := k8s.ListNodeMatchLabels(c, r.NodeMatchLabels)
	return nodes, err
}

func (r Strategy) GetPodsScheduledNodes(c client.Client, deployment appsv1.Deployment) ([]corev1.Pod, error) {
	nodes, err := r.GetMatchNodes(c)
	if err != nil {
		return nil, err
	}

	podLabels := deployment.Spec.Template.ObjectMeta.Labels

	var pods []corev1.Pod
	pods, err = k8s.ListPodMatchLabels(c, deployment.ObjectMeta.Namespace, podLabels)
	if err != nil {
		return nil, err
	}

	return k8s.FilterPods(pods, func(p corev1.Pod) bool {
		for _, node := range nodes {
			if node.Name == p.Spec.NodeName {
				return true
			}
		}
		return false
	}), nil
}

func (r *Strategy) IsSuplus(pods []corev1.Pod) bool {
	if r.KeepPods == 0 {
		return false
	}

	if int32(len(pods)) > r.KeepPods {
		return true
	} else {
		return false
	}
}

func (r Strategy) IsSuplusWithPodsScheduledNodes(c client.Client, deployment appsv1.Deployment) (bool, error) {
	if r.KeepPods == 0 {
		return false, nil
	}

	pods, err := r.GetPodsScheduledNodes(c, deployment)
	if err != nil {
		return false, err
	}

	if int32(len(pods)) > r.KeepPods {
		return true, nil
	} else {
		return false, nil
	}
}

func (r Strategy) IsLess(pods []corev1.Pod) bool {
	if r.KeepPods == 0 {
		return false
	}

	if int32(len(pods)) < r.KeepPods {
		return true
	} else {
		return false
	}
}

func (r Strategy) IsLessWithPodsScheduledNodes(c client.Client, deployment appsv1.Deployment) (bool, error) {
	if r.KeepPods == 0 {
		return false, nil
	}

	pods, err := r.GetPodsScheduledNodes(c, deployment)
	if err != nil {
		return false, err
	}

	if int32(len(pods)) < r.KeepPods {
		return true, nil
	} else {
		return false, nil
	}
}

func FilterStrategies(strategies []Strategy, f func(Strategy) bool) []Strategy {
	filteredStrategies := []Strategy{}
	for _, strategy := range strategies {
		if f(strategy) {
			filteredStrategies = append(filteredStrategies, strategy)
		}
	}
	return filteredStrategies
}

func GetStrategiesMatchNodes(
	c client.Client,
	strategies []Strategy,
) ([]corev1.Node, error) {
	nodes := []corev1.Node{}
	for _, strategy := range strategies {
		snodes, err := strategy.GetMatchNodes(c)
		if err != nil {
			return []corev1.Node{}, err
		}

		for _, node := range snodes {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func GetStrategiesPodsScheduledNodes(
	c client.Client,
	deployment appsv1.Deployment,
	strategies []Strategy,
) ([]corev1.Pod, error) {
	pods := []corev1.Pod{}
	for _, strategy := range strategies {
		spods, err := strategy.GetPodsScheduledNodes(c, deployment)
		if err != nil {
			return []corev1.Pod{}, err
		}

		for _, pod := range spods {
			pods = append(pods, pod)
		}
	}
	return pods, nil
}

func (r Strategy) Equals(s Strategy) bool {
	isEqualMatchLabels := reflect.DeepEqual(r.NodeMatchLabels, s.NodeMatchLabels)
	return r.KeepPods != s.KeepPods && isEqualMatchLabels
}

func GetTotalKeepPods(strategies []Strategy) int {
	total_keep_pods := 0
	for _, strategy := range strategies {
		total_keep_pods += int(strategy.KeepPods)
	}
	return total_keep_pods
}

func ValidateStrategies(c client.Client, deployment appsv1.Deployment, strategies []Strategy) error {
	pods, err := k8s.ListPodMatchLabels(
		c,
		deployment.ObjectMeta.Namespace,
		deployment.Spec.Template.ObjectMeta.Labels,
	)
	if err != nil {
		return err
	}

	var nodes []corev1.Node
	nodes, err = GetStrategiesMatchNodes(c, strategies)
	if err != nil {
		return err
	}

	runningPodsScheduledOnStrategiesNode := k8s.FilterPods(pods, func(p corev1.Pod) bool {
		for _, node := range nodes {
			if node.Name == p.Spec.NodeName && p.Status.Phase == "Running" {
				return true
			}
		}
		return false
	})

	if len(runningPodsScheduledOnStrategiesNode) != len(pods) {
		return errors.New("Deployment pods should always be on nodes of all strategies")
	}

	total_keep_pods := GetTotalKeepPods(strategies)
	if total_keep_pods > len(runningPodsScheduledOnStrategiesNode) {
		newReplicas := int32(total_keep_pods + 1)
		deployment.Spec.Replicas = &newReplicas
		if err := c.Update(context.Background(), &deployment); err != nil {
			return errors.New("failed to update replica count for Deployment")
		}
		return errors.New("Deployment replicas must be greater than the total of Keep Pods for strategy, retry from the beginning with more replicas of the pod")
	}
	return nil
}
