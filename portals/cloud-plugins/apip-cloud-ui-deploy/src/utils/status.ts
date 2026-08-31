/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import type { Gateway, GatewayStatus } from '../types';

/** Maps onto Oxygen/MUI's own `color` prop (Chip, Alert, etc.) so status coloring always follows the active theme instead of hardcoded hex. */
export type Tone = 'success' | 'warning' | 'error' | 'default';

export type StatusTone = {
  label: string;
  tone: Tone;
};

const GATEWAY_STATUS_TONE: Record<GatewayStatus, StatusTone> = {
  active: { label: 'Active', tone: 'success' },
  failed: { label: 'Failed', tone: 'error' },
  deploying: { label: 'Deploying', tone: 'warning' },
  none: { label: 'Not deployed', tone: 'default' },
};

export function gatewayStatusTone(status: GatewayStatus): StatusTone {
  return GATEWAY_STATUS_TONE[status];
}

export function activeGatewayCount(gateways: Gateway[]): number {
  return gateways.filter((gateway) => gateway.status === 'active').length;
}

/** Whether anything has ever been deployed in this environment — gates the Promote button. */
export function hasAnyDeployment(gateways: Gateway[]): boolean {
  return gateways.some((gateway) => gateway.status !== 'none');
}
