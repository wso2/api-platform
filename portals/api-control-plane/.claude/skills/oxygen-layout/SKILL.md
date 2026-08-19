---
name: oxygen-layout
description: Generate application layouts with AppShell, Header, and Sidebar. Use when creating app shells, navigation structures, or dashboard layouts.
---

# Generate Oxygen UI Layout

## Instructions

1. Read `.claude/oxygen-ui/patterns.md` for layout patterns
2. Use `AppShell` as the main wrapper
3. Configure `Header` with navigation
4. Set up `Sidebar` with menu items

## Critical Rules

- Always use `OxygenUIThemeProvider` at the root
- Use compound component pattern for `AppShell`, `Header`, `Sidebar`
- Import icons from `@wso2/oxygen-ui-icons-react`
- Use theme tokens for all styling

## Complete App Shell Layout

```tsx
import { useState } from 'react';
import {
  OxygenUIThemeProvider,
  OxygenTheme,
  AppShell,
  Header,
  Sidebar,
  Footer,
  Box,
  ColorSchemeToggle,
  UserMenu,
} from '@wso2/oxygen-ui';
import {
  HomeIcon,
  DashboardIcon,
  SettingsIcon,
  UsersIcon,
  FileIcon,
} from '@wso2/oxygen-ui-icons-react';

function App() {
  const [collapsed, setCollapsed] = useState(false);
  const [activeItem, setActiveItem] = useState('home');

  return (
    <OxygenUIThemeProvider theme={OxygenTheme}>
      <AppShell>
        <AppShell.Navbar>
          <Header>
            <Header.Toggle
              collapsed={collapsed}
              onToggle={() => setCollapsed(!collapsed)}
            />
            <Header.Brand>
              <Header.BrandLogo>
                <img src="/logo.svg" alt="Logo" height={32} />
              </Header.BrandLogo>
              <Header.BrandTitle>My Application</Header.BrandTitle>
            </Header.Brand>
            <Header.Spacer />
            <Header.Actions>
              <ColorSchemeToggle />
              <UserMenu
                user={{ name: 'John Doe', email: 'john@example.com' }}
                onSignOut={() => console.log('Sign out')}
              />
            </Header.Actions>
          </Header>
        </AppShell.Navbar>

        <AppShell.Sidebar>
          <Sidebar
            collapsed={collapsed}
            activeItem={activeItem}
            onSelect={setActiveItem}
          >
            <Sidebar.Nav>
              <Sidebar.Category>
                <Sidebar.CategoryLabel>Main</Sidebar.CategoryLabel>
                <Sidebar.Item id="home">
                  <Sidebar.ItemIcon><HomeIcon size={20} /></Sidebar.ItemIcon>
                  <Sidebar.ItemLabel>Home</Sidebar.ItemLabel>
                </Sidebar.Item>
                <Sidebar.Item id="dashboard">
                  <Sidebar.ItemIcon><DashboardIcon size={20} /></Sidebar.ItemIcon>
                  <Sidebar.ItemLabel>Dashboard</Sidebar.ItemLabel>
                </Sidebar.Item>
              </Sidebar.Category>

              <Sidebar.Category>
                <Sidebar.CategoryLabel>Management</Sidebar.CategoryLabel>
                <Sidebar.Item id="users">
                  <Sidebar.ItemIcon><UsersIcon size={20} /></Sidebar.ItemIcon>
                  <Sidebar.ItemLabel>Users</Sidebar.ItemLabel>
                  <Sidebar.ItemBadge>12</Sidebar.ItemBadge>
                </Sidebar.Item>
                <Sidebar.Item id="files">
                  <Sidebar.ItemIcon><FileIcon size={20} /></Sidebar.ItemIcon>
                  <Sidebar.ItemLabel>Files</Sidebar.ItemLabel>
                </Sidebar.Item>
              </Sidebar.Category>
            </Sidebar.Nav>

            <Sidebar.Footer>
              <Sidebar.Item id="settings">
                <Sidebar.ItemIcon><SettingsIcon size={20} /></Sidebar.ItemIcon>
                <Sidebar.ItemLabel>Settings</Sidebar.ItemLabel>
              </Sidebar.Item>
            </Sidebar.Footer>
          </Sidebar>
        </AppShell.Sidebar>

        <AppShell.Main>
          <Box sx={{ p: 3 }}>
            {/* Page content goes here */}
            <h1>Welcome</h1>
          </Box>
        </AppShell.Main>

        <AppShell.Footer>
          <Footer companyName="WSO2 LLC" />
        </AppShell.Footer>
      </AppShell>
    </OxygenUIThemeProvider>
  );
}

export default App;
```

## Header with Context Switchers

```tsx
<Header>
  <Header.Toggle collapsed={collapsed} onToggle={toggleSidebar} />
  <Header.Brand>
    <Header.BrandLogo><Logo /></Header.BrandLogo>
    <Header.BrandTitle>App Name</Header.BrandTitle>
  </Header.Brand>

  <Header.Switchers>
    <OrganizationSwitcher />
    <ProjectSwitcher />
  </Header.Switchers>

  <Header.Spacer />

  <Header.Actions>
    <SearchBar />
    <NotificationButton />
    <ColorSchemeToggle />
    <UserMenu user={user} />
  </Header.Actions>
</Header>
```

## Minimal Header (No Switchers)

```tsx
<Header minimal>
  <Header.Brand>
    <Header.BrandTitle>Simple App</Header.BrandTitle>
  </Header.Brand>
  <Header.Spacer />
  <Header.Actions>
    <ColorSchemeToggle />
  </Header.Actions>
</Header>
```

## Sidebar with Nested Items

```tsx
<Sidebar
  collapsed={collapsed}
  activeItem={activeItem}
  expandedMenus={expandedMenus}
  onSelect={setActiveItem}
  onToggleExpand={toggleExpand}
>
  <Sidebar.Nav>
    <Sidebar.Item id="parent" hasChildren>
      <Sidebar.ItemIcon><FolderIcon size={20} /></Sidebar.ItemIcon>
      <Sidebar.ItemLabel>Parent Item</Sidebar.ItemLabel>
    </Sidebar.Item>

    {expandedMenus['parent'] && (
      <>
        <Sidebar.Item id="child1" nested>
          <Sidebar.ItemLabel>Child 1</Sidebar.ItemLabel>
        </Sidebar.Item>
        <Sidebar.Item id="child2" nested>
          <Sidebar.ItemLabel>Child 2</Sidebar.ItemLabel>
        </Sidebar.Item>
      </>
    )}
  </Sidebar.Nav>
</Sidebar>
```

## Dashboard Layout with Stats

```tsx
import { Grid, Paper, Typography, Box } from '@wso2/oxygen-ui';

function DashboardContent() {
  return (
    <Box sx={{ p: 3 }}>
      <Typography variant="h4" gutterBottom>Dashboard</Typography>

      <Grid container spacing={3}>
        <Grid item xs={12} sm={6} md={3}>
          <Paper sx={{ p: 3 }}>
            <Typography color="text.secondary" variant="body2">Total Users</Typography>
            <Typography variant="h4" fontWeight="bold">12,345</Typography>
          </Paper>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Paper sx={{ p: 3 }}>
            <Typography color="text.secondary" variant="body2">Revenue</Typography>
            <Typography variant="h4" fontWeight="bold">$45,678</Typography>
          </Paper>
        </Grid>
        {/* More stats... */}
      </Grid>
    </Box>
  );
}
```
