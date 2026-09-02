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

import { useMemo, useState, type FC } from 'react';
import EnvironmentForm from './EnvironmentForm';
import EnvironmentsList from './EnvironmentsList';
import type { AIWorkspaceHostPort } from './hostPort';
import { createMockEnvironmentPort } from './mockPort';

export type EnvironmentsFeatureProps = { port: AIWorkspaceHostPort };

const EnvironmentsFeature: FC<EnvironmentsFeatureProps> = ({ port }) => {
  const readOnly = Boolean(port.projectHandle);
  const [view, setView] = useState<'list' | 'create'>('list');
  // No real backend exists yet, so each mount gets its own in-memory port.
  const environmentPort = useMemo(() => createMockEnvironmentPort(), []);

  if (!readOnly && view === 'create') {
    return <EnvironmentForm port={environmentPort} onBack={() => setView('list')} notify={port.notify} />;
  }

  return (
    <EnvironmentsList
      port={environmentPort}
      readOnly={readOnly}
      onCreateClick={() => setView('create')}
      notify={port.notify}
    />
  );
};

export default EnvironmentsFeature;
