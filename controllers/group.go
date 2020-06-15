package controllers

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	batonv1 "trsnium.com/baton/api/v1"
)

type GroupPods map[string][]corev1.Pod

func (r *GroupPods) NewGroup(group string) {
	if _, ok := (*r)[group]; !ok {
		(*r)[group] = []corev1.Pod{}
	}
}

func (r *GroupPods) AddPod(group string, pod corev1.Pod) {
	if _, ok := (*r)[group]; ok {
		(*r)[group] = append((*r)[group], pod)
	} else {
		(*r)[group] = []corev1.Pod{}
		(*r)[group] = append((*r)[group], pod)
	}
}

func (r *GroupPods) DeletePod(group string, pod corev1.Pod) {
	pods := []corev1.Pod{}
	for _, p := range (*r)[group] {
		if p.ObjectMeta.Name != pod.ObjectMeta.Name {
			pods = append(pods, p)
		}
	}
	(*r)[group] = pods
}

func (r GroupPods) Group() []string {
	groups := []string{}
	for g := range r {
		groups = append(groups, g)
	}
	return groups
}

func (r GroupPods) SuplusGroups(groupStrategies map[string]batonv1.Strategy, isIncludeOtherGroup bool) []string {
	suplusGroups := []string{}
	for key, _ := range r {
		if int32(len(r[key])) > groupStrategies[key].KeepPods || (key == "`other" && isIncludeOtherGroup) {
			suplusGroups = append(suplusGroups, key)
		}
	}
	return suplusGroups
}

func (r GroupPods) LessGroups(groupStrategies map[string]batonv1.Strategy, isIncludeOtherGroup bool) []string {
	lessGroups := []string{}
	for key, _ := range r {
		if int32(len(r[key])) < groupStrategies[key].KeepPods || (key == "`other" && isIncludeOtherGroup) {
			lessGroups = append(lessGroups, key)
		}
	}
	return lessGroups
}

func (r GroupPods) GroupNodes(c client.Client, group string, groupStrategies map[string]batonv1.Strategy) ([]corev1.Node, error) {
	nodes, err := GetNodes(c)
	if err != nil {
		return nil, nil
	}

	if group == "`other" {
		otherGroupPods := r["`other"]
		groupNodes := FilterNodes(nodes, func(n corev1.Node) bool {
			for _, otherGroupPod := range otherGroupPods {
				if n.Name == otherGroupPod.Spec.NodeName {
					return true
				}
			}
			return false
		})
		return groupNodes, nil
	} else {
		groupNodes := FilterNodes(nodes, func(n corev1.Node) bool {
			return strings.Contains(n.Name, groupStrategies[group].NodeGroup)
		})
		return groupNodes, nil
	}
}

func (r GroupPods) GroupsNodes(c client.Client, groups []string, groupStrategies map[string]batonv1.Strategy) ([]corev1.Node, error) {
	groupsNodes := []corev1.Node{}
	for _, group := range groups {
		nodes, err := r.GroupNodes(c, group, groupStrategies)
		if err != nil {
			return nil, err
		}

		for _, node := range nodes {
			groupsNodes = append(groupsNodes, node)
		}
	}
	return groupsNodes, nil
}

func (r GroupPods) GroupsPods(groups []string) []corev1.Pod {
	groupsPods := []corev1.Pod{}
	for _, group := range groups {
		pods := r[group]

		for _, pod := range pods {
			groupsPods = append(groupsPods, pod)
		}
	}
	return groupsPods
}

func (r GroupPods) UnGroupPods() []corev1.Pod {
	uPods := []corev1.Pod{}
	for _, pods := range r {
		for _, pod := range pods {
			uPods = append(uPods, pod)
		}
	}
	return uPods
}
