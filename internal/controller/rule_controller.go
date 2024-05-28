/*
Copyright 2024.

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

package controller

import (
	"context"
	"fmt"
	loggingv1 "github.com/Gentleelephant/logging-controller/api/v1"
	"github.com/Gentleelephant/logging-controller/internal/constants"
	"github.com/Gentleelephant/logging-controller/internal/impl"
	"github.com/Gentleelephant/logging-controller/internal/inter"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sync"
)

const (
	FinalizerName = "finalizer.logging.birdhk.com"
)

var (
	livingOP = make(map[string]inter.LogOperator)
	startOP  = make(chan inter.LogOperator, 10)
	stopOP   = make(chan inter.LogOperator, 10)
	mx       = sync.RWMutex{}
)

// RuleReconciler reconciles a Rule object
type RuleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=logging.birdhk.com,resources=rules,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=logging.birdhk.com,resources=rules/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=logging.birdhk.com,resources=rules/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Rule object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *RuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	rule := loggingv1.Rule{}
	if err := r.Get(ctx, req.NamespacedName, &rule); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !rule.DeletionTimestamp.IsZero() {
		// 从存活流列表中移除，停止接收数据
		RemoveElementByName(rule.Name)
		// 关闭流
		element := GetElement(rule.Name)
		if element != nil {
			// 从存活流列表中移除，停止接收数据
			RemoveElementByName(rule.Name)
			element.Close()
		}

		controllerutil.RemoveFinalizer(&rule, FinalizerName)
		if err := r.Update(ctx, &rule); err != nil {
			return ctrl.Result{}, err
		}
	}
	if containsFinalizer := controllerutil.ContainsFinalizer(&rule, "finalizer.logging.birdhk.com"); !containsFinalizer {
		rule.Finalizers = append(rule.Finalizers, FinalizerName)
		if err := r.Update(ctx, &rule); err != nil {
			return ctrl.Result{}, err
		}
	}

	tp := rule.Spec.Type
	switch tp {
	case constants.SlidingType:
		lo := GetElement(rule.Name)
		if lo != nil {
			// lo存在说明当前流正在运行，需要先关闭
			RemoveElementByName(rule.Name)
			lo.Close()
		}
		keylogInstance := impl.NewKeylogInstance(rule.Name, rule.Spec.Regex, rule.Spec.Parallelism, nil)
		AddElement(keylogInstance)
		// 启动流
		keylogInstance.Start()
	// 过滤规则
	case constants.FilterType:
		lo := GetElement(rule.Name)
		if lo != nil {
			// lo存在说明当前流正在运行，需要先关闭
			RemoveElementByName(rule.Name)
			lo.Close()
		}
		// 滑动窗口规则
		slidingWindowInstance := impl.NewSlidingWindowInstance(rule.Name, rule.Spec.WindowSize, rule.Spec.SlidingInterval, nil)
		AddElement(slidingWindowInstance)
		// 启动流
		slidingWindowInstance.Start()
	default:
		return ctrl.Result{}, fmt.Errorf("unknown type %s", tp)
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&loggingv1.Rule{}).
		Complete(r)
}

func AddElement(op inter.LogOperator) {
	mx.Lock()
	defer mx.Unlock()

	livingOP[op.GetName()] = op
}

func GetElement(name string) inter.LogOperator {
	mx.RLock()
	defer mx.RUnlock()
	return livingOP[name]
}

func RemoveElement(op inter.LogOperator) {
	mx.Lock()
	defer mx.Unlock()

	delete(livingOP, op.GetName())
}

func RemoveElementByName(name string) {
	mx.Lock()
	defer mx.Unlock()

	delete(livingOP, name)
}

func GetLivingOP() map[string]inter.LogOperator {
	mx.RLock()
	defer mx.RUnlock()
	return livingOP
}
