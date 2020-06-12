package controllers

import (
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"time"
	batonv1 "trsnium.com/baton/api/v1"
)

type BatonStrategiesyRunner struct {
	client   client.Client
	baton    batonv1.Baton
	stopFlag bool
	logger   logr.Logger
}

func NewBatonStrategiesyRunner(client client.Client, baton batonv1.Baton, logger logr.Logger) BatonStrategiesyRunner {
	return BatonStrategiesyRunner{
		client: client,
		baton:  baton,
		logger: logger,
	}
}

func (r *BatonStrategiesyRunner) Run() error {
	r.stopFlag = make(chan bool)
	go func() {
		for {
			r.runStrategies()
			select {
			case <-time.After(r.baton.Spec.IntervalSec * time.Second):
			case <-r.stopFlag:
				return nil
			}
		}
	}()
}

func (r *BatonStrategiesyRunner) Stop() {
	r.stopFlag <- true
}

func (r *BatonStrategiesyRunner) IsUpdatedBatonStrategies(baton batonv1.Baton) bool {
	return true
}

func (r *BatonStrategiesyRunner) runStrategies() error {
	ctx := context.Background()

	deploymentInfo := r.baton.Spec.Deployment
	deployment, err := GetDeployment(r.client, deploymentInfo.Namespace, deploymentInfo.Name)
	if err != nil {
		r.logger.Error(err, fmt.Sprintf("failed to get Deployment{Namespace: %s, Name: %s}", deploymentInfo.Namespace, deploymentInfo.Name))
		return err
	}

	err := r.validateStrategies(deployment)
	if err != nil {
		r.logger.Error(err, "Deployment replicas must be greater than the total of Keep Pods for strategy")
		return err
	}

	pods, err := ListPodMatchLabels(r.client, deploymentInfo.Namespace, Deployment.Spec.Template.ObjectMeta.Labels)
	if err != nil {
		r.logger.Error(err,
			fmt.Sprintf("failed to list Pod{Namespace: %s, MatchingLabels: %v}",
				deployment.ObjectMeta.Namespace, deployment.Spec.Template.ObjectMeta.Labels),
		)
	}

	// Limit to only pods associated with deployment uid
	deploymentUID := deployment.ObjectMeta.UID
	pods := FilterPods(pods, func(p corev1.Pod) bool {
		for _, or := range p.ObjectMeta.GetOwnerReferences() {
			if or.UID == deploymentUID {
				return true
			}
		}
		return false
	})

	groupStrategies := make(map[string]batonv1.Strategy)
	groupPods := make(GroupPods)
	for _, strategy := range r.baton.Spec.Strategies {
		groudStrategies[strategy.NodeGroup] = strategy
		groupPods.NewGroup(strategy.NodeGroup)
	}

	// NOTE pod which Phase is 'Pending' or scheduled to in other strategy nodeGroup is grouped up to '`other'
	for _, pod := range pods {
		addPodToGroupPods(pod, &groupPods)
	}

	// Migrate surplus and shortage pods from other nodes according to strategy
	err := r.deleteSuplusPod(namespace, labels, deploymentUID, groupdStrategies, groupPods)
}

func (r *BatonStrategiesyRunner) validateStrategies(deployment appv1.Deployment) error {
	replicas := deployment.Spec.Replicas
	total_keep_pods = 0
	for _, strategy := range r.baton.Spec.Strategies {
		total_keep_pods += strategy.KeepPods
	}

	if replicas > total_keep_pods {
		return errors.New("failed to validate strategy arguments")
	}
	return nil
}

func (r *BatonStrategiesyRunner) deleteSuplusPod(
	namespace string,
	labels map[string]string,
	deploymentUID types.UID,
	groupedStrategies map[string]batonv1.Strategy,
	groupPods *map[string][]corev1.Pod,
) error {
	ctx := context.Background()
	for _, group := range groupPods.GetGroup() {
		if group == "`other" {
			continue
		}

		keepPods := groupedStrategies[group].KeepPods
		if !len(groupPods[group]) > KeepPods {
			continue
		}

		nodes := GetNodes(r.client)
		cordonedNodes := FilterNode(nodes, func(n corev1.Node) bool {
			return strings.Contains(n.Spec.ObjectMeta.Name, groupedStrategies[group])
		})

		for _, node := range cordonedNodes {
			err := RunCordonOrUncordon(r.client, &node, true)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to cordon Node{Name: %s}", node.ObjectMeta.Name))
				continue
			}
		}

		for _, deletedPod := range groupPods[group][keepPods:] {
			hash := deletedPod.ObjectMeta.GetLabels()["pod-template-hash"]
			err := r.client.Delete(ctx, deletedPod)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to delete Pod{Name: %s}", deletedPod.ObjectMeta.Name))
				continue
			}

			newPods, err := monitorNewPodUntilReady(namespace, labels, deploymentUID, hash, groupPods.UnGroupPods())
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to monitor new pod"))
				continue
			}

			for _, pod := range newPods {
				addPodToGroupPods(pod, groupPods)
			}
			groupPods.DeletePod(group, deletedPod)
		}

		for _, node := range cordonedNodes {
			err := RunCordonOrUncordon(r.client, &node, false)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to uncordon Node{Name: %s}", node.ObjectMeta.Name))
				continue
			}
		}
	}
}

func (r *BatonStrategiesyRunner) monitorNewPodsUntilReady(
	namespace string,
	labels map[string]string,
	deploymentUID types.UID,
	podTemplateHash string,
	observedPods []corev1.Pod,
) {
	// TODO allow timeout to be defined in crd
	timeout := time.After(5 * time.Second)
	tick := time.Tick(1 * time.Second)
Monitor:
	for {
		select {
		case <-timeout:
			return nil, errors.New("time out to monitor new pod")
		case <-tick:
			currentPods := ListPodMatchLabels(r.client, namespace, labels)
			if err != nil {
				r.logger.Error(err,
					fmt.Sprintf("failed to list Pod{Namespace: %s, MatchingLabels: %v}",
						namespace, labels),
				)
				return nil, err
			}

			filterdCurrentPods := FilterPods(currentPods, func(p corev1.Pod) bool {
				for _, or := range p.ObjectMeta.GetOwnerReferences() {
					if or.UID == deploymentUID && podTemplateHash == pod.ObjectMeta.GetLabels()["pod-template-hash"] {
						return true
					}
				}
				return false
			})

			newPods, err := getNotExistPods(observedPods, filterdCurrentPods)
			if len(newPods) == 0 {
				continue Monitor
			}

			for _, pod := range newPods {
				phase := pod.PodStatus.PodPhase
				nodeName := pod.Spec.NodeName
				if nodeName == nil {
					continue Monitor
				} else if phase == "Failed" {
					return nil, errors.New("failed to launch pod")
				}
			}
			return newPods, nil
		}
	}
	return nil, errors.New("could not found new pods")
}

func getNotExistPods(observedPods []corev1.Pod, currentPods []corev1.Pod) []corev1.Pod {
	includePods := func(pod corev1.Pod, pods []corev1.Pod) bool {
		for p := range pods {
			if p.ObjectMeta.Name == pod.ObjectMeta.Name {
				return true
			}
		}
		return false
	}

	notExistPods = []corev1.Pod{}
	for _, cp := range currentPods {
		if !includePods(cp, observedPods) {
			notExistPods := append(notExistPods, cp)
		}
	}
	return notExistPods
}

func addPodToGroupPods(pod corev1.Pod, groupPods *GroupPods) {
	getSubStrContainsInSubStrs := func(s string, subStrs []string) (bool, string) {
		if s == nil {
			return false, nil
		}

		for _, subStr := range subStrs {
			if strings.Contains(s, subStr) {
				return true, subStr
			}
		}
		return false, nil
	}

	scheduledNodeName := pod.Spec.NodeName
	isContain, nodeGroup := getSubStrContainsInSubStrs(scheduledNodeName, groupPods.GetGroup())
	if isContain {
		groupPods.AddPod(nodeGroup, pod)
	} else {
		groupPods.AddPod("`other", pod)
	}
}
