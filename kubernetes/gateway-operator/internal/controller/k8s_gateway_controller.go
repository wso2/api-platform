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
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	contctrl "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/helm"
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
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

func (r *K8sGatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	gw := &gatewayv1.Gateway{}
	if err := r.Get(ctx, req.NamespacedName, gw); err != nil {
		if apierrors.IsNotFound(err) {
			if r.Logger != nil {
				r.Logger.Debug("Gateway not found; likely deleted",
					zap.String("controller", "K8sGateway"),
					zap.String("namespace", req.Namespace),
					zap.String("name", req.Name))
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	log := r.Logger.With(zap.String("controller", "K8sGateway"), zap.String("namespace", req.Namespace), zap.String("name", req.Name))
	log.Info("reconcile Gateway",
		zap.Int64("generation", gw.Generation),
		zap.String("resourceVersion", gw.ResourceVersion),
		zap.String("gatewayClass", string(gw.Spec.GatewayClassName)))

	if !gw.DeletionTimestamp.IsZero() {
		return r.reconcileDeletion(ctx, gw, log)
	}

	if !r.Config.ManagedGatewayClass(string(gw.Spec.GatewayClassName)) {
		log.Debug("skip Gateway: class not managed by operator",
			zap.String("gatewayClass", string(gw.Spec.GatewayClassName)))
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(gw, k8sGatewayFinalizer) {
		controllerutil.AddFinalizer(gw, k8sGatewayFinalizer)
		if err := r.Update(ctx, gw); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("added Gateway finalizer; requeue")
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

	log.Info("Gateway reconcile complete (synced and status updated)",
		zap.String("gatewayClass", string(gw.Spec.GatewayClassName)))
	return ctrl.Result{}, nil
}

func (r *K8sGatewayReconciler) reconcileDeletion(ctx context.Context, gw *gatewayv1.Gateway, log *zap.Logger) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(gw, k8sGatewayFinalizer) {
		return ctrl.Result{}, nil
	}

	log.Info("reconcile Gateway deletion",
		zap.String("gatewayClass", string(gw.Spec.GatewayClassName)))

	ns := gw.Namespace
	if ns == "" {
		ns = "default"
	}

	registry.GetGatewayRegistry().Unregister(ns, gw.Name)
	log.Info("unregistered Gateway from operator registry",
		zap.String("namespace", ns),
		zap.String("name", gw.Name))

	log.Info("attempting Helm uninstall for Gateway deletion",
		zap.String("namespace", ns),
		zap.String("name", gw.Name))
	if err := helmgateway.Uninstall(ctx, log, r.Config, gw.Name, ns); err != nil {
		if isHelmReleaseNotFoundError(err) {
			log.Info("Helm release not found during Gateway deletion; continuing finalizer removal",
				zap.String("namespace", ns),
				zap.String("name", gw.Name),
				zap.Error(err))
		} else {
			log.Error("helm uninstall failed; will retry",
				zap.String("namespace", ns),
				zap.String("name", gw.Name),
				zap.Error(err))
			return ctrl.Result{}, err
		}
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
	log.Info("Gateway finalizer removed after Helm uninstall")
	return ctrl.Result{}, nil
}

// buildK8sGatewayInfraOverlay generates a Helm values YAML overlay from the standard
// Gateway spec.infrastructure labels and annotations (gateway-api v1.1+).
// The overlay sets commonLabels/commonAnnotations so they propagate to all gateway resources.
func buildK8sGatewayInfraOverlay(gw *gatewayv1.Gateway) (string, error) {
	if gw.Spec.Infrastructure == nil {
		return "", nil
	}
	infra := gw.Spec.Infrastructure
	if len(infra.Labels) == 0 && len(infra.Annotations) == 0 {
		return "", nil
	}
	overlay := map[string]interface{}{}
	if len(infra.Labels) > 0 {
		labels := make(map[string]string, len(infra.Labels))
		for k, v := range infra.Labels {
			labels[string(k)] = string(v)
		}
		overlay["commonLabels"] = labels
	}
	if len(infra.Annotations) > 0 {
		annotations := make(map[string]string, len(infra.Annotations))
		for k, v := range infra.Annotations {
			annotations[string(k)] = string(v)
		}
		overlay["commonAnnotations"] = annotations
	}
	out, err := yaml.Marshal(overlay)
	if err != nil {
		return "", fmt.Errorf("marshal K8s Gateway infra overlay: %w", err)
	}
	return string(out), nil
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

	// Build values overlay from spec.infrastructure labels/annotations (lowest priority).
	infraOverlay, err := buildK8sGatewayInfraOverlay(gw)
	if err != nil {
		return fmt.Errorf("build infra overlay: %w", err)
	}

	cmName := gw.Annotations[AnnK8sGatewayHelmValuesConfigMap]
	if cmName != "" {
		cmValues, err := configMapValuesYAML(ctx, r.Client, cmName, ns)
		if err != nil {
			return err
		}
		// Merge order: infra overlay (base) → ConfigMap (higher priority).
		// MergeValuesYAML delegates to deepMergeValues, which performs a recursive
		// per-key merge: ConfigMap entries can add or override individual keys inside
		// nested maps like commonLabels/commonAnnotations, but cannot remove keys that
		// the infra overlay already set. If you need a wholesale map-level replace
		// (e.g. discard all infra-set labels and use only the ConfigMap's map), add
		// "commonLabels" or "commonAnnotations" to probeMergeReplaceKeys in
		// internal/helm/client.go, or use an equivalent replace mechanism.
		if infraOverlay != "" {
			valuesYAML, err = helm.MergeValuesYAML(infraOverlay, cmValues)
			if err != nil {
				return fmt.Errorf("merge infra overlay with ConfigMap values: %w", err)
			}
		} else {
			valuesYAML = cmValues
		}
		log.Info("Merging Helm values: operator base file + infra overlay + ConfigMap overlay",
			zap.String("configMap", cmName),
			zap.String("valuesFile", valuesFile))
	} else {
		valuesYAML = infraOverlay
		log.Info("Using default Helm values file with infra overlay", zap.String("path", valuesFile))
	}

	if overlayYAML, overlayErr := applyListenerOverlayToValues(gw, valuesYAML); overlayErr != nil {
		return fmt.Errorf("apply gateway listener overlay: %w", overlayErr)
	} else if overlayYAML != valuesYAML {
		httpPort, httpsPort, hasHTTPS := listenerPortsFromGateway(gw)
		log.Info("Derived router ports from Gateway.spec.listeners",
			zap.Int32("httpPort", httpPort),
			zap.Int32("httpsPort", httpsPort),
			zap.Bool("httpsEnabled", hasHTTPS))
		valuesYAML = overlayYAML
	}

	if overlayYAML, overlayErr := applyInfrastructureOverlayToValues(gw, valuesYAML); overlayErr != nil {
		return fmt.Errorf("apply gateway infrastructure overlay: %w", overlayErr)
	} else if overlayYAML != valuesYAML {
		log.Info("Applied Gateway.spec.infrastructure overlay to gateway-runtime Service",
			zap.String("serviceType", serviceTypeFromGateway(gw)),
			zap.Int("annotationCount", len(infrastructureAnnotationsFromGateway(gw))),
			zap.Int("labelCount", len(infrastructureLabelsFromGateway(gw))))
		valuesYAML = overlayYAML
	}

	dockerUser, dockerPass, err := r.getDockerHubCredentials(ctx)
	if err != nil {
		return err
	}

	sig, err := helmInstallSignature(valuesYAML, valuesFile, r.Config, gw)
	if err != nil {
		return err
	}
	if gw.Annotations == nil {
		gw.Annotations = map[string]string{}
	}
	hashMatches := gw.Annotations[AnnK8sGatewayHelmValuesHash] == sig

	releaseDeployed, err := helmgateway.ReleaseDeployed(ctx, r.Config, gw.Name, ns)
	if err != nil {
		return fmt.Errorf("helm release status: %w", err)
	}
	resourcesOK, resDetail, err := gatewayHelmExpectedResourcesPresent(ctx, r.Client, gw.Name, ns)
	if err != nil {
		return err
	}

	skipHelm := hashMatches && releaseDeployed && resourcesOK
	if !skipHelm {
		if hashMatches {
			log.Info("Helm install/upgrade required; release or expected cluster resources missing or unhealthy",
				zap.String("valuesHash", sig),
				zap.Bool("releaseDeployed", releaseDeployed),
				zap.Bool("expectedResourcesPresent", resourcesOK),
				zap.String("resourceDetail", resDetail))
		}
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

	if err := registerGatewayInRegistry(ctx, r.Client, gw.Name, ns, apiSel, cpHost, helmCM, true); err != nil {
		return fmt.Errorf("register: %w", err)
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

// enqueueK8sGatewaysForHelmValuesConfigMap enqueues reconciliation for Gateways that reference
// the Helm values ConfigMap via gateway.api-platform.wso2.com/helm-values-configmap (same as
// APIGateway spec.configRef for inline gateway values).
func (r *K8sGatewayReconciler) enqueueK8sGatewaysForHelmValuesConfigMap(ctx context.Context, obj client.Object) []reconcile.Request {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return nil
	}
	logger := log.FromContext(ctx)
	gwList := &gatewayv1.GatewayList{}
	if err := r.List(ctx, gwList, client.InNamespace(cm.Namespace)); err != nil {
		logger.Error(err, "failed to list Gateways for ConfigMap event",
			"configMap", cm.Name,
			"namespace", cm.Namespace)
		return nil
	}
	var requests []reconcile.Request
	for i := range gwList.Items {
		gw := &gwList.Items[i]
		if gw.Annotations == nil {
			continue
		}
		if gw.Annotations[AnnK8sGatewayHelmValuesConfigMap] != cm.Name {
			continue
		}
		logger.Info("enqueue Gateway for Helm values ConfigMap change",
			"gateway", gw.Name,
			"namespace", gw.Namespace,
			"configMap", cm.Name)
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: gw.Namespace, Name: gw.Name},
		})
	}
	return requests
}

// SetupWithManager registers the Kubernetes Gateway controller.
func (r *K8sGatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := contctrl.Options{MaxConcurrentReconciles: r.Config.Reconciliation.MaxConcurrentReconciles}
	if opts.MaxConcurrentReconciles <= 0 {
		opts.MaxConcurrentReconciles = 1
	}
	// Match APIGateway: updates/deletes to the values ConfigMap must re-run Helm (values hash changes).
	configMapPred := predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool { return false },
		UpdateFunc: func(event.UpdateEvent) bool { return true },
		DeleteFunc: func(event.DeleteEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(opts).
		For(&gatewayv1.Gateway{}, builder.WithPredicates(k8sGatewayReconcilePredicate())).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueK8sGatewaysForHelmValuesConfigMap),
			builder.WithPredicates(configMapPred),
		).
		Complete(r)
}

func (r *K8sGatewayReconciler) getDockerHubCredentials(ctx context.Context) (string, string, error) {
	if r.Config.Gateway.RegistryCredentialsSecret == nil {
		return "", "", nil
	}
	secretRef := r.Config.Gateway.RegistryCredentialsSecret
	// Treat fully-empty ref as "not configured" (optional auth), but fail on partial refs.
	if secretRef.Name == "" && secretRef.Namespace == "" {
		return "", "", nil
	}
	if secretRef.Name == "" || secretRef.Namespace == "" {
		return "", "", fmt.Errorf(
			"gateway.registry_credentials_secret is set but name or namespace is empty (both are required)",
		)
	}
	usernameKey, passwordKey := secretRef.UsernameKey, secretRef.PasswordKey
	if usernameKey == "" {
		usernameKey = "username"
	}
	if passwordKey == "" {
		passwordKey = "password"
	}
	secret := &corev1.Secret{}
	key := client.ObjectKey{Name: secretRef.Name, Namespace: secretRef.Namespace}
	if err := r.Get(ctx, key, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return "", "", fmt.Errorf("registry credentials secret %s/%s not found", secretRef.Namespace, secretRef.Name)
		}
		return "", "", fmt.Errorf("get registry credentials secret %s/%s: %w", secretRef.Namespace, secretRef.Name, err)
	}
	userBytes, uok := secret.Data[usernameKey]
	passBytes, pok := secret.Data[passwordKey]
	if !uok || !pok {
		return "", "", fmt.Errorf(
			"registry credentials secret %s/%s: missing data keys %q and/or %q",
			secret.Namespace, secret.Name, usernameKey, passwordKey,
		)
	}
	if len(userBytes) == 0 || len(passBytes) == 0 {
		return "", "", fmt.Errorf(
			"registry credentials secret %s/%s: empty username or password under keys %q / %q",
			secret.Namespace, secret.Name, usernameKey, passwordKey,
		)
	}
	return string(userBytes), string(passBytes), nil
}

func isHelmReleaseNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, driver.ErrReleaseNotFound) || errors.Is(err, driver.ErrNoDeployedReleases) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "release: not found") ||
		strings.Contains(msg, "release not found") ||
		strings.Contains(msg, "no deployed releases")
}
