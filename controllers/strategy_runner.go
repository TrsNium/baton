package controllers

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
	batonv1 "trsnium.com/baton/api/v1"
	k8s "trsnium.com/baton/controllers/kubernetes"
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
				return
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
	return isUpdatedDeploymentInfo ||
		isUpdatedStrategies ||
		r.baton.Spec.IntervalSec != baton.Spec.IntervalSec
}

func (r *BatonStrategiesyRunner) runStrategies() error {
	deploymentInfo := r.baton.Spec.Deployment
	namespace := deploymentInfo.NameSpace
	deploymentName := deploymentInfo.Name
	deployment, err := k8s.GetDeployment(r.client, namespace, deploymentName)
	if err != nil {
		r.logger.Error(err, fmt.Sprintf("failed to get Deployment{Namespace: %s, Name: %s}", namespace, deploymentName))
		return err
	}

	err = batonv1.ValidateStrategies(r.client, deployment, r.baton.Spec.Strategies)
	if err != nil {
		return err
	}

	err = r.scaleOutPodIfSatisfyStrategyKeepPods(r.baton.Spec.Strategies, deployment)
	if err != nil {
		r.logger.Error(err, "failed to scale out deployment for not satisfied strategy's keep pods")
		return err
	}

	err = r.migrateSuplusPodToOther(r.baton.Spec.Strategies, deployment)
	if err != nil {
		r.logger.Error(err, "failed to migrate suplus Pod to other Node")
	}

	err = r.migrateLessPodFromOther(r.baton.Spec.Strategies, deployment)
	if err != nil {
		r.logger.Error(err, "failed to migrate less Pod from other Node")
	}
	return nil
}

func (r *BatonStrategiesyRunner) scaleOutPodIfSatisfyStrategyKeepPods(
	strategies []batonv1.Strategy,
	deployment appsv1.Deployment,
) error {
	for _, strategy := range strategies {
		if !(strategy.KeepPods > 0) {
			continue
		}

		pods, err := k8s.ListPodMatchLabels(
			r.client,
			deployment.ObjectMeta.Namespace,
			deployment.Spec.Template.ObjectMeta.Labels,
		)
		if err != nil {
			return err
		}

		strategyNodes, err := strategy.GetMatchNodes(r.client)
		if err != nil {
			return errors.New("Failed to fetch nodes matching strategy labels")
		}

		runninPodsScheduledOnStrategyNodes := k8s.FilterPods(pods, func(p corev1.Pod) bool {
			for _, node := range strategyNodes {
				if node.Name == p.Spec.NodeName && p.Status.Phase == "Running" {
					return true
				}
			}
			return false
		})

		if len(runninPodsScheduledOnStrategyNodes) >= int(strategy.KeepPods) {
			continue
		}

		r.logger.Info(fmt.Sprintf("Not satisfy strategy. scale out for (%v)", strategy.NodeMatchLabels))
		otherStrategies := batonv1.FilterStrategies(strategies, func(s batonv1.Strategy) bool {
			if !reflect.DeepEqual(strategy, s) {
				return true
			}
			return false
		})

		cordonedNodes, err := batonv1.GetStrategiesMatchNodes(r.client, otherStrategies)
		for i, _ := range cordonedNodes {
			err := k8s.RunCordonOrUncordon(r.client, &cordonedNodes[i], true)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to cordon Node{Name: %s}", cordonedNodes[i].Name))
				continue
			}
		}

		observedPods, err := k8s.ListPodMatchLabels(
			r.client,
			deployment.ObjectMeta.Namespace,
			deployment.Spec.Template.ObjectMeta.Labels,
		)
		if err != nil {
			r.logger.Error(err, fmt.Sprintf("failed to list Pods{Namespace: %s, MatchLabels: %v}",
				deployment.ObjectMeta.Namespace,
				deployment.Spec.Template.ObjectMeta.Labels,
			))
		}

		// Update to one larger than the current replicas.
		newReplicas := *deployment.Spec.Replicas + 1
		deployment.Spec.Replicas = &newReplicas
		if err := r.client.Update(context.Background(), &deployment); err != nil {
			r.logger.Error(err, "failed to Deployment update replica count")
			return errors.New("failed to Deployment update replica count")
		}

		err = r.monitorNewPodsUntilReady(deployment, nil, observedPods)
		if err != nil {
			r.logger.Error(err, fmt.Sprintf("failed to monitor new pod"))
			continue
		}

		for _, cordonedNode := range cordonedNodes {
			var uncordonedNode corev1.Node
			uncordonedNode, err = k8s.GetNode(r.client, cordonedNode.Name)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to get node {Name: %s}", cordonedNode.Name))
				continue
			}

			err = k8s.RunCordonOrUncordon(r.client, &uncordonedNode, false)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to uncordon Node{Name: %s}", cordonedNode.ObjectMeta.Name))
				continue
			}
		}
		// Return an error so that we can start the process all over again.
		return errors.New("Updated deployment's replicas to satisfy strategy")
	}
	return nil
}

func (r *BatonStrategiesyRunner) migrateSuplusPodToOther(
	strategies []batonv1.Strategy,
	deployment appsv1.Deployment,
) error {
	for _, strategy := range strategies {
		pods, err := strategy.GetPodsScheduledNodes(r.client, deployment)
		if err != nil {
			r.logger.Error(err, "failed to list Pods")
			continue
		}

		if !strategy.IsSuplus(pods) {
			continue
		}

		r.logger.Info(fmt.Sprintf("migrate suplus group (%v) to other", strategy.NodeMatchLabels))
		cordonedNodes, err := strategy.GetMatchNodes(r.client)
		if err != nil {
			r.logger.Error(err, "failed to list Nodes")
			continue
		}

		for i, _ := range cordonedNodes {
			err := k8s.RunCordonOrUncordon(r.client, &cordonedNodes[i], true)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to cordon Node{Name: %s}", cordonedNodes[i].Name))
				continue
			}
		}

		for _, deletedPod := range pods[strategy.KeepPods:] {
			observedPods, err := k8s.ListPodMatchLabels(
				r.client,
				deployment.ObjectMeta.Namespace,
				deployment.Spec.Template.ObjectMeta.Labels,
			)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to list Pods{Namespace: %s, MatchLabels: %v}",
					deployment.ObjectMeta.Namespace,
					deployment.Spec.Template.ObjectMeta.Labels,
				))
			}

			hash := deletedPod.ObjectMeta.GetLabels()["pod-template-hash"]
			err = k8s.DeletePod(r.client, deletedPod)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to delete Pod{Name: %s}", deletedPod.ObjectMeta.Name))
				continue
			}

			err = r.monitorNewPodsUntilReady(deployment, &hash, observedPods)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to monitor new pod"))
				continue
			}
		}

		for _, cordonedNode := range cordonedNodes {
			var uncordonedNode corev1.Node
			uncordonedNode, err = k8s.GetNode(r.client, cordonedNode.Name)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to get node {Name: %s}", cordonedNode.Name))
				continue
			}

			err = k8s.RunCordonOrUncordon(r.client, &uncordonedNode, false)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to uncordon Node{Name: %s}", cordonedNode.ObjectMeta.Name))
				continue
			}
		}
	}
	return nil
}

func (r *BatonStrategiesyRunner) migrateLessPodFromOther(
	strategies []batonv1.Strategy,
	deployment appsv1.Deployment,
) error {
	for _, strategy := range strategies {
		pods, err := strategy.GetPodsScheduledNodes(r.client, deployment)
		if err != nil {
			r.logger.Error(err, "failed to list Pods")
			continue
		}

		if !strategy.IsLess(pods) {
			continue
		}
		r.logger.Info(fmt.Sprintf("migrate less group (%v) from other", strategy.NodeMatchLabels))

		suplusStrategies := batonv1.FilterStrategies(strategies, func(s batonv1.Strategy) bool {
			if s.KeepPods == 0 {
				return true
			}
			isSuplus, _ := s.IsSuplusWithPodsScheduledNodes(r.client, deployment)
			return isSuplus
		})

		deleatablePod, err := batonv1.GetStrategiesPodsScheduledNodes(r.client, deployment, suplusStrategies)
		if err != nil {
			r.logger.Error(err, "failed to list Pods")
			continue
		}

		cordonedNodes, err := batonv1.GetStrategiesMatchNodes(r.client, suplusStrategies)
		if err != nil {
			r.logger.Error(err, "failed to list Nodes")
			continue
		}

		for i, _ := range cordonedNodes {
			err := k8s.RunCordonOrUncordon(r.client, &cordonedNodes[i], true)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to cordon Node{Name: %s}", cordonedNodes[i].ObjectMeta.Name))
				continue
			}
		}

		for _, deletedPod := range deleatablePod[:strategy.KeepPods] {
			observedPods, err := k8s.ListPodMatchLabels(
				r.client,
				deployment.ObjectMeta.Namespace,
				deployment.Spec.Template.ObjectMeta.Labels,
			)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to list Pods{Namespace: %s, MatchLabels: %v}",
					deployment.ObjectMeta.Namespace,
					deployment.Spec.Template.ObjectMeta.Labels,
				))
			}

			hash := deletedPod.ObjectMeta.GetLabels()["pod-template-hash"]
			err = k8s.DeletePod(r.client, deletedPod)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to delete Pod{Name: %s}", deletedPod.ObjectMeta.Name))
				continue
			}

			err = r.monitorNewPodsUntilReady(deployment, &hash, observedPods)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to monitor new pod"))
				continue
			}
		}

		for _, cordonedNode := range cordonedNodes {
			var uncordonedNode corev1.Node
			uncordonedNode, err = k8s.GetNode(r.client, cordonedNode.Name)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to get node {Name: %s}", cordonedNode.Name))
				continue
			}

			err = k8s.RunCordonOrUncordon(r.client, &uncordonedNode, false)
			if err != nil {
				r.logger.Error(err, fmt.Sprintf("failed to uncordon Node{Name: %s}", cordonedNode.ObjectMeta.Name))
				continue
			}
		}
	}
	return nil
}

func (r *BatonStrategiesyRunner) monitorNewPodsUntilReady(
	deployment appsv1.Deployment,
	podTemplateHash *string,
	observedPods []corev1.Pod,
) error {
	namespace := deployment.ObjectMeta.Namespace
	labels := deployment.Spec.Template.ObjectMeta.Labels
	timeout := time.After(time.Duration(r.baton.Spec.MonitorTimeoutSec) * time.Second)
	tick := time.Tick(15 * time.Second)
Monitor:
	for {
		select {
		case <-timeout:
			return errors.New("time out to monitor new pod")
		case <-tick:
			currentPods, err := k8s.ListPodMatchLabels(r.client, namespace, labels)
			if err != nil {
				r.logger.Error(err,
					fmt.Sprintf("failed to list Pods{Namespace: %s, MatchLabels: %v}",
						namespace, labels),
				)
				return err
			}

			filterdCurrentPods := k8s.FilterPods(currentPods, func(p corev1.Pod) bool {
				if (podTemplateHash == nil) || (*podTemplateHash == p.ObjectMeta.GetLabels()["pod-template-hash"]) {
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
					return errors.New("failed to launch pod")
				}
			}
			return nil
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
