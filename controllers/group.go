package controllers

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

type GroupPods map[string][]corev1.Pod

func (r *GroupPods) AddPod(group string, pod corev1.Pod) {
	if _, ok := r[group]; ok {
		r[group] = append(r[group], pod)
	} else {
		r[group] = []corev1.Pod{}
		r[group] = append(r[group], pod)
	}
}

func (r *GroupPods) DeletePod(group string, pod corev1.Pod) {
	pods := make([]corev1.Pod)
	for _, p := range r[group] {
		if p.ObjectMeta.Name != pod.ObjectMeta.Name {
			pods = append(pods, p)
		}
	}
	r[group] = pods
}

func (r GroupPods) CordonOrUnCordonGroupNodes(group string, c client.Client) {

}

func (r *GroupPods) GetGroup() []string {
	groups := []string{}
	for g := range r {
		groups := append(groups, g)
	}
	return groups
}

func (r *GroupPods) UnGroupPods() []corev1.Pod {
	uPods := []corev1.Pod{}
	for _, pods := range r {
		uPods := append(uPods, pods)
	}
	return uPods
}
