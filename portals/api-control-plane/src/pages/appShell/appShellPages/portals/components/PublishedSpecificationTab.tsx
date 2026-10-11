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

import { useMemo, useState } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { ErrorState, LoadingState } from '@/components/StateViews';
import type { SpecFormat } from '../../apis/create/utils/specText';
import { reformatDefinition, type StoredDefinition } from '../utils/storedDefinition';
import { SpecificationTab } from './SpecificationTab';

const messages = defineMessages({
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PublishedSpecificationTab.loading',
    defaultMessage: 'Loading the published specification',
  },
  error: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PublishedSpecificationTab.error',
    defaultMessage: 'Unable to load the published specification.',
  },
});

type PublishedSpecificationTabProps = {
  /** The live definition; absent when the portal holds none. */
  definition?: StoredDefinition;
  failed: boolean;
  isLoading: boolean;
};

/**
 * The published version's definition, read-only. It opens in the serialization
 * it was published in and can be re-printed as the other one for reading; that
 * choice is only a view, derived from the stored text, so it never goes stale
 * and never touches the draft's own format.
 */
export function PublishedSpecificationTab({ definition, failed, isLoading }: PublishedSpecificationTabProps) {
  const intl = useIntl();
  const [chosenFormat, setChosenFormat] = useState<SpecFormat>();
  const format = chosenFormat ?? definition?.format ?? 'json';
  const text = useMemo(() => (definition ? reformatDefinition(definition, format) : ''), [definition, format]);

  if (isLoading) return <LoadingState label={intl.formatMessage(messages.loading)} />;
  if (failed) return <ErrorState message={intl.formatMessage(messages.error)} />;

  return <SpecificationTab format={format} onFormatChange={setChosenFormat} readOnly text={text} />;
}
