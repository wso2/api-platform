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
import { Route, Routes, useNavigate, useParams } from 'react-router-dom';
import GatewayForm from './GatewayForm';
import GatewaysList from './GatewaysList';
import type { AIWorkspaceHostPort } from './hostPort';

export type GatewaysFeatureProps = {
  port: AIWorkspaceHostPort;
};

/**
 * The extension's `render(port)` result for the `page.gateways` override
 * slot: a self-contained list/create/edit flow with its own nested routes
 * (`index`, `create`, `edit/:gatewayId`), mounted wherever the host's
 * `gateways/*` route places it — see `App.tsx`'s `GatewaysRoute`. Mirrors
 * the URL shape the built-in `GatewaysLayout` uses, so this is a drop-in
 * swap rather than a new navigation pattern.
 */
const GatewaysFeature: FC<GatewaysFeatureProps> = ({ port }) => (
  <Routes>
    <Route index element={<GatewaysListRoute port={port} />} />
    <Route path="create" element={<GatewaysFormRoute port={port} mode="create" />} />
    <Route path="edit/:gatewayId" element={<GatewaysFormRoute port={port} mode="edit" />} />
  </Routes>
);

const GatewaysListRoute: FC<{ port: AIWorkspaceHostPort }> = ({ port }) => {
  const navigate = useNavigate();
  return (
    <GatewaysList
      port={port}
      onAddClick={() => navigate('create')}
      onEditClick={(gatewayId) => navigate(`edit/${gatewayId}`)}
    />
  );
};

const GatewaysFormRoute: FC<{ port: AIWorkspaceHostPort; mode: 'create' | 'edit' }> = ({ port, mode }) => {
  const navigate = useNavigate();
  const { gatewayId } = useParams<{ gatewayId: string }>();
  return (
    <GatewayForm
      mode={mode}
      gatewayId={mode === 'edit' ? gatewayId : undefined}
      onBack={() => navigate('..')}
      notify={port.notify}
    />
  );
};

export default GatewaysFeature;
