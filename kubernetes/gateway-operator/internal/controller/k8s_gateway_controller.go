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

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"log/slog"
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

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
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
	Logger *slog.Logger
}

// NewK8sGatewayReconciler creates a reconciler for gateway.networking.k8s.io Gateway.
func NewK8sGatewayReconciler(cl client.Client, scheme *runtime.Scheme, cfg *config.OperatorConfig, logger *slog.Logger) *K8sGatewayReconciler {
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
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

func (r *K8sGatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	gw := &gatewayv1.Gateway{}
	if err := r.Get(ctx, req.NamespacedName, gw); err != nil {
		if apierrors.IsNotFound(err) {
			if r.Logger != nil {
				r.Logger.Debug("Gateway not found; likely deleted",
					slog.String("controller", "K8sGateway"),
					slog.String("namespace", req.Namespace),
					slog.String("name", req.Name))
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	log := r.Logger.With(slog.String("controller", "K8sGateway"), slog.String("namespace", req.Namespace), slog.String("name", req.Name))
	log.Info("reconcile Gateway",
		slog.Int64("generation", gw.Generation),
		slog.String("resourceVersion", gw.ResourceVersion),
		slog.String("gatewayClass", string(gw.Spec.GatewayClassName)))

	if !gw.DeletionTimestamp.IsZero() {
		return r.reconcileDeletion(ctx, gw, log)
	}

	if !r.Config.ManagedGatewayClass(string(gw.Spec.GatewayClassName)) {
		log.Debug("skip Gateway: class not managed by operator",
			slog.String("gatewayClass", string(gw.Spec.GatewayClassName)))
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
		log.Error("sync gateway", slog.Any("error", err))
		_ = r.patchGatewayStatus(ctx, gw, nil, nil, metav1.Condition{
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
		slog.String("gatewayClass", string(gw.Spec.GatewayClassName)))
	return ctrl.Result{}, nil
}

func (r *K8sGatewayReconciler) reconcileDeletion(ctx context.Context, gw *gatewayv1.Gateway, log *slog.Logger) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(gw, k8sGatewayFinalizer) {
		return ctrl.Result{}, nil
	}

	log.Info("reconcile Gateway deletion",
		slog.String("gatewayClass", string(gw.Spec.GatewayClassName)))

	ns := gw.Namespace
	if ns == "" {
		ns = "default"
	}

	registry.GetGatewayRegistry().Unregister(ns, gw.Name)
	log.Info("unregistered Gateway from operator registry",
		slog.String("namespace", ns),
		slog.String("name", gw.Name))

	log.Info("attempting Helm uninstall for Gateway deletion",
		slog.String("namespace", ns),
		slog.String("name", gw.Name))
	if err := helmgateway.Uninstall(ctx, log, r.Config, gw.Name, ns); err != nil {
		if isHelmReleaseNotFoundError(err) {
			log.Info("Helm release not found during Gateway deletion; continuing finalizer removal",
				slog.String("namespace", ns),
				slog.String("name", gw.Name),
				slog.Any("error", err))
		} else {
			log.Error("helm uninstall failed; will retry",
				slog.String("namespace", ns),
				slog.String("name", gw.Name),
				slog.Any("error", err))
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

func (r *K8sGatewayReconciler) syncGateway(ctx context.Context, gw *gatewayv1.Gateway, log *slog.Logger) error {
	ns := gw.Namespace
	if ns == "" {
		ns = "default"
	}

	// Validate parametersRef first - reject Gateway if invalid
	paramsValid, paramsReason, err := validateParametersRef(ctx, r.Client, gw)
	if err != nil {
		return fmt.Errorf("validate parametersRef: %w", err)
	}
	if !paramsValid {
		log.Info("Gateway has invalid parametersRef; setting Accepted=False",
			slog.String("reason", paramsReason))
		return r.patchGatewayStatus(ctx, gw, nil, nil,
			metav1.Condition{
				Type:               string(gatewayv1.GatewayConditionAccepted),
				Status:             metav1.ConditionFalse,
				Reason:             paramsReason,
				Message:            "Gateway infrastructure parametersRef is invalid or does not exist",
				ObservedGeneration: gw.Generation,
				LastTransitionTime: metav1.Now(),
			},
			metav1.Condition{
				Type:               string(gatewayv1.GatewayConditionProgrammed),
				Status:             metav1.ConditionFalse,
				Reason:             string(gatewayv1.GatewayReasonInvalid),
				Message:            "Gateway has invalid parametersRef",
				ObservedGeneration: gw.Generation,
				LastTransitionTime: metav1.Now(),
			},
		)
	}

	// Evaluate listener protocols - reject gateway if no listeners are supported
	listenerStatuses, hasAnyAccepted, err := evaluateListeners(ctx, r.Client, gw)
	if err != nil {
		return fmt.Errorf("evaluate listeners: %w", err)
	}

	if !hasAnyAccepted {
		// No supported listeners at all - Gateway must not be Accepted or Programmed
		log.Info("Gateway has no accepted listeners; setting Accepted=False and Programmed=False",
			slog.String("reason", string(gatewayv1.GatewayReasonListenersNotValid)))
		return r.patchGatewayStatus(ctx, gw, listenerStatuses, nil,
			metav1.Condition{
				Type:               string(gatewayv1.GatewayConditionAccepted),
				Status:             metav1.ConditionFalse,
				Reason:             string(gatewayv1.GatewayReasonListenersNotValid),
				Message:            "No listeners with supported protocols (HTTP, HTTPS)",
				ObservedGeneration: gw.Generation,
				LastTransitionTime: metav1.Now(),
			},
			metav1.Condition{
				Type:               string(gatewayv1.GatewayConditionProgrammed),
				Status:             metav1.ConditionFalse,
				Reason:             string(gatewayv1.GatewayReasonInvalid),
				Message:            "Gateway has no accepted listeners",
				ObservedGeneration: gw.Generation,
				LastTransitionTime: metav1.Now(),
			},
		)
	}

	// Patch Accepted status early (before Helm) to satisfy observedGeneration requirements
	// This ensures conformance tests see the status update quickly
	acceptedReason := gatewayv1.GatewayReasonAccepted
	acceptedMessage := "Gateway accepted"
	if !hasAllListenersAccepted(listenerStatuses) {
		acceptedReason = gatewayv1.GatewayReasonListenersNotValid
		acceptedMessage = "Gateway accepted but some listeners are invalid"
	}

	earlyConds := []metav1.Condition{
		{
			Type:               string(gatewayv1.GatewayConditionAccepted),
			Status:             metav1.ConditionTrue,
			Reason:             string(acceptedReason),
			Message:            acceptedMessage,
			ObservedGeneration: gw.Generation,
			LastTransitionTime: metav1.Now(),
		},
	}
	// The Programmed condition is otherwise only patched after the Helm install, which runs
	// with Wait (up to 300s). Until then the condition still carries the previous (or the CRD
	// default, observedGeneration 0) generation, and conformance helpers that require every
	// condition to be at metadata.generation (e.g. GatewayStatusMustHaveListeners in
	// GatewayInvalidTLSConfiguration) time out — a Gateway whose release never becomes ready
	// (nonexistent TLS secret) never reaches the later Programmed patch. Refresh it here as
	// Unknown/Pending when stale; an already-current condition (steady-state Programmed=True)
	// is left untouched so existing Gateways never flap.
	if gatewayProgrammedConditionStale(gw) {
		earlyConds = append(earlyConds, metav1.Condition{
			Type:               string(gatewayv1.GatewayConditionProgrammed),
			Status:             metav1.ConditionUnknown,
			Reason:             string(gatewayv1.GatewayReasonPending),
			Message:            "Gateway dataplane deployment in progress",
			ObservedGeneration: gw.Generation,
			LastTransitionTime: metav1.Now(),
		})
	}

	if err := r.patchGatewayStatus(ctx, gw, listenerStatuses, nil, earlyConds...); err != nil {
		return fmt.Errorf("patch early Accepted status: %w", err)
	}

	// Refresh Gateway object after status patch
	if err := r.Get(ctx, client.ObjectKeyFromObject(gw), gw); err != nil {
		return err
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
			slog.String("configMap", cmName),
			slog.String("valuesFile", valuesFile))
	} else {
		valuesYAML = infraOverlay
		log.Info("Using default Helm values file with infra overlay", slog.String("path", valuesFile))
	}

	if overlayYAML, overlayErr := applyListenerOverlayToValues(gw, valuesYAML); overlayErr != nil {
		return fmt.Errorf("apply gateway listener overlay: %w", overlayErr)
	} else if overlayYAML != valuesYAML {
		httpPort, httpsPort, hasHTTPS := listenerPortsFromGateway(gw)
		log.Info("Derived router ports from Gateway.spec.listeners",
			slog.Int("httpPort", int(httpPort)),
			slog.Int("httpsPort", int(httpsPort)),
			slog.Bool("httpsEnabled", hasHTTPS))
		valuesYAML = overlayYAML
	}

	if overlayYAML, overlayErr := applyListenerTLSOverlayToValues(ctx, r.Client, gw, valuesYAML); overlayErr != nil {
		return fmt.Errorf("apply gateway listener TLS overlay: %w", overlayErr)
	} else if overlayYAML != valuesYAML {
		secretName, _ := listenerTLSSecretFromGateway(gw)
		log.Info("Sourcing gateway-runtime HTTPS listener certificate from Gateway listener certificateRef",
			slog.String("secret", secretName))
		valuesYAML = overlayYAML
	}

	if overlayYAML, overlayErr := applyInfrastructureOverlayToValues(gw, valuesYAML); overlayErr != nil {
		return fmt.Errorf("apply gateway infrastructure overlay: %w", overlayErr)
	} else if overlayYAML != valuesYAML {
		log.Info("Applied Gateway.spec.infrastructure overlay to gateway-runtime Service",
			slog.String("serviceType", serviceTypeFromGateway(gw)),
			slog.Int("annotationCount", len(infrastructureAnnotationsFromGateway(gw))),
			slog.Int("labelCount", len(infrastructureLabelsFromGateway(gw))))
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
				slog.String("valuesHash", sig),
				slog.Bool("releaseDeployed", releaseDeployed),
				slog.Bool("expectedResourcesPresent", resourcesOK),
				slog.String("resourceDetail", resDetail))
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
		log.Info("Helm install/upgrade applied", slog.String("valuesHash", sig))
	} else {
		log.Info("Skipping Helm install/upgrade; values signature unchanged", slog.String("valuesHash", sig))
	}

	cpHost := gw.Annotations[AnnK8sGatewayControlPlaneHost]
	helmCM := cmName

	ready, msg, err := evaluateGatewayDeploymentsReady(ctx, r.Client, gw.Name, ns)
	if err != nil {
		return err
	}
	if !ready {
		_ = r.patchGatewayStatus(ctx, gw, nil, nil, metav1.Condition{
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

	// Refresh listener statuses (including AttachedRoutes) before final Programmed patch.
	listenerStatuses, _, err = evaluateListeners(ctx, r.Client, gw)
	if err != nil {
		return fmt.Errorf("refresh listener statuses: %w", err)
	}

	runtimeSvc, err := discoverGatewayRuntimeService(ctx, r.Client, gw.Name, ns)
	if err != nil {
		return fmt.Errorf("discover gateway runtime service: %w", err)
	}
	var nodePortAddrs []gatewayv1.GatewayStatusAddress
	if runtimeSvc != nil && runtimeSvc.Spec.Type == corev1.ServiceTypeNodePort {
		nodePortAddrs, err = r.resolveNodePortGatewayAddresses(ctx)
		if err != nil {
			return fmt.Errorf("resolve NodePort gateway addresses: %w", err)
		}
	}
	addrs := resolveGatewayAddressesFromService(runtimeSvc, nodePortAddrs)
	if runtimeSvc.Spec.Type == corev1.ServiceTypeLoadBalancer && len(addrs) == 0 {
		pendingMsg := "Waiting for LoadBalancer address to be assigned to gateway runtime Service"
		_ = r.patchGatewayStatus(ctx, gw, listenerStatuses, nil, metav1.Condition{
			Type:               string(gatewayv1.GatewayConditionProgrammed),
			Status:             metav1.ConditionFalse,
			Reason:             string(gatewayv1.GatewayReasonPending),
			Message:            pendingMsg,
			ObservedGeneration: gw.Generation,
			LastTransitionTime: metav1.Now(),
		})
		return fmt.Errorf("pending: %s", pendingMsg)
	}

	return r.patchGatewayStatus(ctx, gw, listenerStatuses, &addrs,
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

// resolveNodePortGatewayAddresses determines the Gateway status addresses for a NodePort
// runtime Service. When NodePortAddressOverride is configured (intended for local clusters
// reachable only via loopback) it is published verbatim; otherwise the operator lists the
// cluster Nodes and advertises their real addresses (ExternalIP, falling back to InternalIP).
func (r *K8sGatewayReconciler) resolveNodePortGatewayAddresses(ctx context.Context) ([]gatewayv1.GatewayStatusAddress, error) {
	if r.Config != nil {
		if override := strings.TrimSpace(r.Config.GatewayAPI.NodePortAddressOverride); override != "" {
			return []gatewayv1.GatewayStatusAddress{gatewayIPAddress(override)}, nil
		}
	}
	var nodes corev1.NodeList
	if err := r.List(ctx, &nodes); err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	return nodePortGatewayAddressesFromNodes(nodes.Items), nil
}

// gatewayProgrammedConditionStale reports whether the Gateway's Programmed condition is
// missing or carries an observedGeneration older than the current spec generation (e.g. the
// CRD default condition on a fresh Gateway, or after a spec update before re-assessment).
func gatewayProgrammedConditionStale(gw *gatewayv1.Gateway) bool {
	cond := meta.FindStatusCondition(gw.Status.Conditions, string(gatewayv1.GatewayConditionProgrammed))
	return cond == nil || cond.ObservedGeneration != gw.Generation
}

func (r *K8sGatewayReconciler) patchGatewayStatus(ctx context.Context, gw *gatewayv1.Gateway, listenerStatuses []gatewayv1.ListenerStatus, addresses *[]gatewayv1.GatewayStatusAddress, conds ...metav1.Condition) error {
	latest := &gatewayv1.Gateway{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(gw), latest); err != nil {
		return err
	}
	base := latest.DeepCopy()
	for _, cond := range conds {
		meta.SetStatusCondition(&latest.Status.Conditions, cond)
	}
	if listenerStatuses != nil {
		latest.Status.Listeners = listenerStatuses
	}
	if addresses != nil {
		latest.Status.Addresses = *addresses
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

// enqueueK8sGatewaysForHTTPRoute enqueues parent Gateways when an HTTPRoute changes so
// listener AttachedRoutes counts stay in sync with route attachment status.
func (r *K8sGatewayReconciler) enqueueK8sGatewaysForHTTPRoute(ctx context.Context, obj client.Object) []reconcile.Request {
	route, ok := obj.(*gatewayv1.HTTPRoute)
	if !ok {
		return nil
	}

	targets := parentGatewayRefs(route)
	if len(targets) == 0 {
		return nil
	}

	logger := log.FromContext(ctx)
	seen := make(map[types.NamespacedName]struct{}, len(targets))
	var requests []reconcile.Request
	for _, target := range targets {
		if _, dup := seen[target.key]; dup {
			continue
		}
		gw := &gatewayv1.Gateway{}
		if err := r.Get(ctx, target.key, gw); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			logger.Error(err, "failed to get parent Gateway for HTTPRoute event",
				"httproute", route.Name,
				"httprouteNamespace", route.Namespace,
				"gateway", target.key.Name,
				"gatewayNamespace", target.key.Namespace)
			continue
		}
		if !r.Config.ManagedGatewayClass(string(gw.Spec.GatewayClassName)) {
			continue
		}
		seen[target.key] = struct{}{}
		requests = append(requests, reconcile.Request{NamespacedName: target.key})
	}
	return requests
}

// enqueueK8sGatewaysForRuntimeService enqueues Gateways when a gateway-runtime Service
// changes (e.g. LoadBalancer ingress assigned).
func (r *K8sGatewayReconciler) enqueueK8sGatewaysForRuntimeService(ctx context.Context, obj client.Object) []reconcile.Request {
	svc, ok := obj.(*corev1.Service)
	if !ok || svc.Labels == nil {
		return nil
	}
	if svc.Labels["app.kubernetes.io/component"] != "gateway-runtime" {
		return nil
	}
	releaseName := svc.Labels["app.kubernetes.io/instance"]
	if releaseName == "" {
		return nil
	}

	logger := log.FromContext(ctx)
	gwList := &gatewayv1.GatewayList{}
	if err := r.List(ctx, gwList, client.InNamespace(svc.Namespace)); err != nil {
		logger.Error(err, "failed to list Gateways for gateway-runtime Service event",
			"service", svc.Name,
			"namespace", svc.Namespace)
		return nil
	}

	var requests []reconcile.Request
	for i := range gwList.Items {
		gw := &gwList.Items[i]
		if !r.Config.ManagedGatewayClass(string(gw.Spec.GatewayClassName)) {
			continue
		}
		if helm.GetReleaseName(gw.Name) != releaseName {
			continue
		}
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: gw.Namespace, Name: gw.Name},
		})
	}
	return requests
}

// enqueueK8sGatewaysForListenerTLSSecret enqueues Gateways whose listener certificateRefs
// reference the changed Secret. The TLS overlay only mounts a Secret that exists and is a
// valid kubernetes.io/tls pair, so a Secret that is created (or fixed) after the Gateway
// must re-run reconcile for the overlay to be applied.
func (r *K8sGatewayReconciler) enqueueK8sGatewaysForListenerTLSSecret(ctx context.Context, obj client.Object) []reconcile.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil
	}

	logger := log.FromContext(ctx)
	gwList := &gatewayv1.GatewayList{}
	if err := r.List(ctx, gwList, client.InNamespace(secret.Namespace)); err != nil {
		logger.Error(err, "failed to list Gateways for Secret event",
			"secret", secret.Name,
			"namespace", secret.Namespace)
		return nil
	}

	var requests []reconcile.Request
	for i := range gwList.Items {
		gw := &gwList.Items[i]
		if !r.Config.ManagedGatewayClass(string(gw.Spec.GatewayClassName)) {
			continue
		}
		if !gatewayListenersReferenceTLSSecret(gw, secret.Namespace, secret.Name) {
			continue
		}
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
		CreateFunc:  func(event.CreateEvent) bool { return false },
		UpdateFunc:  func(event.UpdateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return true },
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
		Watches(
			&gatewayv1.HTTPRoute{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueK8sGatewaysForHTTPRoute),
		).
		Watches(
			&corev1.Service{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueK8sGatewaysForRuntimeService),
			builder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
				svc, ok := obj.(*corev1.Service)
				return ok && svc.Labels != nil && svc.Labels["app.kubernetes.io/component"] == "gateway-runtime"
			})),
		).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueK8sGatewaysForListenerTLSSecret),
			builder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
				secret, ok := obj.(*corev1.Secret)
				return ok && secret.Type == corev1.SecretTypeTLS
			})),
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
