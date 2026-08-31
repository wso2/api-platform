/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import type { FC } from 'react';
import { Chip } from '@wso2/oxygen-ui';
import type { StatusTone } from '../utils/status';

export type StatusPillProps = {
  tone: StatusTone;
};

/** A status Chip using Oxygen's own `color` palette — the theme decides the actual hue, this only picks which semantic bucket applies. */
const StatusPill: FC<StatusPillProps> = ({ tone }) => (
  <Chip label={tone.label} size="small" color={tone.tone} sx={{ fontWeight: 600 }} />
);

export default StatusPill;
