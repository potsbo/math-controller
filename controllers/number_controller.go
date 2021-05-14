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
	"math"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/potsbo/math-controller/api/v1beta1"
	mathv1beta1 "github.com/potsbo/math-controller/api/v1beta1"
)

// NumberReconciler reconciles a Number object
type NumberReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=math.potsbo.k8s.wantedly.com,resources=numbers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=math.potsbo.k8s.wantedly.com,resources=numbers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=math.potsbo.k8s.wantedly.com,resources=numbers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Number object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.2/pkg/reconcile
func (r *NumberReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("number", req.NamespacedName)

	// your logic here
	obj := v1beta1.Number{}

	if err := r.Client.Get(ctx, req.NamespacedName, &obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, errors.WithStack(err)
	}

	{
		obj.Status.FizzBuzz = fizzbuzz(obj.Spec.Value)
		obj.Status.IsSquare = isSquare(obj.Spec.Value)
		logger.Info("New Status", "status", obj.Status)
		if err := r.Status().Update(ctx, &obj); err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
	}

	return ctrl.Result{}, nil
}

func fizzbuzz(n int64) string {
	if n%15 == 0 {
		return "FizzBuzz"
	}
	if n%3 == 0 {
		return "Fizz"
	}
	if n%5 == 0 {
		return "Buzz"
	}

	return fmt.Sprintf("%d", n)
}

func isSquare(num int64) bool {
	sqrt := int64(math.Floor(math.Sqrt(float64(num))))
	return sqrt*sqrt == num
}

// SetupWithManager sets up the controller with the Manager.
func (r *NumberReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mathv1beta1.Number{}).
		Complete(r)
}
