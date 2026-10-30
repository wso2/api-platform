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

import { Typography } from '@wso2/oxygen-ui';
import type { ReactNode } from 'react';
import { FormattedMessage, type MessageDescriptor } from 'react-intl';

type SelectionSummaryProps = {
  /** The number in the published message: what the listing holds. */
  count: number;
  /** Published view: `{count, plural, ...}` with its number wrapped in an `<n>` tag, which the summary styles. */
  published: MessageDescriptor;
  readOnly?: boolean;
  /** Draft view: `{count} of {total} ...`, where `{count}` is the styled number of selected items. */
  selected: MessageDescriptor;
  /** The number of items offered, for the draft message. */
  total: number;
};

/**
 * The line over a selection tab: how many items are selected in the draft, or published in the live
 * listing, with the number set larger in the version's colour (primary for the draft, success for the live one).
 */
export function SelectionSummary({ count, published, readOnly, selected, total }: SelectionSummaryProps) {
  const styled = (value: ReactNode) => (
    <Typography component="span" sx={{ color: readOnly ? 'success.main' : 'primary.main', fontSize: 20, fontWeight: 500 }}>
      {value}
    </Typography>
  );

  return (
    <Typography color="text.secondary" variant="body2">
      {readOnly ? (
        <FormattedMessage {...published} values={{ count, n: styled }} />
      ) : (
        <FormattedMessage {...selected} values={{ count: styled(count), total }} />
      )}
    </Typography>
  );
}
