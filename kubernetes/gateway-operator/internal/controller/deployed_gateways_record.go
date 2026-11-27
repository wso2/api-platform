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
	"log/slog"
	"sort"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
)

// AnnDeployedToGateways records, on a RestApi or a resource handled by the generic reconciler,
// every gateway the operator has sent it to. The value is a JSON object mapping "namespace/name"
// to the gateway's kind (APIGateway or Gateway).
//
// The gateway registry lives only in memory, so after an operator restart it is empty until each
// gateway reconciles again. Deletion reads this record so a resource deleted in that window is
// still removed from its gateways instead of losing its finalizer while it stays deployed.
const AnnDeployedToGateways = "gateway.api-platform.wso2.com/deployed-to-gateways"

const (
	gatewayKindAPIGateway = "APIGateway"
	gatewayKindGatewayAPI = "Gateway"
)

// gatewayRecord maps a gateway key ("namespace/name") to its kind.
type gatewayRecord map[string]string

func gatewayKindOf(info *registry.GatewayInfo) string {
	if info.FromGatewayAPI {
		return gatewayKindGatewayAPI
	}
	return gatewayKindAPIGateway
}

// readGatewayRecord returns the gateways recorded on obj. Unreadable values and keys are skipped.
func readGatewayRecord(obj client.Object) gatewayRecord {
	out := gatewayRecord{}
	raw := strings.TrimSpace(obj.GetAnnotations()[AnnDeployedToGateways])
	if raw == "" {
		return out
	}
	var parsed map[string]string
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return out
	}
	for k, kind := range parsed {
		if key, ok := parseGatewayKey(k); ok {
			out[gatewayKeyString(key)] = kind
		}
	}
	return out
}

// recordGatewayDeployment adds the gateway to obj's deployment record. It runs before the deploy
// call, so a deploy that reaches the gateway is never left unrecorded; a recorded gateway that
// never received the resource answers the delete with 404, which counts as removed.
// obj is updated in place with the patched object.
func recordGatewayDeployment(ctx context.Context, c client.Client, obj client.Object, info *registry.GatewayInfo) error {
	rec := readGatewayRecord(obj)
	key := info.Namespace + "/" + info.Name
	kind := gatewayKindOf(info)
	if rec[key] == kind {
		return nil
	}
	rec[key] = kind
	// json.Marshal sorts map keys, so the value is stable.
	raw, err := json.Marshal(map[string]string(rec))
	if err != nil {
		return fmt.Errorf("encode deployed gateways: %w", err)
	}
	base, ok := obj.DeepCopyObject().(client.Object)
	if !ok {
		return fmt.Errorf("copy %T", obj)
	}
	ann := obj.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
	}
	ann[AnnDeployedToGateways] = string(raw)
	obj.SetAnnotations(ann)
	if err := c.Patch(ctx, obj, client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{})); err != nil {
		return fmt.Errorf("record gateway %s in %s annotation: %w", key, AnnDeployedToGateways, err)
	}
	return nil
}

// payloadAnnotations returns the annotations to forward to the gateway: the CR's own, without the
// operator's deployment record.
func payloadAnnotations(ann map[string]string) map[string]string {
	if _, ok := ann[AnnDeployedToGateways]; !ok {
		return ann
	}
	out := make(map[string]string, len(ann)-1)
	for k, v := range ann {
		if k != AnnDeployedToGateways {
			out[k] = v
		}
	}
	return out
}

// gatewayCleanupTarget is a gateway a deleted resource may still be deployed to.
type gatewayCleanupTarget struct {
	key  client.ObjectKey
	kind string // APIGateway, Gateway, or "" when unknown
}

// deletionGatewayTargets lists the recorded gateways plus the registered gateways that select the
// resource today (which covers resources deployed before the record existed), sorted by key.
func deletionGatewayTargets(rec gatewayRecord, matched []*registry.GatewayInfo) []gatewayCleanupTarget {
	byKey := map[string]gatewayCleanupTarget{}
	for k, kind := range rec {
		key, _ := parseGatewayKey(k)
		byKey[k] = gatewayCleanupTarget{key: key, kind: kind}
	}
	for _, info := range matched {
		k := info.Namespace + "/" + info.Name
		if _, ok := byKey[k]; !ok {
			byKey[k] = gatewayCleanupTarget{key: client.ObjectKey{Namespace: info.Namespace, Name: info.Name}, kind: gatewayKindOf(info)}
		}
	}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]gatewayCleanupTarget, 0, len(keys))
	for _, k := range keys {
		out = append(out, byKey[k])
	}
	return out
}

// gatewayObjectState is what the cluster says about a target's gateway object.
type gatewayObjectState int

const (
	gatewayObjectGone gatewayObjectState = iota
	gatewayObjectDeleting
	gatewayObjectAwaitsRegistration // exists and will register once its runtime is up
	gatewayObjectNeverRegisters     // exists, but this operator does not manage it
)

// lookupGatewayObject finds the target's APIGateway or Gateway API Gateway. A missing object, or a
// Gateway API that is not installed, means the gateway is gone.
func lookupGatewayObject(ctx context.Context, c client.Client, cfg *config.OperatorConfig, t gatewayCleanupTarget) (gatewayObjectState, error) {
	if t.kind != gatewayKindGatewayAPI {
		apigw := &apiv1.APIGateway{}
		err := c.Get(ctx, t.key, apigw)
		switch {
		case err == nil:
			if !apigw.DeletionTimestamp.IsZero() {
				return gatewayObjectDeleting, nil
			}
			return gatewayObjectAwaitsRegistration, nil
		case !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err):
			return 0, err
		case t.kind == gatewayKindAPIGateway:
			return gatewayObjectGone, nil
		}
	}
	gw := &gatewayv1.Gateway{}
	if err := c.Get(ctx, t.key, gw); err != nil {
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return gatewayObjectGone, nil
		}
		return 0, err
	}
	if !gw.DeletionTimestamp.IsZero() {
		return gatewayObjectDeleting, nil
	}
	if cfg != nil && cfg.ManagedGatewayClass(string(gw.Spec.GatewayClassName)) {
		return gatewayObjectAwaitsRegistration, nil
	}
	return gatewayObjectNeverRegisters, nil
}

// cleanupGatewayTargets removes a deleted resource from every target with removeFn. It reports
// waiting=true when a target's gateway still exists but has not registered with the operator yet
// (for example right after an operator restart); the caller keeps its finalizer and requeues.
// Removal errors are returned so the finalizer stays and the delete is retried; removeFn must
// treat "not found" on the gateway as success.
func cleanupGatewayTargets(
	ctx context.Context,
	c client.Client,
	cfg *config.OperatorConfig,
	log *slog.Logger,
	targets []gatewayCleanupTarget,
	removeFn func(info *registry.GatewayInfo) error,
) (waiting bool, err error) {
	var errs []error
	for _, t := range targets {
		tlog := log.With(slog.String("gateway", gatewayKeyString(t.key)))
		state, lookupErr := lookupGatewayObject(ctx, c, cfg, t)
		if lookupErr != nil {
			errs = append(errs, fmt.Errorf("%s: look up gateway: %w", gatewayKeyString(t.key), lookupErr))
			continue
		}
		info, registered := registry.GetGatewayRegistry().Get(t.key.Namespace, t.key.Name)
		switch state {
		case gatewayObjectGone:
			// A deleted gateway's runtime is uninstalled with it, so nothing is left to remove.
			tlog.Info("gateway no longer exists; nothing to remove")
			continue
		case gatewayObjectDeleting:
			// Its runtime is being uninstalled: try once, but don't hold the resource on it.
			if registered {
				if rmErr := removeFn(info); rmErr != nil {
					tlog.Info("skip removal from gateway being deleted", slog.Any("error", rmErr))
				}
			}
			continue
		}
		if registered {
			if rmErr := removeFn(info); rmErr != nil {
				tlog.Error("failed to remove resource from gateway", slog.Any("error", rmErr))
				errs = append(errs, fmt.Errorf("%s: %w", gatewayKeyString(t.key), rmErr))
				continue
			}
			tlog.Info("resource removed from gateway")
			continue
		}
		if state == gatewayObjectAwaitsRegistration {
			tlog.Info("gateway not registered yet; waiting to remove resource")
			waiting = true
		}
	}
	return waiting, errors.Join(errs...)
}
