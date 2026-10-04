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
import ComingSoonPanel from './ComingSoonPanel';

export type ScopedLogsNoticeProps = {
  /** Named in the message, the way the built-in Metrics page names its own. */
  scope: 'project' | 'API';
  /** Sends the reader to the organization's own logs page. */
  onViewOrganizationLogs: () => void;
};

/**
 * Observability > Logs inside a project or an API, where there is nothing to
 * show: a gateway's log line carries no API identity, and the observability API
 * scopes a query by organization namespace, so the only real logs page is the
 * organization's.
 *
 * Deliberately the same panel the sibling Metrics page uses, with one thing
 * added — the way out.
 */
const ScopedLogsNotice: FC<ScopedLogsNoticeProps> = ({ onViewOrganizationLogs, scope }) => (
  <ComingSoonPanel
    action={{ label: 'View organization logs', onClick: onViewOrganizationLogs }}
    detail="Only organization-level gateway logs are available today."
    feature={`Runtime logs for this ${scope}`}
  />
);

export default ScopedLogsNotice;
