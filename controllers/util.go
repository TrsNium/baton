package controllers

import (
	corev1 "k8s.io/api/core/v1"
	"strings"
)

func Contains(s []string, e string) bool {
	for _, v := range s {
		if e == v {
			return true
		}
	}
	return false
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

func IncludePods(pod corev1.Pod, pods []corev1.Pod) bool {
	for p := range pods {
		if p.ObjectMeta.Name == pod.ObjectMeta.Name {
			return true
		}
	}
	return false
}
