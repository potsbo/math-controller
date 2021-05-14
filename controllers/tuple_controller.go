/*
Copyright 2021.

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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/potsbo/math-controller/api/v1beta1"
	mathv1beta1 "github.com/potsbo/math-controller/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	util "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// TupleReconciler reconciles a Tuple object
type TupleReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=math.potsbo.k8s.wantedly.com,resources=tuples,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=math.potsbo.k8s.wantedly.com,resources=tuples/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=math.potsbo.k8s.wantedly.com,resources=tuples/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Tuple object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.2/pkg/reconcile
func (r *TupleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("tuple", req.NamespacedName)

	// your logic here
	up := TupleUpdater{r.Client, r.Log, r.Scheme}
	return ctrl.Result{}, up.Update(ctx, req.NamespacedName)
}

// SetupWithManager sets up the controller with the Manager.
func (r *TupleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mathv1beta1.Tuple{}).
		Complete(r)
}

type TupleUpdater struct {
	Client client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *TupleUpdater) Update(ctx context.Context, req types.NamespacedName) error {
	_ = r.Log.WithValues("tuple", req)

	// your logic here
	parent := v1beta1.Tuple{}
	if err := r.Client.Get(ctx, req, &parent); err != nil {
		return client.IgnoreNotFound(err)
	}

	desireds := []v1beta1.Number{}
	for i, v := range parent.Spec.Numbers {
		desired := v1beta1.Number{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%d", req.Name, i),
				Namespace: req.Namespace,
			},
			Spec: v1beta1.NumberSpec{
				Value: v,
			},
		}
		desireds = append(desireds, desired)
	}

	children, err := r.getChildren(ctx, parent)
	if err != nil {
		return errors.WithStack(err)
	}

	deleteTargets := []v1beta1.Number{}
	{
		desiredNames := map[string]bool{}
		for _, desired := range desireds {
			desiredNames[desired.Name] = true
		}

		for _, existing := range children {
			if _, ok := desiredNames[existing.Name]; !ok {
				deleteTargets = append(deleteTargets, existing)
			}
		}
	}

	for _, desired := range desireds {
		obj := v1beta1.Number{
			ObjectMeta: desired.ObjectMeta,
		}
		if _, err := util.CreateOrUpdate(ctx, r.Client, &obj, func() error {
			obj.Spec = desired.Spec
			return errors.WithStack(util.SetControllerReference(&parent, &obj, r.Scheme))
		}); err != nil {
			return errors.WithStack(err)
		}
	}

	for _, item := range deleteTargets {
		del := item
		if err := r.Client.Delete(ctx, &del); err != nil {
			return errors.WithStack(client.IgnoreNotFound(err))
		}
	}

	return errors.WithStack(r.UpdateStatus(ctx, req))
}

func (r *TupleUpdater) getChildren(ctx context.Context, parent v1beta1.Tuple) ([]v1beta1.Number, error) {
	children := []v1beta1.Number{}
	existings := v1beta1.NumberList{}
	if err := r.Client.List(ctx, &existings); err != nil {
		return nil, errors.WithStack(err)
	}
	for _, item := range existings.Items {
		i := item
		if ownedByParent(&i, &parent) {
			children = append(children, i)
		}
	}

	return children, nil
}

func (r *TupleUpdater) UpdateStatus(ctx context.Context, req types.NamespacedName) error {
	parent := v1beta1.Tuple{}
	if err := r.Client.Get(ctx, req, &parent); err != nil {
		return client.IgnoreNotFound(err)
	}

	children, err := r.getChildren(ctx, parent)
	if err != nil {
		return errors.WithStack(err)
	}

	{
		allSquare := true
		for _, existing := range children {
			if !existing.Status.IsSquare {
				allSquare = false
			}
		}
		if parent.Status.AllSquares != allSquare {
			parent.Status.AllSquares = allSquare
			if err := r.Client.Status().Update(ctx, &parent); err != nil {
				return errors.WithStack(err)
			}
		}
	}

	{
		up := FermatUpdater{r.Client, r.Log, r.Scheme}
		for _, ref := range parent.ObjectMeta.OwnerReferences {
			if ref.APIVersion == "math.potsbo.k8s.wantedly.com/v1beta1" && ref.Kind == "Fermat" {
				if err := up.UpdateStatus(ctx, types.NamespacedName{Namespace: req.Namespace, Name: ref.Name}); err != nil {
					return errors.WithStack(err)
				}
			}
		}
	}

	return nil
}

func (r *TupleUpdater) UpdateAll(ctx context.Context) error {
	existings := v1beta1.TupleList{}
	if err := r.Client.List(ctx, &existings); err != nil {
		return errors.WithStack(err)
	}

	for _, existing := range existings.Items {
		if err := r.Update(ctx, types.NamespacedName{Namespace: existing.Namespace, Name: existing.Name}); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func ownedByParent(child util.Object, parent util.Object) bool {
	parentGVK := parent.GetObjectKind().GroupVersionKind()

	for _, ref := range child.GetOwnerReferences() {
		if ref.Name != parent.GetName() {
			continue
		}

		if ref.APIVersion != parentGVK.GroupVersion().String() {
			continue
		}

		if ref.Kind != parentGVK.Kind {
			continue
		}

		return true
	}

	return false
}
