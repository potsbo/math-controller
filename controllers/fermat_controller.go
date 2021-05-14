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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	util "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/potsbo/math-controller/api/v1beta1"
	mathv1beta1 "github.com/potsbo/math-controller/api/v1beta1"
)

// FermatReconciler reconciles a Fermat object
type FermatReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=math.potsbo.k8s.wantedly.com,resources=fermats,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=math.potsbo.k8s.wantedly.com,resources=fermats/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=math.potsbo.k8s.wantedly.com,resources=fermats/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Fermat object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.2/pkg/reconcile
func (r *FermatReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("fermat", req.NamespacedName)

	// your logic here
	up := FermatUpdater{r.Client, r.Log, r.Scheme}
	if err := up.Update(ctx, req.NamespacedName); err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FermatReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mathv1beta1.Fermat{}).
		Complete(r)
}

type FermatUpdater struct {
	Client client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *FermatUpdater) Update(ctx context.Context, req types.NamespacedName) error {
	_ = r.Log.WithValues("fermat updater", req)

	// your logic here
	parent := v1beta1.Fermat{}
	if err := r.Client.Get(ctx, req, &parent); err != nil {
		return client.IgnoreNotFound(err)
	}

	desireds := []v1beta1.Tuple{}
	for i := int64(1); i <= parent.Spec.Number/2; i++ {
		desired := v1beta1.Tuple{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%d-and-%d", req.Name, i, parent.Spec.Number-i),
				Namespace: req.Namespace,
			},
			Spec: v1beta1.TupleSpec{
				Numbers: []int64{i, parent.Spec.Number - i},
			},
		}
		desireds = append(desireds, desired)
	}

	deleteTargets := []v1beta1.Tuple{}
	children, err := r.getChildren(ctx, parent)
	if err != nil {
		return errors.WithStack(err)
	}
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
		obj := v1beta1.Tuple{
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

func (r *FermatUpdater) getChildren(ctx context.Context, parent v1beta1.Fermat) ([]v1beta1.Tuple, error) {
	children := []v1beta1.Tuple{}
	existings := v1beta1.TupleList{}
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

func (r *FermatUpdater) UpdateStatus(ctx context.Context, req types.NamespacedName) error {
	parent := v1beta1.Fermat{}
	if err := r.Client.Get(ctx, req, &parent); err != nil {
		return client.IgnoreNotFound(err)
	}

	children, err := r.getChildren(ctx, parent)
	if err != nil {
		return errors.WithStack(err)
	}

	{
		found := false
		for _, existing := range children {
			if existing.Status.AllSquares {
				found = true
			}
		}
		if parent.Status.TwoSquaresFound != found {
			parent.Status.TwoSquaresFound = found
			if err := r.Client.Status().Update(ctx, &parent); err != nil {
				return errors.WithStack(err)
			}
		}
	}

	return nil
}
