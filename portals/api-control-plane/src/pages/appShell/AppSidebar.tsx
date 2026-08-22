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

import { useCallback, useEffect, useMemo, useState } from 'react';
import { Sidebar, useAppShell } from '@wso2/oxygen-ui';
import { Link, useNavigate } from 'react-router-dom';

import { useNavigationClusters } from '../../navigation/useNavigationItems';
import type { NavigationItem } from '../../navigation/navigationTypes';

/** Every item and sub-item, depth-first — the order the sidebar renders them. */
const flatten = (items: NavigationItem[]): NavigationItem[] =>
  items.flatMap((item) => [item, ...flatten(item.children ?? [])]);

export function AppSidebar() {
  const clusters = useNavigationClusters();
  const { state } = useAppShell();
  const navigate = useNavigate();
  const [expandedMenus, setExpandedMenus] = useState<Record<string, boolean>>(
    {}
  );

  const items = useMemo(() => clusters.flat(), [clusters]);
  const allItems = useMemo(() => flatten(items), [items]);
  const activeItem = allItems.find((item) => item.isActive)?.id;

  // A parent whose child is active opens itself, so landing on a submenu page —
  // by deep link, by browser Back, or after a ScopeGate resolves — shows where
  // you are. Only this parent is forced; anything the user opened by hand stays
  // as they left it.
  const activeParent = items.find((item) =>
    item.children?.some((child) => child.id === activeItem)
  )?.id;
  useEffect(() => {
    if (!activeParent) return;
    setExpandedMenus((previous) =>
      previous[activeParent] ? previous : { ...previous, [activeParent]: true }
    );
  }, [activeParent]);

  const toggleMenu = useCallback((id: string) => {
    setExpandedMenus((previous) => ({ ...previous, [id]: !previous[id] }));
  }, []);

  // Collapsed to the icon rail, Oxygen renders a submenu as a popover whose
  // entries call `onSelect` and ignore each child's `link` — so without this,
  // sub-items would be unreachable in the collapsed sidebar. Navigating to the
  // item's own target is idempotent for the expanded case, where the `link` has
  // already taken the user there.
  const selectItem = useCallback(
    (id: string) => {
      const target = allItems.find((item) => item.id === id)?.to;
      if (target) navigate(target);
    },
    [allItems, navigate]
  );

  const renderItem = (item: NavigationItem) => {
    const children = item.children ?? [];
    return (
      <Sidebar.Item
        key={item.id}
        id={item.id}
        // A parent showing children is a disclosure, not a link: Oxygen turns a
        // click into `onToggleExpand` as soon as an item has nested children, so
        // linking it too would navigate *and* expand. Out of scope the children
        // are withheld and this is an ordinary link into the first child's page,
        // where `ScopeGate` asks for the missing API.
        link={children.length ? undefined : <Link to={item.to} />}
      >
        <Sidebar.ItemIcon>{item.icon}</Sidebar.ItemIcon>
        <Sidebar.ItemLabel>{item.label}</Sidebar.ItemLabel>
        {children.map(renderItem)}
      </Sidebar.Item>
    );
  };

  return (
    <Sidebar
      activeItem={activeItem}
      collapsed={state.sidebarCollapsed}
      expandedMenus={expandedMenus}
      onSelect={selectItem}
      onToggleExpand={toggleMenu}
    >
      {/*
        `showDividers` and label-less categories: the sidebar separates its
        clusters with a rule rather than a heading, because an item is no longer
        tied to one scope — Overview follows you from organization to project to
        API, so no single section title fits it.
      */}
      <Sidebar.Nav showDividers>
        {clusters.map((clusterItems) => (
          <Sidebar.Category key={clusterItems[0].id}>
            {clusterItems.map(renderItem)}
          </Sidebar.Category>
        ))}
      </Sidebar.Nav>
    </Sidebar>
  );
}
