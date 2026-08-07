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

import { Sidebar, useAppShell } from '@wso2/oxygen-ui';
import { Link } from 'react-router-dom';

import { useNavigationGroups } from '../navigation/useNavigationItems';

export function AppSidebar() {
  const groups = useNavigationGroups();
  const { state } = useAppShell();
  const activeItem = groups
    .flatMap((group) => group.items)
    .find((item) => item.isActive)?.id;

  return (
    <Sidebar activeItem={activeItem} collapsed={state.sidebarCollapsed}>
      <Sidebar.Nav>
        {groups.map((group) => (
          <Sidebar.Category key={group.label}>
            <Sidebar.CategoryLabel>{group.label}</Sidebar.CategoryLabel>
            {group.items.map((item) => (
              <Sidebar.Item
                key={item.id}
                id={item.id}
                link={<Link to={item.to} />}
              >
                <Sidebar.ItemIcon>{item.icon}</Sidebar.ItemIcon>
                <Sidebar.ItemLabel>{item.label}</Sidebar.ItemLabel>
              </Sidebar.Item>
            ))}
          </Sidebar.Category>
        ))}
      </Sidebar.Nav>
    </Sidebar>
  );
}
