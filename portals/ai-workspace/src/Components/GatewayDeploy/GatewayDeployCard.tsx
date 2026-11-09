/*
 * Copyright (c) 2026, WSO2 Inc. (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 Inc. and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein is strictly forbidden, unless permitted by WSO2 in accordance with
 * the WSO2 Commercial License available at http://wso2.com/licenses.
 * For specific language governing the permissions and limitations under
 * this license, please see the license as well as any agreement you've
 * entered into with WSO2 governing the purchase of this software and any
 * associated services.
 */

import { useMemo } from 'react';
import {
  Accordion,
  AccordionSummary,
  AccordionDetails,
  Alert,
  Box,
  Chip,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronDown, TriangleAlert } from '@wso2/oxygen-ui-icons-react';
import type { HybridGateway } from '../../apis/gateway/types';
import { useGatewayDeploy } from '../../contexts/GatewayDeployContext';
import { useAIEntity } from '../../contexts/AIEntitiesContext';
import { useAppAuth } from '../../contexts/AppAuthContext';
import { SCOPES } from '../../auth/permissions';
import { useGatewayPolicyManifestNames } from '../../hooks/useGatewayPolicyManifestNames';
import {
  getAIEntityTypeLabel,
  getEntityPolicyNames,
  getMissingGatewayPolicies,
} from '../../utils/policyCompatibility';
import GatewayDeployCardActions from './GatewayDeployCardActions';
import GatewayDeployCardContent from './GatewayDeployCardContent';

interface GatewayDeployCardProps {
  gateway: HybridGateway;
  isExpanded: boolean;
  onToggleExpand: (expanded: boolean) => void;
  onConfigureAndDeploy?: (gateway: HybridGateway) => void;
}

/**
 * Expandable card for a single hybrid gateway in the deploy list.
 */
export default function GatewayDeployCard({
  gateway,
  isExpanded,
  onToggleExpand,
  onConfigureAndDeploy,
}: GatewayDeployCardProps) {
  const isActive = gateway.status === 'connected';
  const { deployments } = useGatewayDeploy();

  // Gateway-policy compatibility check: shared across LLM Providers, LLM
  // Proxies, MCP Proxies and Agents since they all render through this same
  // card. `useAIEntity()` safely returns `entity: null` when no
  // `AIEntityProvider` is mounted above this card (entity kinds not yet
  // wired up), so the check is a no-op there rather than throwing.
  const {
    entityType,
    entity,
    isLoading: isLoadingEntity,
  } = useAIEntity();
  const { hasPermission } = useAppAuth();
  const canViewManifest = hasPermission(SCOPES.GATEWAY_MANIFEST_READ);

  const entityPolicyNames = useMemo(
    () => getEntityPolicyNames(entityType, entity),
    [entityType, entity]
  );

  const { policyNameSet: gatewayPolicyNameSet } = useGatewayPolicyManifestNames(
    canViewManifest && entityPolicyNames.length > 0 ? gateway.id : null
  );

  const missingPolicies = useMemo(() => {
    if (!gatewayPolicyNameSet || entityPolicyNames.length === 0) return [];
    return getMissingGatewayPolicies(entityPolicyNames, gatewayPolicyNameSet);
  }, [entityPolicyNames, gatewayPolicyNameSet]);

  const entityTypeLabel = getAIEntityTypeLabel(entityType);

  // Whether the compatibility verdict is still unknown — either the entity
  // itself hasn't loaded yet (its policy names aren't known), or it has
  // policies and the gateway's manifest hasn't resolved yet. Deploy must stay
  // disabled through this window too, not just once a mismatch is confirmed —
  // otherwise a click in the gap before either fetch resolves bypasses the
  // check entirely. Deliberately keyed off `gatewayPolicyNameSet === null`
  // rather than the hook's own `isLoading` flag: `policyNameSet` is the value
  // actually consumed below, with no separate loading boolean that could lag
  // it by a render — one fewer piece of state to get out of sync. Gated on
  // `canViewManifest`: without that scope the verdict can never be
  // determined, so there's nothing to wait for.
  const isCheckingPolicyCompatibility =
    canViewManifest &&
    (isLoadingEntity || (entityPolicyNames.length > 0 && gatewayPolicyNameSet === null));

  // Generic on purpose — the tooltip is a quick hover hint, not the place to
  // enumerate which policies are missing. The full list is in the banner below.
  const deployDisabledReason = isCheckingPolicyCompatibility
    ? 'Checking gateway policy compatibility…'
    : missingPolicies.length > 0
      ? `This gateway is missing ${missingPolicies.length === 1 ? 'a policy' : 'policies'} required by this ${entityTypeLabel}.`
      : undefined;

  const hasDeployments = useMemo(() => {
    if (!deployments?.list) return false;
    return deployments.list.some((d) => d.gatewayId === gateway.id);
  }, [deployments, gateway.id]);

  const currentDeploymentDisplay = useMemo(() => {
    if (!deployments?.list) return null;

    const gatewayDeployments = deployments.list
      .filter((d) => d.gatewayId === gateway.id)
      .sort((a, b) => {
        const timeA = a.createdAt ? new Date(a.createdAt).getTime() : 0;
        const timeB = b.createdAt ? new Date(b.createdAt).getTime() : 0;
        return timeB - timeA;
      });

    if (gatewayDeployments.length === 0) return null;

    const deployed = gatewayDeployments.find((d) => d.status === 'DEPLOYED');
    const target = deployed || gatewayDeployments[0];
    return target.name ?? `Deployment ${gatewayDeployments.length}`;
  }, [deployments, gateway.id]);

  return (
    <Accordion
      expanded={isExpanded}
      onChange={(_, expanded) => onToggleExpand(expanded)}
      variant="outlined"
      sx={{
        borderRadius: '8px',
        overflow: 'hidden',
        '&:before': { display: 'none' },
        '&.Mui-expanded': {
          margin: 0,
          borderRadius: '8px',
        },
        '&:first-of-type': {
          borderTopLeftRadius: '8px',
          borderTopRightRadius: '8px',
        },
        '&:last-of-type': {
          borderBottomLeftRadius: '8px',
          borderBottomRightRadius: '8px',
        },
      }}
    >
      <AccordionSummary
        sx={{
          px: 3,
          // py: 2,
          '& .MuiAccordionSummary-content': {
            m: 0,
            alignItems: 'center',
            justifyContent: 'space-between',
            flexWrap: 'wrap',
          },
        }}
      >
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            flexWrap: 'wrap',
            width: '100%',
          }}
        >
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              flexWrap: 'wrap',
              gap: 1.5,
            }}
          >
            <Typography variant="h6" sx={{ fontWeight: 500 }}>
              {gateway.displayName}
            </Typography>
            <Chip
              size="small"
              variant="outlined"
              label={isActive ? 'Active' : 'Not Active'}
              color={isActive ? 'success' : 'error'}
            />
            {missingPolicies.length > 0 && (
              <Chip
                size="small"
                variant="outlined"
                color="warning"
                icon={<TriangleAlert size={14} />}
                label={`Missing ${missingPolicies.length} ${missingPolicies.length === 1 ? 'policy' : 'policies'}`}
              />
            )}
            {currentDeploymentDisplay != null && (
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  component="span"
                >
                  Current Deployment:
                </Typography>
                <Chip
                  size="small"
                  color="default"
                  variant="outlined"
                  label={currentDeploymentDisplay}
                />
              </Box>
            )}
          </Box>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
            <Box onClick={(e) => e.stopPropagation()} component="span">
              <GatewayDeployCardActions
                gateway={gateway}
                onConfigureAndDeploy={onConfigureAndDeploy}
                disabled={!isActive}
                disabledReason={deployDisabledReason}
              />
            </Box>
            <ChevronDown
              size={20}
              style={{
                transition: 'transform 0.2s ease',
                transform: isExpanded ? 'rotate(180deg)' : 'rotate(0deg)',
              }}
            />
          </Box>
        </Box>
      </AccordionSummary>
      <AccordionDetails sx={{ px: 3, py: 2 }}>
        {missingPolicies.length > 0 && (
          <Alert severity="warning" sx={{ mb: 2 }}>
            Deployment is disabled because this gateway is missing{' '}
            {missingPolicies.length === 1 ? 'a policy' : 'policies'} used by
            this {entityTypeLabel}: {missingPolicies.join(', ')}. Install{' '}
            {missingPolicies.length === 1 ? 'it' : 'them'} on the gateway to
            enable deployment here.
          </Alert>
        )}
        <GatewayDeployCardContent
          gateway={gateway}
          hasDeployments={hasDeployments}
          isGatewayActive={isActive}
        />
      </AccordionDetails>
    </Accordion>
  );
}
