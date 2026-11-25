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

package conversionwebhook

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// secretManager reads and writes the webhook's certificate Secret.
type secretManager struct {
	reader    client.Reader // uncached: the operator's namespace may be outside the cache
	writer    client.Client
	namespace string
	name      string
	dnsNames  []string
	validity  validity
	now       func() time.Time
	// currentCA returns the CA bundle the CRD trusts today. When the Secret is gone
	// and a new CA is created, it supplies the old CA for the overlap. May be nil.
	currentCA func(ctx context.Context) []byte
}

func (m *secretManager) key() types.NamespacedName {
	return types.NamespacedName{Namespace: m.namespace, Name: m.name}
}

// read returns the current bundle, or found=false when the Secret does not exist.
func (m *secretManager) read(ctx context.Context) (certBundle, *corev1.Secret, bool, error) {
	secret := &corev1.Secret{}
	if err := m.reader.Get(ctx, m.key(), secret); err != nil {
		if apierrors.IsNotFound(err) {
			return certBundle{}, nil, false, nil
		}
		return certBundle{}, nil, false, fmt.Errorf("get conversion webhook secret %s: %w", m.key(), err)
	}
	return bundleFromSecret(secret.Data, secret.Annotations), secret, true, nil
}

// ensure creates the Secret, or renews the certificates in it, and returns the bundle
// to serve. Every replica calls it, so writes race; the API server resolves them. A
// replica that loses the create re-reads and serves the winner's Secret. A lost
// update (resourceVersion conflict) is an error the caller retries; the retry reads
// the winner's certificates, which no longer need renewing.
func (m *secretManager) ensure(ctx context.Context) (certBundle, error) {
	bundle, err := m.ensureOnce(ctx)
	if apierrors.IsAlreadyExists(err) {
		return m.ensureOnce(ctx)
	}
	return bundle, err
}

func (m *secretManager) ensureOnce(ctx context.Context) (certBundle, error) {
	bundle, secret, found, err := m.read(ctx)
	if err != nil {
		return certBundle{}, err
	}
	now := m.now()
	plan := renewAll
	if found {
		plan = planRenewal(bundle, m.dnsNames, now, m.validity.renewBefore)
	}
	if plan == renewNothing {
		return bundle, nil
	}
	renewed, err := renew(bundle, plan, m.dnsNames, now, m.validity)
	if err != nil {
		return certBundle{}, err
	}
	if plan == renewAll {
		// A new CA: keep trusting the old one for a while, so replicas still serving a
		// certificate it signed are not rejected while they load the new one. The old
		// CA comes from the Secret, or from the CRD when the Secret was deleted.
		old := bundle.caCert
		if !found && m.currentCA != nil {
			old = m.currentCA(ctx)
		}
		renewed.previousCA = previousCAs(old, renewed.caCert, now)
		renewed.previousUntil = time.Time{}
		if len(renewed.previousCA) > 0 {
			renewed.previousUntil = now.Add(m.validity.caOverlap)
		}
	}
	if !found {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      m.name,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "gateway-operator",
					"app.kubernetes.io/component":  "conversion-webhook",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: renewed.data(),
		}
		secret.Annotations = renewed.annotations()
		if err := m.writer.Create(ctx, secret); err != nil {
			// %w keeps IsAlreadyExists working for ensure's retry.
			return certBundle{}, fmt.Errorf("create conversion webhook secret %s: %w", m.key(), err)
		}
		return renewed, nil
	}
	secret = secret.DeepCopy()
	secret.Data = renewed.data()
	if secret.Annotations == nil {
		secret.Annotations = map[string]string{}
	}
	delete(secret.Annotations, PreviousCAUntilAnnotation)
	for k, v := range renewed.annotations() {
		secret.Annotations[k] = v
	}
	if err := m.writer.Update(ctx, secret); err != nil {
		return certBundle{}, fmt.Errorf("update conversion webhook secret %s: %w", m.key(), err)
	}
	return renewed, nil
}

// jittered returns d plus a random amount up to a tenth of d, so replicas and restarts
// do not hit the API server in lockstep.
func jittered(d time.Duration) time.Duration {
	if tenth := d / 10; tenth > 0 {
		return d + rand.N(tenth)
	}
	return d
}

// server-side state every serving loop shares: the Secret, the CRD and the in-memory
// certificate.
type servingState struct {
	secrets    *secretManager
	conversion *crdConversion
	store      *certStore
}

// serve makes the CRD trust b, then serves b's certificate. The order matters: a
// certificate from a new CA is only served once the API server trusts that CA. If the
// CRD cannot be updated, the certificate already in memory keeps being served.
func (s *servingState) serve(ctx context.Context, b certBundle) error {
	if s.conversion != nil {
		if err := s.conversion.trust(ctx, b); err != nil {
			return err
		}
	}
	if s.store.serving(b) {
		return nil
	}
	return s.store.set(b)
}

// rotator keeps the certificate Secret valid. It runs on every replica, not only the
// leader: during a rolling upgrade the old operator version keeps the leader lease,
// and a new pod that waited for leadership would never get a certificate, never turn
// ready, and so never let the rollout replace the old pods. The writes are safe to
// race (see secretManager.ensure).
type rotator struct {
	servingState
	interval time.Duration
	retry    time.Duration
	logger   *slog.Logger
}

func (r *rotator) NeedLeaderElection() bool { return false }

func (r *rotator) Start(ctx context.Context) error {
	for {
		wait := jittered(r.interval)
		bundle, err := r.secrets.ensure(ctx)
		if err != nil {
			r.logger.Warn("Cannot ensure the conversion webhook certificate; retrying", slog.Any("error", err))
			wait = jittered(r.retry)
		} else if err := r.serve(ctx, bundle); err != nil {
			r.logger.Warn("Cannot serve the conversion webhook certificate; retrying", slog.Any("error", err))
			wait = jittered(r.retry)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(wait):
		}
	}
}

// loader checks the certificate Secret on every replica about once a minute. It serves
// a certificate another replica created or renewed, and recreates a Secret that was
// deleted (the replicas keep serving the certificate they hold meanwhile).
type loader struct {
	servingState
	interval time.Duration
	logger   *slog.Logger
}

func (l *loader) NeedLeaderElection() bool { return false }

func (l *loader) Start(ctx context.Context) error {
	// Every replica runs this loop, so even the first read waits a random moment:
	// replicas restarted together do not all read at once.
	first := l.interval / 30
	if first <= 0 {
		first = time.Millisecond
	}
	select {
	case <-ctx.Done():
		return nil
	case <-time.After(rand.N(first)):
	}
	for {
		l.tick(ctx)
		wait := l.interval
		if !l.store.ready() {
			wait = l.interval / 6
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(jittered(wait)):
		}
	}
}

func (l *loader) tick(ctx context.Context) {
	bundle, _, found, err := l.secrets.read(ctx)
	if err != nil {
		l.logger.Warn("Cannot read the conversion webhook certificate", slog.Any("error", err))
		return
	}
	if !found {
		l.logger.Info("The conversion webhook certificate Secret is missing; recreating it")
		if bundle, err = l.secrets.ensure(ctx); err != nil {
			l.logger.Warn("Cannot recreate the conversion webhook certificate", slog.Any("error", err))
			return
		}
	}
	if l.store.serving(bundle) {
		return
	}
	if err := l.serve(ctx, bundle); err != nil {
		l.logger.Warn("Cannot serve the conversion webhook certificate", slog.Any("error", err))
		return
	}
	l.logger.Info("Loaded the conversion webhook certificate")
}
