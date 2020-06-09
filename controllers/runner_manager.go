package controllers

import (
	"fmt"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	batonv1 "trsnium.com/baton/api/v1"
)

type BatonStrategiesRunnerManager struct {
	client                   client.Client
	batonStrategiesRunnerMap map[string]BatonStrategiesyRunner
}

func (r *BatonManager) IsManaged(baton batonv1.Baton) bool {
	metadata := baton.ObjectMeta
	key := fmt.Sprintf("%s-%s", metadata.Namespace, metadata.Name)
	_, isManaged := r.batonStrategiesRunnerMap[key]
	return isManaged
}

func (r *BatonManager) IsUpdated(baton batonv1.Baton) bool {
	metadata := baton.ObjectMeta
	key := fmt.Sprintf("%s-%s", metadata.Namespace, metadata.Name)
	batonRunner, _ := r.batonStrategiesRunnerMap[key]
	return batonRuleRunner.IsUpdated(baton)
}

func (r *BatonManager) Add(baton batonv1.Baton) {
	metadata := baton.ObjectMeta
	key := fmt.Sprintf("%s-%s", metadata.Namespace, metadata.Name)
	batonStrategiesRunner := NewBatonStrategiesyRunner(baton)
	batonStrategiesRunner.Run()
	r.batonStrategiesRunnerMap[key] = batonStrategiesRunner
}

func (r *BatonManager) Delete(baton batonv1.Baton) {
	metadata := baton.ObjectMeta
	key := fmt.Sprintf("%s-%s", metadata.Namespace, metadata.Name)
	batonRunner, _ := r.batonStrategiesRunnerMap[key]
	batonRunner.Stop()
}

func (r *BatonManager) DeleteNotExists(batons batonv1.BatonList) {
	expectedKeys := []string{}
	batonMap := make(map[string]batonv1.Baton)
	for _, baton := range batons.Items {
		metadata := baton.ObjectMeta
		key := fmt.Sprintf("%s-%s", metadata.Namespace, metadata.Name)
		expectedKeys = append(expectedKeys, key)
		batonMap[key] = baton
	}

	batonStrategiesRunneKeys = r.getBatonStrategiesRunnerKeys()
	for _, batonStrategiesRunneKey := range batonStrategiesRunneKeys {
		if !contains(expectedKeys, batonStrategiesRunneKey) {
			r.Delete(batonMap[batonStrategiesRunneKey])
		}
	}
}

func (r *BatonManager) getBatonStrategiesRunnerKeys() []string {
	ks := []string{}
	for k, _ := range r.batonStrategiesRunnerMap {
		ks = append(ks, k)
	}
	return ks
}

func contains(s []string, e string) bool {
	for _, v := range s {
		if e == v {
			return true
		}
	}
	return false
}

func NewBatonStrategiesyRunnerManager(client client.Client) *BatonStrategiesRunnerManager {
	return &BatonStrategiesRunnerManager{
		client:                   client,
		batonStrategiesRunnerMap: make(map[string]BatonStrategiesyRunner),
	}
}
