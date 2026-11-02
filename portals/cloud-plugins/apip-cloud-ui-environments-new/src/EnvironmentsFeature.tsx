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
import { Alert, PageContent, PageTitle } from '@wso2/oxygen-ui';
import EnvironmentForm from './EnvironmentForm';
import EnvironmentsList from './EnvironmentsList';
import type { AIWorkspaceHostPort } from './hostPort';
import { createApiEnvironmentPort } from './apiPort';
import { ENVIRONMENT_OPERATIONS } from './permissions';

export type EnvironmentsFeatureProps = { port: AIWorkspaceHostPort };

const EnvironmentsFeature: FC<EnvironmentsFeatureProps> = ({ port }) => {
  const readOnly = Boolean(port.projectHandle);
  const [view, setView] = useState<'list' | 'create'>('list');
  // Backed by platform-api's /environments endpoints through the host-injected apiFetch.
  const environmentPort = useMemo(() => createApiEnvironmentPort(port.apiFetch), [port.apiFetch]);

  // What the signed-in user may be offered, from the scopes each operation
  // accepts. The server still decides; this keeps the page from offering a call
  // it would refuse, and from making one it knows will be refused.
  const { canAny } = port.permissions;
  const canList = canAny(ENVIRONMENT_OPERATIONS.ListEnvironments);
  const canCreate = canAny(ENVIRONMENT_OPERATIONS.CreateEnvironment);
  const canDelete = canAny(ENVIRONMENT_OPERATIONS.DeleteEnvironment);

  if (!canList) {
    return (
      <PageContent fullWidth>
        <PageTitle sx={{ mb: 3 }}>
          <PageTitle.Header>Environments</PageTitle.Header>
        </PageTitle>
        <Alert severity="warning">You don&apos;t have permission to view environments.</Alert>
      </PageContent>
    );
  }

  if (!readOnly && canCreate && view === 'create') {
    return <EnvironmentForm port={environmentPort} onBack={() => setView('list')} notify={port.notify} />;
  }

  return (
    <EnvironmentsList
      port={environmentPort}
      readOnly={readOnly}
      canCreate={canCreate}
      canDelete={canDelete}
      onCreateClick={() => setView('create')}
      notify={port.notify}
    />
  );
};

export default EnvironmentsFeature;
