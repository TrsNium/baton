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

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	batonv1 "trsnium.com/baton/api/v1"
)

// BatonReconciler reconciles a Baton object
type BatonReconciler struct {
	client.Client
	Log                          logr.Logger
	Scheme                       *runtime.Scheme
	BatonStrategiesRunnerManager *BatonStrategiesRunnerManager
}

// +kubebuilder:rbac:groups=baton.baton,resources=batons,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=baton.baton,resources=batons/status,verbs=get;update;patch
func (r *BatonReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	batons := &batonv1.BatonList{}
	if err := r.Client.List(ctx, batons, &client.ListOptions{}); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	for _, baton := range batons.Items {
		if !r.BatonStrategiesRunnerManager.IsManaged(baton) {
			r.BatonStrategiesRunnerManager.Add(baton)
			continue
		}

		if r.BatonStrategiesRunnerManager.IsUpdated(baton) {
			r.BatonStrategiesRunnerManager.Delete(baton)
			r.BatonStrategiesRunnerManager.Add(baton)
		}
	}

	r.BatonStrategiesRunnerManager.DeleteNotExists(batons)
	return ctrl.Result{}, nil
}

func (r *BatonReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batonv1.Baton{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
