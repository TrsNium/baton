package controllers

import (
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"time"
	batonv1 "trsnium.com/baton/api/v1"
)

type BatonStrategiesyRunner struct {
	client   client.Client
	baton    *batonv1.Baton
	stopFlag chan bool
	logger   logr.Logger
}

func NewBatonStrategiesyRunner(client client.Client, baton batonv1.Baton, logger logr.Logger, runnerName string) BatonStrategiesyRunner {
	return BatonStrategiesyRunner{
		client: client,
		baton:  &baton,
		logger: logger.WithName("BatonStrategiesRunnerManager").WithName(runnerName),
	}
}

func (r *BatonStrategiesyRunner) Run() {
	r.logger.Info("Run runner")
	r.stopFlag = make(chan bool)
	go func() {
		for {
			err := r.runStrategies()
			if err != nil {
				r.logger.Error(err, "failed to run strategy")
			}
			select {
			case <-time.After(time.Duration(r.baton.Spec.IntervalSec) * time.Second):
			case <-r.stopFlag:
			}
		}
	}()
}

func (r *BatonStrategiesyRunner) Stop() {
	r.stopFlag <- true
	r.logger.Info("Stop runner")
}

func (r *BatonStrategiesyRunner) IsUpdatedBatonStrategies(baton batonv1.Baton) bool {
	isUpdatedDeploymentInfo := r.baton.Spec.Deployment == baton.Spec.Deployment
	isUpdatedStrategies := true
	gs := make(map[string]batonv1.Strategy)
	for _, s := range r.baton.Spec.Strategies {
		gs[s.NodeGroup] = s
	}

	sgs := make(map[string]batonv1.Strategy)
	for _, s := range baton.Spec.Strategies {
		sgs[s.NodeGroup] = s
	}

	for k, v := range gs {
		if v != sgs[k] {
			isUpdatedStrategies = false
			break
		}
	}

	for k, v := range sgs {
		if v != gs[k] {
			isUpdatedStrategies = false
			break
		}
	}
	return isUpdatedDeploymentInfo ||
		isUpdatedStrategies ||
		r.baton.Spec.IntervalSec != baton.Spec.IntervalSec
}

func (r *BatonStrategiesyRunner) runStrategies() error {
	deploymentInfo := r.baton.Spec.Deployment
	namespace := deploymentInfo.NameSpace
	deploymentName := deploymentInfo.Name
	deployment, err := GetDeployment(r.client, namespace, deploymentName)
	if err != nil {
		r.logger.Error(err, fmt.Sprintf("failed to get Deployment{Namespace: %s, Name: %s}", namespace, deploymentName))
		return err
	}
	podLabels := deployment.Spec.Template.ObjectMeta.Labels

	err = r.validateStrategies(deployment)
	if err != nil {
		r.logger.Error(err, "Deployment replicas must be greater than the total of Keep Pods for strategy")
		return err
	}

	var pods []corev1.Pod
	pods, err = ListPodMatchLabels(r.client, namespace, podLabels)
	if err != nil {
		r.logger.Error(err,
			fmt.Sprintf("failed to list Pod{Namespace: %s, MatchingLabels: %v}",
				namespace, podLabels),
		)
	}

	groupStrategies := make(map[string]batonv1.Strategy)
	groupPods := make(GroupPods)
	for _, strategy := range r.baton.Spec.Strategies {
		groupStrategies[strategy.NodeGroup] = strategy
		groupPods.NewGroup(strategy.NodeGroup)
	}

	// NOTE pod which Phase is 'Pending' or scheduled to in other strategy nodeGroup is grouped up to '`other'
	for _, pod := range pods {
		addPodToGroupPods(pod, &groupPods)
	}

	// Migrate surplus and shortage pods from other nodes according to strategy
	err = r.migrateSuplusPodToOther(namespace, podLabels, groupStrategies, &groupPods)
	if err != nil {
		r.logger.Error(err, "failed to migrate suplus Pod to other Node")
	}

	err = r.migrateLessPodFromOther(namespace, podLabels, groupStrategies, &groupPods)
	if err != nil {
		r.logger.Error(err, "failed to migrate less Pod from other Node")
	}
	return nil
}

func (r *BatonStrategiesyRunner) validateStrategies(deployment appsv1.Deployment) error {
	replicas := *deployment.Spec.Replicas
	total_keep_pods := int32(0)
	for _, strategy := range r.baton.Spec.Strategies {
		total_keep_pods += strategy.KeepPods
	}

	if total_keep_pods > replicas {
		return errors.New("failed to validate strategy arguments")
	}
	return nil
}

func (r *BatonStrategiesyRunner) migrateSuplusPodToOther(
	namespace string,
	labels map[string]string,
	groupStrategies map[string]batonv1.Strategy,
	groupPods *GroupPods,
) error {
	for _, suplusGroup := range groupPods.SuplusGroups(groupStrategies, false) {
		r.logger.Info(fmt.Sprintf("migrate suplus group (%s) to other", suplusGroup))
		cordonedNodes, err := groupPods.GroupNodes(r.client, suplusGroup, groupStrategies)
		if err != nil {
			r.logger.Error(err, "failed to list Nodes")
		}

		for i, _ := range cordonedNodes {
			err := RunCordonOrUncordon(r.client, &cordonedNodes[i], true)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to cordon Node{Name: %s}", cordonedNodes[i].ObjectMeta.Name))
				continue
			}
		}

		keepPods := groupStrategies[suplusGroup].KeepPods
		for _, deletedPod := range (*groupPods)[suplusGroup][keepPods:] {
			hash := deletedPod.ObjectMeta.GetLabels()["pod-template-hash"]
			err := DeletePod(r.client, deletedPod)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to delete Pod{Name: %s}", deletedPod.ObjectMeta.Name))
				continue
			}

			newPods, err := r.monitorNewPodsUntilReady(namespace, labels, hash, groupPods.UnGroupPods())
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to monitor new pod"))
				continue
			}

			for _, pod := range newPods {
				addPodToGroupPods(pod, groupPods)
			}
			groupPods.DeletePod(suplusGroup, deletedPod)
		}

		for _, cordonedNode := range cordonedNodes {
			var uncordonedNode corev1.Node
			uncordonedNode, err = GetNode(r.client, cordonedNode.Name)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to get node {Name: %s}", cordonedNode.Name))
				continue
			}

			err = RunCordonOrUncordon(r.client, &uncordonedNode, false)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to uncordon Node{Name: %s}", &cordonedNode.ObjectMeta.Name))
				continue
			}
		}
	}
	return nil
}

func (r *BatonStrategiesyRunner) migrateLessPodFromOther(
	namespace string,
	labels map[string]string,
	groupStrategies map[string]batonv1.Strategy,
	groupPods *GroupPods,
) error {
	for _, lessGroup := range groupPods.LessGroups(groupStrategies, false) {
		r.logger.Info(fmt.Sprintf("migrate less group (%s) from other", lessGroup))
		suplusGroups := groupPods.SuplusGroups(groupStrategies, true)
		cordonedNodes, err := groupPods.GroupsNodes(r.client, suplusGroups, groupStrategies)
		if err != nil {
			r.logger.Error(err, "failed to list Nodes")
		}

		for i, _ := range cordonedNodes {
			err := RunCordonOrUncordon(r.client, &cordonedNodes[i], true)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to cordon Node{Name: %s}", &cordonedNodes[i].ObjectMeta.Name))
				continue
			}
		}

		keepPods := groupStrategies[lessGroup].KeepPods
		for _, deletedPod := range groupPods.GroupsPods(suplusGroups)[:keepPods] {
			hash := deletedPod.ObjectMeta.GetLabels()["pod-template-hash"]
			err := DeletePod(r.client, deletedPod)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to delete Pod{Name: %s}", deletedPod.ObjectMeta.Name))
				continue
			}

			newPods, err := r.monitorNewPodsUntilReady(namespace, labels, hash, groupPods.UnGroupPods())
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to monitor new pod"))
				continue
			}

			for _, pod := range newPods {
				addPodToGroupPods(pod, groupPods)
			}
			groupPods.DeletePod(lessGroup, deletedPod)
		}

		for _, cordonedNode := range cordonedNodes {
			var uncordonedNode corev1.Node
			uncordonedNode, err = GetNode(r.client, cordonedNode.Name)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to get node {Name: %s}", cordonedNode.Name))
				continue
			}

			err = RunCordonOrUncordon(r.client, &uncordonedNode, false)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to uncordon Node{Name: %s}", &cordonedNode.ObjectMeta.Name))
				continue
			}
		}
	}
	return nil
}

func (r *BatonStrategiesyRunner) monitorNewPodsUntilReady(
	namespace string,
	labels map[string]string,
	podTemplateHash string,
	observedPods []corev1.Pod,
) ([]corev1.Pod, error) {
	timeout := time.After(time.Duration(r.baton.Spec.MonitorTimeoutSec) * time.Second)
	tick := time.Tick(15 * time.Second)
Monitor:
	for {
		select {
		case <-timeout:
			return nil, errors.New("time out to monitor new pod")
		case <-tick:
			currentPods, err := ListPodMatchLabels(r.client, namespace, labels)
			if err != nil {
				r.logger.Error(err,
					fmt.Sprintf("failed to list Pod{Namespace: %s, MatchingLabels: %v}",
						namespace, labels),
				)
				return nil, err
			}

			filterdCurrentPods := FilterPods(currentPods, func(p corev1.Pod) bool {
				if podTemplateHash == p.ObjectMeta.GetLabels()["pod-template-hash"] {
					return true
				}
				return false
			})

			newPods := getNewPods(observedPods, filterdCurrentPods)
			if len(newPods) == 0 {
				continue Monitor
			}

			for _, pod := range newPods {
				phase := pod.Status.Phase
				nodeName := pod.Spec.NodeName
				if nodeName == "" || phase == "Unknow" {
					continue Monitor
				} else if phase == "Failed" {
					return nil, errors.New("failed to launch pod")
				}
			}
			return newPods, nil
		}
	}
}

func getNewPods(observedPods []corev1.Pod, currentPods []corev1.Pod) []corev1.Pod {
	includePods := func(pod corev1.Pod, pods []corev1.Pod) bool {
		for _, p := range pods {
			if p.Name == pod.Name {
				return true
			}
		}
		return false
	}

	notExistPods := []corev1.Pod{}
	for _, cp := range currentPods {
		if !includePods(cp, observedPods) {
			notExistPods = append(notExistPods, cp)
		}
	}
	return notExistPods
}

func addPodToGroupPods(pod corev1.Pod, groupPods *GroupPods) {
	getSubStrContainsInSubStrs := func(s string, subStrs []string) (bool, string) {
		for _, subStr := range subStrs {
			if strings.Contains(s, subStr) {
				return true, subStr
			}
		}
		return false, ""
	}

	scheduledNodeName := pod.Spec.NodeName
	isContain, nodeGroup := getSubStrContainsInSubStrs(scheduledNodeName, groupPods.Group())
	if isContain {
		groupPods.AddPod(nodeGroup, pod)
	} else {
		groupPods.AddPod("`other", pod)
	}
}
