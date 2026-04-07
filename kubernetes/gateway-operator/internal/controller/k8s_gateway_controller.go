/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	contctrl "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/helmgateway"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
)

const (
	k8sGatewayFinalizer = "gateway.api-platform.wso2.com/k8s-gateway-finalizer"
)

// K8sGatewayReconciler reconciles gateway.networking.k8s.io/Gateway (platform Helm + registry).
type K8sGatewayReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *config.OperatorConfig
	Logger *zap.Logger
}

// NewK8sGatewayReconciler creates a reconciler for gateway.networking.k8s.io Gateway.
func NewK8sGatewayReconciler(cl client.Client, scheme *runtime.Scheme, cfg *config.OperatorConfig, logger *zap.Logger) *K8sGatewayReconciler {
	return &K8sGatewayReconciler{
		Client: cl,
		Scheme: scheme,
		Config: cfg,
		Logger: logger,
	}
}

//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/finalizers,verbs=update

func (r *K8sGatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	gw := &gatewayv1.Gateway{}
	if err := r.Get(ctx, req.NamespacedName, gw); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !r.Config.ManagedGatewayClass(string(gw.Spec.GatewayClassName)) {
		return ctrl.Result{}, nil
	}

	log := r.Logger.With(zap.String("controller", "K8sGateway"), zap.String("namespace", req.Namespace), zap.String("name", req.Name))

	if !gw.DeletionTimestamp.IsZero() {
		return r.reconcileDeletion(ctx, gw, log)
	}

	if !controllerutil.ContainsFinalizer(gw, k8sGatewayFinalizer) {
		controllerutil.AddFinalizer(gw, k8sGatewayFinalizer)
		if err := r.Update(ctx, gw); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if err := r.syncGateway(ctx, gw, log); err != nil {
		log.Error("sync gateway", zap.Error(err))
		_ = r.patchGatewayStatus(ctx, gw, metav1.Condition{
			Type:               string(gatewayv1.GatewayConditionProgrammed),
			Status:             metav1.ConditionFalse,
			Reason:             string(gatewayv1.GatewayReasonPending),
			Message:            err.Error(),
			ObservedGeneration: gw.Generation,
			LastTransitionTime: metav1.Now(),
		})
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *K8sGatewayReconciler) reconcileDeletion(ctx context.Context, gw *gatewayv1.Gateway, log *zap.Logger) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(gw, k8sGatewayFinalizer) {
		return ctrl.Result{}, nil
	}

	ns := gw.Namespace
	if ns == "" {
		ns = "default"
	}

	registry.GetGatewayRegistry().Unregister(ns, gw.Name)

	if err := helmgateway.Uninstall(ctx, log, r.Config, gw.Name, ns); err != nil {
		log.Error("helm uninstall", zap.Error(err))
		return ctrl.Result{}, err
	}

	if err := r.Get(ctx, client.ObjectKeyFromObject(gw), gw); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	controllerutil.RemoveFinalizer(gw, k8sGatewayFinalizer)
	if err := r.Update(ctx, gw); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *K8sGatewayReconciler) syncGateway(ctx context.Context, gw *gatewayv1.Gateway, log *zap.Logger) error {
	ns := gw.Namespace
	if ns == "" {
		ns = "default"
	}

	apiSel, err := parseK8sGatewayAPISelector(gw)
	if err != nil {
		return fmt.Errorf("api selector annotation: %w", err)
	}

	valuesFile := r.Config.Gateway.HelmValuesFilePath
	var valuesYAML string
	cmName := gw.Annotations[AnnK8sGatewayHelmValuesConfigMap]
	if cmName != "" {
		valuesYAML, err = configMapValuesYAML(ctx, r.Client, cmName, ns)
		if err != nil {
			return err
		}
		log.Info("Merging Helm values: operator base file + ConfigMap overlay",
			zap.String("configMap", cmName),
			zap.String("valuesFile", valuesFile))
	} else {
		log.Info("Using default Helm values file", zap.String("path", valuesFile))
	}

	dockerUser, dockerPass, _ := r.getDockerHubCredentials(ctx)

	sig, err := helmInstallSignature(valuesYAML, valuesFile, r.Config, gw)
	if err != nil {
		return err
	}
	if gw.Annotations == nil {
		gw.Annotations = map[string]string{}
	}
	if gw.Annotations[AnnK8sGatewayHelmValuesHash] != sig {
		if err := helmgateway.InstallOrUpgrade(ctx, helmgateway.DeployInput{
			Logger:         r.Logger,
			Config:         r.Config,
			GatewayName:    gw.Name,
			Namespace:      ns,
			ValuesYAML:     valuesYAML,
			ValuesFilePath: valuesFile,
			DockerUsername: dockerUser,
			DockerPassword: dockerPass,
		}); err != nil {
			return fmt.Errorf("helm: %w", err)
		}
		if err := r.patchHelmValuesHash(ctx, gw, sig); err != nil {
			return fmt.Errorf("helm hash annotation: %w", err)
		}
		if err := r.Get(ctx, client.ObjectKeyFromObject(gw), gw); err != nil {
			return err
		}
		log.Info("Helm install/upgrade applied", zap.String("valuesHash", sig))
	} else {
		log.Info("Skipping Helm install/upgrade; values signature unchanged", zap.String("valuesHash", sig))
	}

	cpHost := gw.Annotations[AnnK8sGatewayControlPlaneHost]
	helmCM := cmName

	if err := registerGatewayInRegistry(ctx, r.Client, gw.Name, ns, apiSel, cpHost, helmCM, true); err != nil {
		return fmt.Errorf("register: %w", err)
	}

	ready, msg, err := evaluateGatewayDeploymentsReady(ctx, r.Client, gw.Name, ns)
	if err != nil {
		return err
	}
	if !ready {
		_ = r.patchGatewayStatus(ctx, gw, metav1.Condition{
			Type:               string(gatewayv1.GatewayConditionProgrammed),
			Status:             metav1.ConditionFalse,
			Reason:             string(gatewayv1.GatewayReasonPending),
			Message:            msg,
			ObservedGeneration: gw.Generation,
			LastTransitionTime: metav1.Now(),
		})
		return fmt.Errorf("pending: %s", msg)
	}

	return r.patchGatewayStatus(ctx, gw,
		metav1.Condition{
			Type:               string(gatewayv1.GatewayConditionAccepted),
			Status:             metav1.ConditionTrue,
			Reason:             string(gatewayv1.GatewayReasonAccepted),
			Message:            "Gateway accepted",
			ObservedGeneration: gw.Generation,
			LastTransitionTime: metav1.Now(),
		},
		metav1.Condition{
			Type:               string(gatewayv1.GatewayConditionProgrammed),
			Status:             metav1.ConditionTrue,
			Reason:             string(gatewayv1.GatewayReasonProgrammed),
			Message:            msg,
			ObservedGeneration: gw.Generation,
			LastTransitionTime: metav1.Now(),
		},
	)
}

func (r *K8sGatewayReconciler) patchGatewayStatus(ctx context.Context, gw *gatewayv1.Gateway, conds ...metav1.Condition) error {
	latest := &gatewayv1.Gateway{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(gw), latest); err != nil {
		return err
	}
	base := latest.DeepCopy()
	for _, cond := range conds {
		meta.SetStatusCondition(&latest.Status.Conditions, cond)
	}
	return r.Status().Patch(ctx, latest, client.MergeFrom(base))
}

func parseK8sGatewayAPISelector(gw *gatewayv1.Gateway) (*apiv1.APISelector, error) {
	raw := gw.Annotations[AnnK8sGatewayAPISelector]
	if raw == "" {
		return &apiv1.APISelector{Scope: apiv1.ClusterScope}, nil
	}
	var s apiv1.APISelector
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// SetupWithManager registers the Kubernetes Gateway controller.
func (r *K8sGatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := contctrl.Options{MaxConcurrentReconciles: r.Config.Reconciliation.MaxConcurrentReconciles}
	if opts.MaxConcurrentReconciles <= 0 {
		opts.MaxConcurrentReconciles = 1
	}
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(opts).
		For(&gatewayv1.Gateway{}, builder.WithPredicates(k8sGatewayReconcilePredicate())).
		Complete(r)
}

func (r *K8sGatewayReconciler) getDockerHubCredentials(ctx context.Context) (string, string, error) {
	secret := &corev1.Secret{}
	if r.Config.Gateway.RegistryCredentialsSecret == nil {
		return "", "", nil
	}
	secretRef := r.Config.Gateway.RegistryCredentialsSecret
	if secretRef.Name == "" || secretRef.Namespace == "" {
		return "", "", nil
	}
	usernameKey, passwordKey := secretRef.UsernameKey, secretRef.PasswordKey
	if usernameKey == "" {
		usernameKey = "username"
	}
	if passwordKey == "" {
		passwordKey = "password"
	}
	if err := r.Get(ctx, client.ObjectKey{Name: secretRef.Name, Namespace: secretRef.Namespace}, secret); err != nil {
		return "", "", err
	}
	return string(secret.Data[usernameKey]), string(secret.Data[passwordKey]), nil
}
