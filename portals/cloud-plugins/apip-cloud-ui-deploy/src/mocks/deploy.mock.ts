/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { isoMinutesAgo } from '../utils/time';
import type { BuildRecord, Environment } from '../types';

/**
 * Seed data for the Deploy page demo: Development has one active, one failed
 * and one never-deployed gateway; Stage has two active gateways; Production
 * has three gateways, none deployed yet. Timestamps are relative to load
 * time so "N hours ago" stays sensible however long the demo session runs.
 */
export function seedEnvironments(): Environment[] {
  return [
    {
      id: 'development',
      name: 'Development',
      envVars: 6,
      gateways: [
        {
          id: 'dev-eu',
          name: 'EU Gateway',
          region: 'EU (Frankfurt)',
          status: 'active',
          buildId: 'b-1042',
          deployedAt: isoMinutesAgo(180),
          envVars: 4,
          history: [
            { result: 'Success', buildId: 'b-1042', when: isoMinutesAgo(180) },
            { result: 'Failed', buildId: 'b-1039', when: isoMinutesAgo(1500) },
          ],
        },
        {
          id: 'dev-us',
          name: 'US Gateway',
          region: 'US (Oregon)',
          status: 'failed',
          buildId: 'b-1041',
          deployedAt: isoMinutesAgo(300),
          envVars: 2,
          history: [
            { result: 'Failed', buildId: 'b-1041', when: isoMinutesAgo(300) },
            { result: 'Success', buildId: 'b-1035', when: isoMinutesAgo(2880) },
          ],
        },
        {
          id: 'dev-apac',
          name: 'APAC Gateway',
          region: 'AP (Singapore)',
          status: 'none',
          envVars: 0,
          history: [],
        },
      ],
    },
    {
      id: 'stage',
      name: 'Stage',
      envVars: 5,
      gateways: [
        {
          id: 'stage-eu',
          name: 'EU Gateway',
          region: 'EU (Frankfurt)',
          status: 'active',
          buildId: 'b-1030',
          deployedAt: isoMinutesAgo(1440),
          envVars: 5,
          history: [{ result: 'Success', buildId: 'b-1030', when: isoMinutesAgo(1440) }],
        },
        {
          id: 'stage-us',
          name: 'US Gateway',
          region: 'US (Oregon)',
          status: 'active',
          buildId: 'b-1030',
          deployedAt: isoMinutesAgo(1440),
          envVars: 6,
          history: [{ result: 'Success', buildId: 'b-1030', when: isoMinutesAgo(1440) }],
        },
      ],
    },
    {
      id: 'production',
      name: 'Production',
      envVars: 5,
      gateways: [
        {
          id: 'prod-eu',
          name: 'EU Gateway',
          region: 'EU (Frankfurt)',
          status: 'none',
          envVars: 0,
          history: [],
        },
        {
          id: 'prod-us',
          name: 'US Gateway',
          region: 'US (Oregon)',
          status: 'none',
          envVars: 0,
          history: [],
        },
        {
          id: 'prod-apac',
          name: 'APAC Gateway',
          region: 'AP (Singapore)',
          status: 'none',
          envVars: 0,
          history: [],
        },
      ],
    },
  ];
}

export function seedBuildHistory(): BuildRecord[] {
  return [
    {
      id: 'build-1042',
      buildId: 'b-1042',
      result: 'Success',
      when: isoMinutesAgo(180),
      targetEnvironmentId: 'development',
      targetGatewayCount: 1,
    },
    {
      id: 'build-1041',
      buildId: 'b-1041',
      result: 'Failed',
      when: isoMinutesAgo(300),
      targetEnvironmentId: 'development',
      targetGatewayCount: 1,
    },
    {
      id: 'build-1030',
      buildId: 'b-1030',
      result: 'Success',
      when: isoMinutesAgo(1440),
      targetEnvironmentId: 'stage',
      targetGatewayCount: 2,
    },
    {
      id: 'build-1035',
      buildId: 'b-1035',
      result: 'Success',
      when: isoMinutesAgo(2880),
      targetEnvironmentId: 'development',
      targetGatewayCount: 1,
    },
    {
      id: 'build-1020',
      buildId: 'b-1020',
      result: 'Failed',
      when: isoMinutesAgo(5760),
      targetEnvironmentId: 'development',
      targetGatewayCount: 3,
    },
  ];
}
