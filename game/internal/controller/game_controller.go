/*
Copyright 2024 costa92.

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
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/pointer"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	appsv1 "k8s.io/api/apps/v1"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gamev1 "github.com/costa92/game/api/v1"
)

// GameReconciler reconciles a Game object
type GameReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// 添加rbac规则
//+kubebuilder:rbac:groups=game.game.com,resources=games,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=game.game.com,resources=games/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=game.game.com,resources=games/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Game object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *GameReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	defer utilruntime.HandleCrash()
	logger := log.FromContext(ctx)
	logger.Info("Reconciling Game", "name", req.String())

	// 根据req获取Game资源
	game := &gamev1.Game{}
	if err := r.Get(ctx, req.NamespacedName, game); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 判断是否被删除
	if game.DeletionTimestamp != nil {
		logger.Info("Deleting Game", "name", req.String())
		return ctrl.Result{}, nil
	}
	// 调用syncGame
	if err := r.syncGame(ctx, game); err != nil {
		logger.Error(err, "Failed to sync Game", "name", req.String())
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

const (
	gameLabelName = "game-label"
	port          = 80
)

// syncGame 确保Game的Deployment，Service和Ingress都处于预期的状态，
func (r *GameReconciler) syncGame(ctx context.Context, game *gamev1.Game) error {
	logger := log.FromContext(ctx)

	//创建深拷贝,避免修改原对象
	game = game.DeepCopy()
	name := types.NamespacedName{
		Namespace: game.Namespace,
		Name:      game.Name,
	}

	// 创建OwnerReference数组，将Game资源设置为子资源的所有者。确保当Game资源被删除时，它的子资源也会被自动删除。

	ower := []metav1.OwnerReference{
		{
			APIVersion: game.APIVersion,
			Kind:       game.Kind,
			Name:       game.Name,
			// pointer.Bool(true)返回一个指向true的布尔型指针，赋值给Controller字段
			Controller: pointer.Bool(true),
			// 指定BlockOwnerDeletion为true，用于控制是否应该在删除父资源时同时删除子资源。
			BlockOwnerDeletion: pointer.Bool(true),
			UID:                game.UID, // UID是资源的唯一标识符
		},
	}

	// 使用Game资源的名称创建一个标签映射，用于标识该资源所管理的子资源。
	labels := map[string]string{
		gameLabelName: game.Name,
	}

	// 设置元数据
	meta := metav1.ObjectMeta{
		Name:            name.Name,
		Namespace:       name.Namespace,
		Labels:          labels,
		OwnerReferences: ower,
	}

	// 创建或更新Deployment
	deploy, err := r.syncDeployment(ctx, game, name, meta)
	if err != nil {
		logger.Error(err, "Failed to sync Deployment", "name", name)
		return err
	}

	// 创建或更新Service
	if err := r.syncService(ctx, game, name, meta); err != nil {
		logger.Error(err, "Failed to sync Service", "name", name)
		return err
	}

	// 创建或更新Ingress
	if err := r.syncIngress(ctx, game, name, meta, deploy); err != nil {
		logger.Error(err, "Failed to sync Ingress", "name", name)
		return err
	}

	return nil
}

// syncIngress 确保Game的Ingress处于预期状态
func (r *GameReconciler) syncIngress(ctx context.Context, game *gamev1.Game, name types.NamespacedName, meta metav1.ObjectMeta, deploy *appsv1.Deployment) error {
	logger := log.FromContext(ctx)

	ing := &networkingv1.Ingress{}
	if err := r.Get(ctx, name, ing); err != nil {
		if !errors.IsNotFound(err) {
			logger.Error(err, "Failed to get Ingress", "name", game.Name)
			return err
		}
		// 创建Ingress
		ing = &networkingv1.Ingress{
			ObjectMeta: meta,
			Spec:       getIngressSpec(game),
		}
		if err := r.Create(ctx, ing); err != nil {
			return err
		}
		logger.Info("Create ingress success", "name", name.String())
	}

	// 更新Game资源的Status
	newStatus := gamev1.GameStatus{
		Replicas:      deploy.Status.Replicas,
		ReadyReplicas: deploy.Status.ReadyReplicas,
	}

	// 如果就绪副本数等于当前副本数,则将状态设置为Running; 否则设置为NotReady。
	if game.Status.Replicas == newStatus.ReadyReplicas {
		newStatus.Phase = gamev1.Running
	} else {
		newStatus.Phase = gamev1.NotReady
	}

	// 如果Game资源的状态发生变化,则更新Game资源的状态子资源和Game资源。
	if !reflect.DeepEqual(game.Status, newStatus) {
		game.Status = newStatus
		logger.Info("Update game status", "name", name.String())
		if err := r.Client.Status().Update(ctx, game); err != nil {
			return err
		}
		game.Spec.Replicas = newStatus.Replicas
		logger.Info("Update game spec", "name", name.String())
		if err := r.Client.Update(ctx, game); err != nil {
			return err
		}
	}

	return nil
}

// syncService 确保Game的Service处于预期状态
func (r *GameReconciler) syncService(ctx context.Context, game *gamev1.Game, name types.NamespacedName, meta metav1.ObjectMeta) error {
	logger := log.FromContext(ctx)
	// 创建Service
	service := &corev1.Service{}
	if err := r.Get(ctx, name, service); err != nil {
		if !errors.IsNotFound(err) {
			logger.Error(err, "Failed to get Service", "name", game.Name)
			return err
		}
		// 创建Service
		service = &corev1.Service{
			ObjectMeta: meta,
			Spec: corev1.ServiceSpec{
				// Selector 字段指定了Service控制的Pod的标签选择器。
				Selector: meta.Labels,
				// Ports字段指定了Service的端口映射规则。
				Ports: []corev1.ServicePort{
					{
						Name:     "http",
						Protocol: corev1.ProtocolTCP,
						Port:     port,
						TargetPort: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: port,
						},
					},
				},
			},
		}

		if err := r.Create(ctx, service); err != nil {
			logger.Error(err, "Failed to create Service", "name", name.String())
			return nil
		}
		logger.Info("Create service success", "name", name.String())
	}
	return nil
}

// syncDeployment 确保Game的Deployment处于预期状态
func (r *GameReconciler) syncDeployment(ctx context.Context, game *gamev1.Game, name types.NamespacedName, meta metav1.ObjectMeta) (*appsv1.Deployment, error) {
	logger := log.FromContext(ctx)
	// 创建Deployment
	deployment := &appsv1.Deployment{}
	// 获取是否存在
	if err := r.Get(ctx, name, deployment); err != nil {
		if !errors.IsNotFound(err) {
			logger.Error(err, "Failed to get Deployment", "name", deployment.Name)
			return nil, err
		}
		// 创建Deployment
		deployment = &appsv1.Deployment{
			ObjectMeta: meta,
			// Spec是Deployment的规范部分，用于描述Deployment的期望状态。
			Spec: getDeploymentSpec(game, meta.Labels),
		}

		if err := r.Create(ctx, deployment); err != nil {
			logger.Error(err, "Failed to create Deployment", "name", deployment.Name)
			return nil, err
		}
		logger.Info("Create deployment success", "name", name.String())
	} else {
		// 比较期望Spec和当前的Spec是否完全一致，如果不同则更新Deployment为期望Spec。
		desire := getDeploymentSpec(game, meta.Labels)
		now := getSpecFromDeployment(deployment)
		// reflect.DeepEqual比较两个对象是否相等
		if !reflect.DeepEqual(desire, now) {
			// 创建一个Deployment的深拷贝
			newDeploy := deployment.DeepCopy()
			// 更新Deployment的Spec
			newDeploy.Spec = desire

			if err := r.Update(ctx, newDeploy); err != nil {
				logger.Error(err, "Failed to update Deployment", "name", deployment.Name)
				return nil, err
			}
			logger.Info("Update deployment success", "name", name.String())
		}
	}
	logger.Info("Created Deployment", "name", deployment.Name)
	return deployment, nil
}

// getDeploymentSpec 为Game资源创建DeploymentSpec
func getDeploymentSpec(game *gamev1.Game, labels map[string]string) appsv1.DeploymentSpec {
	return appsv1.DeploymentSpec{
		//  replicas字段指定了Deployment的副本数，即期望的Pod数量。
		Replicas: pointer.Int32(game.Spec.Replicas),
		// selector字段指定了Deployment控制的Pod的标签选择器。
		// 使用提供的标签映射创建一个LabelSelector
		Selector: metav1.SetAsLabelSelector(labels),
		// 创建Pod模板，设置标签，指定容器名字和镜像。
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  game.Name,
						Image: game.Spec.Image,
					},
				},
			},
		},
	}
}

// getSpecFromDeployment 从Deployment中获取Spec
func getSpecFromDeployment(deployment *appsv1.Deployment) appsv1.DeploymentSpec {
	// 获取第一个容器的Spec，pod只有一个主容器
	container := deployment.Spec.Template.Spec.Containers[0]
	return appsv1.DeploymentSpec{
		Replicas: deployment.Spec.Replicas,
		Selector: deployment.Spec.Selector,
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				// 从Deployment的Pod模板规格中获取标签
				Labels: deployment.Spec.Template.Labels,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  container.Name,
						Image: container.Image,
					},
				},
			},
		},
	}
}

// getIngressSpec 根据Game和labels，生成Ingress的期望Spec
func getIngressSpec(game *gamev1.Game) networkingv1.IngressSpec {
	pathType := networkingv1.PathTypePrefix
	return networkingv1.IngressSpec{
		Rules: []networkingv1.IngressRule{
			{
				Host: game.Spec.Host,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{
								Path:     "/",
								PathType: &pathType,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: game.Name,
										Port: networkingv1.ServiceBackendPort{
											Number: int32(port),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *GameReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// MaxConcurrentReconciles是可以运行的最大并发Reconciles数。默认值为 1。
		WithOptions(controller.Options{MaxConcurrentReconciles: 3}).
		For(&gamev1.Game{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
