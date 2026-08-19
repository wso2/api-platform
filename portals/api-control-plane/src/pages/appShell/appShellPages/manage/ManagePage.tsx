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

import { Button, Card, CardContent, Stack, TextField, PageContent, PageTitle } from '@wso2/oxygen-ui';

import { useApi } from '../../../../api/hooks/useMvpQueries';
import { ErrorState, LoadingState } from '../../../../components/StateViews';

export function ManagePage() {
  const apiQuery = useApi();

  if (apiQuery.isLoading) return <LoadingState label="Loading manage view" />;
  if (!apiQuery.data) return <ErrorState title="API not found" />;

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>Manage {apiQuery.data.displayName}</PageTitle.Header>
        <PageTitle.SubHeader>Core editable metadata only for MVP.</PageTitle.SubHeader>
      </PageTitle>
      <Card variant="outlined">
        <CardContent>
          <Stack spacing={3}>
            <TextField disabled label="Display name" value={apiQuery.data.displayName} />
            <TextField disabled label="Description" multiline minRows={3} value={apiQuery.data.description || ''} />
            <TextField disabled label="Visibility" value={apiQuery.data.httpBased ? 'HTTP based' : 'Internal'} />
            <Button disabled variant="contained">Save changes</Button>
          </Stack>
        </CardContent>
      </Card>
    </PageContent>
  );
}
