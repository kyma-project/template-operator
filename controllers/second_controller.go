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
	"math/rand"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
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
		Named("second_reconciler").
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

	// wait for the main controller to correctly set status:
	if objectInstance.Status.State == "" {
		return ctrl.Result{RequeueAfter: requeueInterval}, nil
	}

	// wait for random time [1, 100] ms
	time.Sleep(time.Duration(1+rand.Intn(100)) * time.Millisecond) //nolint:gosec,mnd // pseduo-random sleep time

	// check if deletionTimestamp is set, retry until it gets deleted
	existingStatus := getStatusFromSample(&objectInstance)

	switch existingStatus.State {
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
	// Note there is no 'state' field in the status - we only set the conditions. Setting the state is done by the main controller.
	// Actually setting the state here is causing conflicts with the main controller.
	partialStatus := (&v1alpha1.SampleStatus{}).
		WithDivisibleByThreeConditionStatus(metav1.ConditionUnknown, objectInstance.GetGeneration())

	return r.setStatusForObjectInstance(ctx, client.ObjectKeyFromObject(objectInstance), partialStatus)
}

func (r *SecondReconciler) HandleAnyOtherState(ctx context.Context, objectInstance *v1alpha1.Sample) error {
	// partialStatus := (&v1alpha1.SampleStatus{}) //.WithState(objectInstance.Status.State)

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

	// NOTE: there is no 'state' field in the status - we only set the conditions.
	//       Setting the state is done by the main controller.
	//       Setting the state here leads to field ownership conflicts with the main controller.
	partialStatus := (&v1alpha1.SampleStatus{}).WithDivisibleByThreeConditionStatus(divisibleByThreeCondition, objectInstance.GetGeneration())

	// set eventual state to Ready - if no errors were found
	return r.setStatusForObjectInstance(ctx, client.ObjectKeyFromObject(objectInstance), partialStatus)
}

func (r *SecondReconciler) setStatusForObjectInstance(ctx context.Context, targetObjectKey client.ObjectKey,
	partialStatus *v1alpha1.SampleStatus,
) error {
	if err := r.ssaStatus(ctx, targetObjectKey, partialStatus); err != nil {
		return fmt.Errorf("error updating status %s to: %w", partialStatus.State, err)
	}
	return nil
}

// ssaStatus patches status using SSA on the passed object.
func (r *SecondReconciler) ssaStatus(ctx context.Context, targetObjectKey client.ObjectKey, partialStatus *v1alpha1.SampleStatus) error {
	dynClient, err := dynamic.NewForConfig(r.Config)
	if err != nil {
		return fmt.Errorf("error creating dynamic client: %w", err)
	}

	// Note: Gvr is NOT Gvk! (GroupVersionResource vs GroupVersionKind)
	sampleGvr := schema.GroupVersionResource{Group: "operator.kyma-project.io", Version: "v1alpha1", Resource: "samples"}
	unstructuredPatch := toUnstructured(targetObjectKey.Name, targetObjectKey.Namespace, partialStatus)

	/*
		json, err := unstructuredSample.MarshalJSON()
		if err != nil {
			return err
		}
		fmt.Println("========================================")
		fmt.Println(string(json))
		fmt.Println("========================================")
	*/

	_, err = dynClient.Resource(sampleGvr).
		Namespace(targetObjectKey.Namespace).
		ApplyStatus(ctx, targetObjectKey.Name, unstructuredPatch, metav1.ApplyOptions{FieldManager: "sample.kyma-project.io/secondowner", Force: true})
	if err != nil {
		return fmt.Errorf("error patching status: %w", err)
	}

	return nil
}

func toUnstructured(name, namespace string, status *v1alpha1.SampleStatus) *unstructured.Unstructured {
	res := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "operator.kyma-project.io/v1alpha1",
			"kind":       "Sample",
			"metadata":   map[string]interface{}{"name": name, "namespace": namespace},
			"status": map[string]interface{}{
				// "state":      status.State, <- do not set state here, it is set by the main controller
				"conditions": conditionListToUnstructured(status.Conditions),
			},
		},
	}
	return res
}

func conditionListToUnstructured(conditions []metav1.Condition) []map[string]interface{} {
	res := make([]map[string]interface{}, 0, len(conditions))
	for _, condition := range conditions {
		res = append(res, conditionToUnstructured(condition))
	}
	return res
}

func conditionToUnstructured(condition metav1.Condition) map[string]interface{} {
	return map[string]interface{}{
		"lastTransitionTime": condition.LastTransitionTime,
		"message":            condition.Message,
		"observedGeneration": condition.ObservedGeneration,
		"reason":             condition.Reason,
		"status":             condition.Status,
		"type":               condition.Type,
	}
}
