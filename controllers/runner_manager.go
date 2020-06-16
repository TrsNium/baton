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
	logger                   logr.Logger
}

func NewBatonStrategiesyRunnerManager(client client.Client, logger logr.Logger) *BatonStrategiesRunnerManager {
	return &BatonStrategiesRunnerManager{
		client:                   client,
		batonStrategiesRunnerMap: make(map[string]BatonStrategiesyRunner),
		logger:                   logger.WithName("BatonStrategiesRunnerManager"),
	}
}

func (r *BatonStrategiesRunnerManager) IsManaged(baton batonv1.Baton) bool {
	metadata := baton.ObjectMeta
	key := fmt.Sprintf("%s-%s", metadata.Namespace, metadata.Name)
	_, isManaged := r.batonStrategiesRunnerMap[key]
	return isManaged
}

func (r *BatonStrategiesRunnerManager) IsUpdated(baton batonv1.Baton) bool {
	metadata := baton.ObjectMeta
	key := fmt.Sprintf("%s-%s", metadata.Namespace, metadata.Name)
	batonStrategiesRunner := r.batonStrategiesRunnerMap[key]
	return batonStrategiesRunner.IsUpdatedBatonStrategies(baton)
}

func (r *BatonStrategiesRunnerManager) Add(baton batonv1.Baton) {
	metadata := baton.ObjectMeta
	key := fmt.Sprintf("%s-%s", metadata.Namespace, metadata.Name)
	batonStrategiesRunner := NewBatonStrategiesyRunner(r.client, baton, r.logger, key)
	batonStrategiesRunner.Run()
	r.batonStrategiesRunnerMap[key] = batonStrategiesRunner
	r.logger.Info(fmt.Sprintf("%s is Started", key))
}

func (r *BatonStrategiesRunnerManager) Delete(baton batonv1.Baton) {
	metadata := baton.ObjectMeta
	key := fmt.Sprintf("%s-%s", metadata.Namespace, metadata.Name)
	batonRunner := r.batonStrategiesRunnerMap[key]
	batonRunner.Stop()
	delete(r.batonStrategiesRunnerMap, key)
	r.logger.Info(fmt.Sprintf("%s is Stoped", key))
}

func (r *BatonStrategiesRunnerManager) DeleteNotExists(batons *batonv1.BatonList) {
	expectedKeys := []string{}
	batonMap := make(map[string]batonv1.Baton)
	for _, baton := range batons.Items {
		metadata := baton.ObjectMeta
		key := fmt.Sprintf("%s-%s", metadata.Namespace, metadata.Name)
		expectedKeys = append(expectedKeys, key)
		batonMap[key] = baton
	}

	batonStrategiesRunneKeys := r.getBatonStrategiesRunnerKeys()
	for _, batonStrategiesRunneKey := range batonStrategiesRunneKeys {
		if !contains(expectedKeys, batonStrategiesRunneKey) {
			r.Delete(batonMap[batonStrategiesRunneKey])
		}
	}
}

func (r *BatonStrategiesRunnerManager) getBatonStrategiesRunnerKeys() []string {
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
