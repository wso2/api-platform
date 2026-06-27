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
  Box,
  Chip,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronDown } from '@wso2/oxygen-ui-icons-react';
import type { HybridGateway } from '../../apis/gateway/types';
import { useGatewayDeploy } from '../../contexts/GatewayDeployContext';
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

  const hasDeployments = useMemo(() => {
    if (!deployments?.list) return false;
    return deployments.list.some((d) => d.gatewayHandle === gateway.handle);
  }, [deployments, gateway.handle]);

  const currentDeploymentDisplay = useMemo(() => {
    if (!deployments?.list) return null;

    const gatewayDeployments = deployments.list
      .filter((d) => d.gatewayHandle === gateway.handle)
      .sort((a, b) => {
        const timeA = a.createdAt ? new Date(a.createdAt).getTime() : 0;
        const timeB = b.createdAt ? new Date(b.createdAt).getTime() : 0;
        return timeB - timeA;
      });

    if (gatewayDeployments.length === 0) return null;

    const deployed = gatewayDeployments.find((d) => d.status === 'DEPLOYED');
    const target = deployed || gatewayDeployments[0];
    return target.name ?? `Deployment ${gatewayDeployments.length}`;
  }, [deployments, gateway.handle]);

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
        <GatewayDeployCardContent
          gateway={gateway}
          hasDeployments={hasDeployments}
          isGatewayActive={isActive}
        />
      </AccordionDetails>
    </Accordion>
  );
}
