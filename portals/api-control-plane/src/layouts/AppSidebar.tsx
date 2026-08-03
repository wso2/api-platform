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
