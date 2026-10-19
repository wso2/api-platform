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

import { useMemo } from 'react';
import { useParams } from 'react-router-dom';

import { AppPage } from '@/components/AppPage';
import { PortalPublishPageContent } from '../../portals/PortalPublishPage';
import { graphqlPublishTarget } from '../../portals/publishTarget';

/**
 * Publishing a GraphQL API to one API Portal: REST's publish page over the
 * GraphQL API's own draft and publication, with its SDL as the definition.
 */
export function GraphqlPortalPublishPage() {
  const { graphqlApiHandler = '', orgHandle = '', projectHandler = '' } = useParams();
  const target = useMemo(
    () => graphqlPublishTarget(orgHandle, projectHandler, graphqlApiHandler),
    [graphqlApiHandler, orgHandle, projectHandler],
  );

  return (
    <AppPage>
      <PortalPublishPageContent target={target} />
    </AppPage>
  );
}
