/*
Copyright 2022.

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
	"fmt"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/template-operator/api/v1alpha1"
)

// SecondReconciler reconciles a Sample object.
type SecondReconciler struct {
	client.Client
	*rest.Config
	record.EventRecorder
	FinalDeletionState v1alpha1.State
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecondReconciler) SetupWithManager(mgr ctrl.Manager, rateLimiter RateLimiter) error {
	r.Config = mgr.GetConfig()

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Sample{}).
		WithOptions(controller.Options{
			RateLimiter: TemplateRateLimiter(
				rateLimiter.BaseDelay,
				rateLimiter.FailureMaxDelay,
				rateLimiter.Frequency,
				rateLimiter.Burst,
			),
		}).
		Complete(r); err != nil {
		return fmt.Errorf("error while setting up controller: %w", err)
	}
	return nil
}

// Reconcile is the entry point from the controller-runtime framework.
// It performs a reconciliation based on the passed ctrl.Request object.
func (r *SecondReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	objectInstance := v1alpha1.Sample{}

	if err := r.Client.Get(ctx, req.NamespacedName, &objectInstance); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		logger.Info(req.NamespacedName.String() + " got deleted!")
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, fmt.Errorf("error while getting object: %w", err)
		}
		return ctrl.Result{}, nil
	}

	logger.Info("[SecondReconciler]: Reconciling Sample CR", "name", objectInstance.Name)

	// check if deletionTimestamp is set, retry until it gets deleted
	status := getStatusFromSample(&objectInstance)

	// set state to FinalDeletionState (default is Deleting) if not set for an object with deletion timestamp
	if !objectInstance.GetDeletionTimestamp().IsZero() && status.State != r.FinalDeletionState {
		return ctrl.Result{}, r.setStatusForObjectInstance(ctx, &objectInstance, status.WithState(r.FinalDeletionState))
	}

	if objectInstance.GetDeletionTimestamp().IsZero() {
		// add finalizer if not present
		if controllerutil.AddFinalizer(&objectInstance, finalizer) {
			return ctrl.Result{}, r.ssa(ctx, &objectInstance)
		}
	}

	switch status.State {
	case "":
		return ctrl.Result{}, r.HandleInitialState(ctx, &objectInstance)
	case v1alpha1.StateProcessing, v1alpha1.StateDeleting, v1alpha1.StateError:
		return ctrl.Result{Requeue: true}, r.HandleAnyOtherState(ctx, &objectInstance)
	case v1alpha1.StateReady, v1alpha1.StateWarning:
		return ctrl.Result{RequeueAfter: requeueInterval}, r.HandleAnyOtherState(ctx, &objectInstance)
	}

	return ctrl.Result{}, nil
}

// HandleInitialState bootstraps state handling for the reconciled resource.
func (r *SecondReconciler) HandleInitialState(ctx context.Context, objectInstance *v1alpha1.Sample) error {
	status := getStatusFromSample(objectInstance)

	return r.setStatusForObjectInstance(ctx, objectInstance, status.
		WithDivisibleByThreeConditionStatus(metav1.ConditionUnknown, objectInstance.GetGeneration()))
}

func (r *SecondReconciler) HandleAnyOtherState(ctx context.Context, objectInstance *v1alpha1.Sample) error {
	status := getStatusFromSample(objectInstance)

	someNumberStr := objectInstance.Spec.SomeNumber
	divisibleByThreeCondition := metav1.ConditionFalse

	if someNumberStr != "" {
		someNumber, err := strconv.Atoi(someNumberStr)
		if err == nil {
			if someNumber%3 == 0 {
				divisibleByThreeCondition = metav1.ConditionTrue
			}
		}
	}

	// set eventual state to Ready - if no errors were found
	return r.setStatusForObjectInstance(ctx, objectInstance, status.
		WithDivisibleByThreeConditionStatus(divisibleByThreeCondition, objectInstance.GetGeneration()))
}

// HandleReadyState checks for the consistency of reconciled resource, by verifying the underlying resources.
func (r *SecondReconciler) HandleReadyState(ctx context.Context, objectInstance *v1alpha1.Sample) error {
	status := getStatusFromSample(objectInstance)

	someNumberStr := objectInstance.Spec.SomeNumber
	divisibleByThreeCondition := metav1.ConditionFalse

	if someNumberStr != "" {
		someNumber, err := strconv.Atoi(someNumberStr)
		if err == nil {
			if someNumber%3 == 0 {
				divisibleByThreeCondition = metav1.ConditionTrue
			}
		}
	}

	// set eventual state to Ready - if no errors were found
	return r.setStatusForObjectInstance(ctx, objectInstance, status.
		WithDivisibleByThreeConditionStatus(divisibleByThreeCondition, objectInstance.GetGeneration()))
}

func (r *SecondReconciler) setStatusForObjectInstance(ctx context.Context, objectInstance *v1alpha1.Sample,
	status *v1alpha1.SampleStatus,
) error {
	objectInstance.Status = *status

	if err := r.ssaStatus(ctx, objectInstance); err != nil {
		r.Event(objectInstance, "Warning", "ErrorUpdatingStatus",
			fmt.Sprintf("updating state to %v", string(status.State)))
		return fmt.Errorf("error while updating status %s to: %w", status.State, err)
	}

	r.Event(objectInstance, "Normal", "StatusUpdated", fmt.Sprintf("updating state to %v", string(status.State)))
	return nil
}

// ssaStatus patches status using SSA on the passed object.
func (r *SecondReconciler) ssaStatus(ctx context.Context, obj client.Object) error {
	obj.SetManagedFields(nil)
	obj.SetResourceVersion("")
	if err := r.Status().Patch(ctx, obj, client.Apply,
		&client.SubResourcePatchOptions{PatchOptions: client.PatchOptions{FieldManager: fieldOwner}}); err != nil {
		return fmt.Errorf("error while patching status: %w", err)
	}
	return nil
}

// ssa patches the object using SSA.
func (r *SecondReconciler) ssa(ctx context.Context, obj client.Object) error {
	obj.SetManagedFields(nil)
	obj.SetResourceVersion("")
	if err := r.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner(fieldOwner)); err != nil {
		return fmt.Errorf("error while patching object: %w", err)
	}
	return nil
}
