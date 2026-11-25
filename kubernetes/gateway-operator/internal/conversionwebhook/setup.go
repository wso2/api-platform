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

// Package conversionwebhook serves the CRD conversion webhook for the operator's
// multi-version CRDs whose versions differ (APIGateway v1 <-> v2alpha1), and manages
// what the webhook needs: a self-signed CA and serving certificate in a Secret, and
// the CRD's spec.conversion pointing at the operator's Service.
package conversionwebhook

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/fields"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/conversion"
)

//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;patch,resourceNames=apigateways.gateway.api-platform.wso2.com
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;create;update

// APIGatewayCRDName is the CRD whose versions the webhook converts.
const APIGatewayCRDName = "apigateways.gateway.api-platform.wso2.com"

// Defaults for Options.
const (
	DefaultPort            = 9443
	DefaultMaxRequestBytes = 64 << 20 // a LIST conversion carries every object in one request
)

// Options configures the conversion webhook.
type Options struct {
	// Namespace is the operator's namespace, where the webhook Service and the
	// certificate Secret live.
	Namespace string
	// ServiceName is the Service that routes port 443 to Port on the operator pods.
	ServiceName string
	// SecretName is the Secret holding the CA and serving certificate.
	SecretName string
	// Port is the container port the webhook listens on.
	Port int
	// MaxRequestBytes caps a ConversionReview request body.
	MaxRequestBytes int64
	// PQCKeyExchange puts the hybrid X25519MLKEM768 key exchange first.
	PQCKeyExchange bool
	// EnableHTTP2 allows HTTP/2.
	EnableHTTP2 bool
	// Logger receives the webhook's logs.
	Logger *slog.Logger
}

func (o Options) validate() error {
	var errs []error
	if o.Namespace == "" {
		errs = append(errs, errors.New("namespace is required (set POD_NAMESPACE)"))
	}
	if o.ServiceName == "" {
		errs = append(errs, errors.New("service name is required"))
	}
	if o.SecretName == "" {
		errs = append(errs, errors.New("secret name is required"))
	}
	if o.Port <= 0 || o.Port > 65535 {
		errs = append(errs, fmt.Errorf("port %d is out of range", o.Port))
	}
	if o.MaxRequestBytes <= 0 {
		errs = append(errs, errors.New("max request bytes must be positive"))
	}
	return errors.Join(errs...)
}

// certificate lifetimes: a 10-year CA and a 1-year serving certificate re-issued 90
// days before it expires. When the CA is replaced (it nears expiry, or the Secret was
// deleted), the CRD trusts the old one too for 15 minutes, far longer than the loader
// takes to move every replica to the new certificate.
var defaultValidity = validity{
	ca:          10 * 365 * 24 * time.Hour,
	serving:     365 * 24 * time.Hour,
	renewBefore: 90 * 24 * time.Hour,
	caOverlap:   15 * time.Minute,
}

// CacheByObject returns the manager cache settings the webhook needs: the CRD
// informer selects only the converted CRD by name, which also lets RBAC restrict the
// operator to that CRD with resourceNames.
func CacheByObject() map[client.Object]cache.ByObject {
	return map[client.Object]cache.ByObject{
		&apiextensionsv1.CustomResourceDefinition{}: {
			Field: fields.OneTermEqualSelector("metadata.name", APIGatewayCRDName),
		},
	}
}

// Setup adds the webhook server, the certificate loader and rotator, and the CRD
// reconciler to the manager, and a readiness check that fails until the webhook can
// serve. The manager's scheme must include apiextensions/v1 and every converted
// version, and its cache must use CacheByObject.
func Setup(mgr ctrl.Manager, opts Options) error {
	if err := opts.validate(); err != nil {
		return fmt.Errorf("conversion webhook: %w", err)
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With(slog.String("component", "conversion-webhook"))

	store := &certStore{}
	secrets := &secretManager{
		reader:    mgr.GetAPIReader(),
		writer:    mgr.GetClient(),
		namespace: opts.Namespace,
		name:      opts.SecretName,
		dnsNames:  serviceDNSNames(opts.ServiceName, opts.Namespace),
		validity:  defaultValidity,
		now:       time.Now,
	}
	conv := &crdConversion{
		reader:    mgr.GetAPIReader(),
		writer:    mgr.GetClient(),
		crdName:   APIGatewayCRDName,
		namespace: opts.Namespace,
		service:   opts.ServiceName,
		now:       time.Now,
		logger:    logger,
	}
	secrets.currentCA = conv.caBundle
	state := servingState{secrets: secrets, conversion: conv, store: store}
	srv := &server{
		port:            opts.Port,
		handler:         conversion.NewWebhookHandler(mgr.GetScheme(), mgr.GetConverterRegistry()),
		maxRequestBytes: opts.MaxRequestBytes,
		tls:             tlsConfig(store, opts.PQCKeyExchange, opts.EnableHTTP2, logger),
		logger:          logger,
	}
	// Everything runs on every replica (NeedLeaderElection false), so a new pod can
	// serve, and turn ready, while an older operator version still holds the leader
	// lease during a rolling upgrade.
	for _, r := range []manager.Runnable{
		srv,
		&loader{servingState: state, interval: time.Minute, logger: logger},
		&rotator{servingState: state, interval: time.Hour, retry: 10 * time.Second, logger: logger},
	} {
		if err := mgr.Add(r); err != nil {
			return fmt.Errorf("conversion webhook: add runnable: %w", err)
		}
	}
	if err := (&crdReconciler{
		conversion: conv,
		secrets:    secrets,
		crdName:    APIGatewayCRDName,
		resync:     10 * time.Minute,
	}).setupWithManager(mgr); err != nil {
		return fmt.Errorf("conversion webhook: CRD reconciler: %w", err)
	}
	return mgr.AddReadyzCheck("conversion-webhook", func(*http.Request) error {
		if !srv.listening.Load() {
			return errors.New("conversion webhook is not listening")
		}
		if !store.ready() {
			return errors.New("conversion webhook certificate is not loaded")
		}
		return nil
	})
}
