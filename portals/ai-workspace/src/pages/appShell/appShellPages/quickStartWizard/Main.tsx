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

import React, { useEffect, useState, type JSX } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { useRole } from '../../../../contexts/RoleContext';
import { buildOrgPath, getOrgSlug } from '../../../../utils/projectRouting';
import { dismissQuickStart } from '../../../../utils/quickStartUtils';
import QuickStartErrorBoundary from './QuickStartErrorBoundary';
import QuickStartLayout from './QuickStartLayout';
import LLMProviderQuickStart from './LLMProviderQuickStart';
import MCPProxyQuickStart from './MCPProxyQuickStart';

type ResourceType = 'provider' | 'mcp';

export default function QuickStartWizard(): JSX.Element {
  const navigate = useNavigate();
  const { currentOrganization, currentProject, setCurrentProject } = useAppShell();
  const { role } = useRole();

  const [resourceType, setResourceType] = useState<ResourceType>(
    role === 'developer' ? 'mcp' : 'provider'
  );

  useEffect(() => {
    if (currentProject) {
      setCurrentProject(null);
    }
  }, [currentProject, setCurrentProject]);

  useEffect(() => {
    if (role === 'developer') {
      setResourceType('mcp');
    }
  }, [role]);

  const orgHomePath = buildOrgPath(currentOrganization, '/home');
  const handleClose = () => {
    dismissQuickStart(getOrgSlug(currentOrganization));
    navigate(orgHomePath);
  };

  return (
    <QuickStartLayout onClose={handleClose}>
      <QuickStartErrorBoundary>
        {resourceType === 'provider' ? (
          <LLMProviderQuickStart
            resourceType={resourceType}
            onResourceTypeChange={setResourceType}
          />
        ) : (
          <MCPProxyQuickStart
            role={role}
            resourceType={resourceType}
            onResourceTypeChange={setResourceType}
            onClose={handleClose}
          />
        )}
      </QuickStartErrorBoundary>
    </QuickStartLayout>
  );
}
