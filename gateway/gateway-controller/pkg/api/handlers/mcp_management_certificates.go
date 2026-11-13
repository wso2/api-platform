/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/certificate"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// manageCertificates serves wso2_apip_gw_manage_certificates, calling
// CertificateService directly as the REST handlers do.
func (h *McpHandler) manageCertificates(ctx context.Context, _ *mcp.CallToolRequest, in manageCertificatesInput) (*mcp.CallToolResult, any, error) {
	op, err := resolveCertAction(in.Action)
	if err != nil {
		return nil, nil, err
	}
	// The immutable middleware rejects every POST/PUT/DELETE on the management
	// router, so REST already refuses every mutating action
	if op.Mutating && h.immutable {
		return nil, nil, fmt.Errorf(
			"this Gateway runs in immutable mode; certificates are loaded at startup and cannot be changed at runtime")
	}
	if op.NeedsConfirm && !in.Confirm {
		return nil, nil, fmt.Errorf(
			"refusing to %s: confirm with the user, then retry with confirm=true", op.Action)
	}
	if err := h.authz.authorize(ctx, op.RouteKey); err != nil {
		return nil, nil, err
	}

	// POST and PUT /certificates cap their body at max_certificate_upload_bytes.
	// Update checks it after the stored certificate, as PUT does, in its case.
	if op.Action == certActionApply && h.certificateRequestTooLarge(in) {
		return nil, nil, fmt.Errorf("the certificate request is too large for this Gateway; nothing was stored")
	}

	correlationID := uuid.NewString()
	log := h.logger.With(
		slog.String("correlation_id", correlationID),
		slog.String("source", "mcp"),
		slog.String("tool", toolManageCertificates),
		slog.String("action", op.Action))

	switch op.Action {
	case certActionList:
		result, err := h.certificateService.List(certificate.ListParams{Usage: in.Usage, Logger: log})
		if err != nil {
			log.Error("MCP certificate listing failed", slog.Any("error", err))
			return nil, nil, mcpCertError(certActionList, "", err)
		}
		// Rendered as GET /certificates renders it. CertificateResponse, not the
		// stored model: StoredCertificate marshals its PEM bytes under
		// "certificate", so returning rows directly would put every stored chain
		// into the transcript.
		items := make([]CertificateResponse, 0, len(result.Items))
		for _, item := range result.Items {
			items = append(items, listedCertificateToResponse(item))
		}
		return nil, map[string]any{
			"count": len(items), "totalBytes": result.TotalBytes, "items": items,
		}, nil

	case certActionApply:
		// An id on apply means the caller expected an update. Quietly creating a
		// second certificate instead is the wrong recovery.
		if strings.TrimSpace(in.ID) != "" {
			return nil, nil, fmt.Errorf(
				"apply only creates, so it does not take an id. To replace a usage: identity certificate's " +
					"chain and key in place, use action=update with that id; to replace any other certificate, " +
					"delete it and apply the new one")
		}
		match, matchErrors := certificateMatchFromInput(in.Match)

		result, err := h.certificateService.Upload(certificate.UploadParams{
			Name:          in.Name,
			Certificate:   in.Certificate,
			Usage:         in.Usage,
			Role:          in.Role,
			Match:         match,
			PrivateKey:    in.PrivateKey,
			FieldErrors:   matchErrors,
			CorrelationID: correlationID,
			Logger:        log,
		})
		if err != nil {
			log.Error("MCP certificate upload failed", slog.String("name", in.Name), slog.Any("error", err))
			return nil, nil, mcpCertError(certActionApply, in.Name, err)
		}
		return nil, map[string]any{
			"status": "success", "operation": "create",
			"certificate": uploadResultToResponse(result),
			"message":     "Certificate stored and pushed to the router.",
		}, nil

	case certActionUpdate:
		if strings.TrimSpace(in.ID) == "" {
			return nil, nil, fmt.Errorf(`update requires "id"; call this tool with action=list to find it`)
		}
		// As PUT /certificates/{id}: the stored certificate is checked first,
		// then the size of what is sent.
		existing, err := h.certificateService.PrepareUpdate(in.ID, log)
		if err != nil {
			log.Error("MCP certificate update failed", slog.String("id", in.ID), slog.Any("error", err))
			return nil, nil, mcpCertError(certActionUpdate, in.ID, err)
		}
		if h.certificateRequestTooLarge(in) {
			return nil, nil, fmt.Errorf("the certificate request is too large for this Gateway; nothing was stored")
		}
		// Name, usage, role and match never change on rotation, so only the
		// certificate and key are passed.
		result, err := h.certificateService.Update(certificate.UpdateParams{
			ID:            in.ID,
			Existing:      existing,
			Certificate:   in.Certificate,
			PrivateKey:    in.PrivateKey,
			CorrelationID: correlationID,
			Logger:        log,
		})
		if err != nil {
			log.Error("MCP certificate update failed", slog.String("id", in.ID), slog.Any("error", err))
			return nil, nil, mcpCertError(certActionUpdate, in.ID, err)
		}
		return nil, map[string]any{
			"status": "success", "operation": "update",
			"certificate": updateResultToResponse(result),
			"message":     "Certificate and private key replaced and pushed to the router.",
		}, nil

	case certActionDelete:
		if strings.TrimSpace(in.ID) == "" {
			return nil, nil, fmt.Errorf(`delete requires "id"; call this tool with action=list to find it`)
		}
		if _, err := h.certificateService.Delete(certificate.DeleteParams{
			ID:            in.ID,
			CorrelationID: correlationID,
			Logger:        log,
		}); err != nil {
			log.Error("MCP certificate deletion failed", slog.String("id", in.ID), slog.Any("error", err))
			return nil, nil, mcpCertError(certActionDelete, in.ID, err)
		}
		return nil, map[string]any{
			"status": "success", "id": in.ID,
			"message": "Certificate deleted and the resulting trust store pushed to the router.",
		}, nil

	default:
		result, err := h.certificateService.Reload(certificate.ReloadParams{
			CorrelationID: correlationID,
			Logger:        log,
		})
		if err != nil {
			log.Error("MCP certificate reload failed", slog.Any("error", err))
			return nil, nil, mcpCertError(certActionReload, "", err)
		}
		return nil, map[string]any{
			"status": "success", "totalBytes": result.TotalBytes,
			"message": "Every stored certificate was re-read and pushed to the router.",
		}, nil
	}
}

// certificateMatchFromInput converts the tool's match argument. An empty
// object is refused and dropped, exactly as POST /certificates refuses it;
// unknown keys and non-list values are already refused by the tool's input
// schema, as REST's decoder refuses them.
func certificateMatchFromInput(in *certificateMatchInput) (*models.CertificateMatch, []certificate.FieldError) {
	if in == nil {
		return nil, nil
	}
	if in.URISANs == nil && in.DNSSANs == nil {
		return nil, []certificate.FieldError{{
			Field:   "match",
			Message: "match must list uriSANs or dnsSANs, or be omitted",
		}}
	}
	return &models.CertificateMatch{URISANs: in.URISANs, DNSSANs: in.DNSSANs}, nil
}

// certificateRequestTooLarge reports whether the REST request equivalent to in
// would exceed the body limit POST and PUT /certificates enforce.
func (h *McpHandler) certificateRequestTooLarge(in manageCertificatesInput) bool {
	if h.maxCertificateUploadBytes <= 0 {
		return false
	}
	match, _ := certificateMatchFromInput(in.Match)
	body, err := json.Marshal(UploadCertificateRequest{
		Name:        in.Name,
		Certificate: in.Certificate,
		Usage:       in.Usage,
		Role:        in.Role,
		Match:       match,
		PrivateKey:  in.PrivateKey,
	})
	if err != nil {
		return true
	}
	return int64(len(body)) > h.maxCertificateUploadBytes
}

// mcpCertError maps a CertificateService failure to a message for the model,
// reading the service's sentinels rather than its error strings.
func mcpCertError(action, subject string, err error) error {
	if errors.Is(err, certificate.ErrCertStoreNotConfigured) {
		// Upload saves the row before it discovers there is no cert store, so
		// apply has changed state even though it failed. Say so, or the model
		// retries into a name conflict it cannot clear (delete is refused too).
		if action == certActionApply {
			return fmt.Errorf(
				"certificate %q was stored, but the Gateway's certificate store is unavailable, so the "+
					"router is not using it. Do not repeat this call: the name is now taken, and certificates "+
					"cannot be deleted or reloaded until the certificate store is available. Tell the user; "+
					"the certificate appears in action=list", subject)
		}
		// Update writes the new chain and key before it finds there is no cert
		// store, so it too has changed state.
		if action == certActionUpdate {
			return fmt.Errorf(
				"the new certificate and key for %q were stored, but the Gateway's certificate store is "+
					"unavailable, so the router is not using them. Do not repeat this call. Tell the user", subject)
		}
		return fmt.Errorf("the Gateway's certificate store is unavailable, so certificate operations cannot run right now")
	}
	// Every problem in the request at once, field by field, as REST reports them in its 400.
	var validationErr *certificate.ValidationError
	if errors.As(err, &validationErr) {
		lines := make([]string, 0, len(validationErr.Fields))
		for i, f := range validationErr.Fields {
			lines = append(lines, fmt.Sprintf("  %d. %s: %s", i+1, f.Field, f.Message))
		}
		refused := "the Gateway refused the certificate"
		if action == certActionList {
			refused = "the Gateway refused the listing"
		}
		return fmt.Errorf("%s: %s:\n%s", refused, validationErr.Message, strings.Join(lines, "\n"))
	}

	if errors.Is(err, certificate.ErrNotFound) {
		return fmt.Errorf("no certificate with id %q; call this tool with action=list to find it", subject)
	}
	// A failed read before an update or delete. Nothing was changed; the
	// storage error itself stays in the log.
	var readErr *certificate.PersistError
	if errors.As(err, &readErr) && readErr.Op == certificate.OpLoad {
		return fmt.Errorf("the Gateway could not read certificate %q, so nothing was changed. Retry later", subject)
	}

	// Delete refused because deployed APIs still depend on the certificate.
	// Naming each one is what lets the model remove those references first.
	var inUse *certificate.InUseError
	if errors.As(err, &inUse) {
		lines := make([]string, 0, len(inUse.References))
		for i, ref := range inUse.References {
			lines = append(lines, fmt.Sprintf("  %d. API %q (%s)", i+1, ref.API, ref.Field))
		}
		return fmt.Errorf("cannot delete certificate %q: %s:\n%s", subject, inUse.Message, strings.Join(lines, "\n"))
	}

	var refCheck *certificate.ReferenceCheckError
	if errors.As(err, &refCheck) {
		return fmt.Errorf("cannot delete certificate %q: the Gateway could not check whether a deployed API "+
			"still uses it, so nothing was deleted. Retry later", subject)
	}

	var defaultErr *certificate.DefaultIdentityExistsError
	if errors.As(err, &defaultErr) {
		return errors.New(defaultErr.Error())
	}
	if errors.Is(err, certificate.ErrIdentityStorageUnavailable) {
		return fmt.Errorf("this Gateway has no encryption provider configured, so gateway identities cannot be stored")
	}
	var encryptErr *certificate.EncryptionError
	if errors.As(err, &encryptErr) {
		return fmt.Errorf("the Gateway could not store the gateway identity's private key; nothing was stored")
	}

	// Checked before PersistError: SaveCertificate's duplicate-name conflict
	// arrives wrapped in one, and "that name is taken" is the actionable half.
	if storage.IsConflictError(err) {
		return fmt.Errorf(
			"a certificate named %q already exists. Names are unique across every usage: choose a different "+
				"name, or delete that one first. To rotate a usage: identity certificate's chain and key in "+
				"place, use action=update with its id", subject)
	}

	var syncErr *certificate.SyncError
	if errors.As(err, &syncErr) {
		if syncErr.Persisted {
			// The stored state is already correct and only the push failed.
			// Repeating the write would conflict or duplicate; say so.
			return fmt.Errorf(
				"the change was stored but could not be pushed to the router (it failed at the %s stage). "+
					"Do not repeat this call — retry only the push, with action=reload", syncErr.Stage)
		}
		return fmt.Errorf(
			"nothing changed: the trust store could not be pushed to the router (it failed at the %s stage)",
			syncErr.Stage)
	}

	var persistErr *certificate.PersistError
	if errors.As(err, &persistErr) && persistErr.Op == certificate.OpDelete {
		return fmt.Errorf(
			"cannot delete certificate %q: no certificate with that id, or it could not be removed. "+
				"Call this tool with action=list to see what is stored", subject)
	}

	return fmt.Errorf("the Gateway could not %s the certificate", action)
}
