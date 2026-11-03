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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { Link } from '@wso2/oxygen-ui';
import type { JSX } from 'react';

export type ReadOnlyNoticeProps = {
  /** Where to restore write access. Omitted when no billing console is configured. */
  upgradeHref?: string;
};

/**
 * What the host's read-only banner says.
 *
 * Markup rather than a string because the copy has a phrase in bold and a link
 * in the middle of it — and because the wording and the destination both belong
 * on this side. The host renders what it is handed and knows about neither
 * plans nor the billing console, whose address is this plugin's own config.
 *
 * Deliberately not translated here: the host portal owns `react-intl`, and this
 * package is not wired into its message catalogue. The strings are
 * cloud-commercial copy that only ever renders in a WSO2 Cloud deployment.
 */
export default function ReadOnlyNotice({ upgradeHref }: ReadOnlyNoticeProps): JSX.Element {
  return (
    <>
      This organization is in <strong>read-only mode</strong> because its trial has
      ended. Creating, editing and deploying are disabled; everything stays visible.
      {upgradeHref ? (
        <>
          {' '}
          <Link href={upgradeHref} underline="always" color="inherit">
            Subscribe to a plan
          </Link>{' '}
          to restore full access.
        </>
      ) : null}
    </>
  );
}
