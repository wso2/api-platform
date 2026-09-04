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

import type { FC } from 'react';
import { Chip } from '@wso2/oxygen-ui';
import type { StatusTone } from '../utils/status';

export type StatusPillProps = {
  tone: StatusTone;
  variant?: 'filled' | 'outlined';
};

/** A status Chip using Oxygen's own `color` palette — the theme decides the actual hue, this only picks which semantic bucket applies. */
const StatusPill: FC<StatusPillProps> = ({ tone, variant }) => (
  <Chip label={tone.label} size="small" color={tone.tone} variant={variant} sx={{ fontWeight: 600 }} />
);

export default StatusPill;
