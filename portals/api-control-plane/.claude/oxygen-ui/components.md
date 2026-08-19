# Oxygen UI Component Reference

Complete API reference for custom Oxygen UI components. For MUI components, refer to [Material-UI documentation](https://mui.com/material-ui/).

## Table of Contents

- [OxygenUIThemeProvider](#oxygenuitthemeprovider)
- [ListingTable](#listingtable)
- [AppShell](#appshell)
- [Layout](#layout)
- [Header](#header)
- [Sidebar](#sidebar)
- [Footer](#footer)
- [UserMenu](#usermenu)
- [Form](#form)
- [NotificationPanel](#notificationpanel)
- [NotificationBanner](#notificationbanner)
- [ParticleBackground](#particlebackground)
- [Other Components](#other-components)
- [MUI X Namespaces](#mui-x-namespaces)
- [Hooks](#hooks)
- [Utilities](#utilities)

---

## OxygenUIThemeProvider

Theme provider that wraps your application. Required at the root level.

```tsx
import { OxygenUIThemeProvider, OxygenTheme } from '@wso2/oxygen-ui';
```

### Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `children` | `ReactNode` | required | App content |
| `theme` | `Theme` | `OxygenTheme` | Single theme (disables switching) |
| `themes` | `ThemeOption[]` | - | Array of themes (enables switching) |
| `initialTheme` | `string` | First theme key | Initial theme key |

### ThemeOption Type

```tsx
interface ThemeOption {
  key: string;    // Unique identifier
  label: string;  // Display name
  theme: Theme;   // MUI Theme object
}
```

### Basic Usage

```tsx
// Single theme (no switching)
<OxygenUIThemeProvider theme={OxygenTheme}>
  <App />
</OxygenUIThemeProvider>
```

### Theme Switching

```tsx
import {
  OxygenUIThemeProvider,
  OxygenTheme,
  AcrylicOrangeTheme,
  AcrylicPurpleTheme,
} from '@wso2/oxygen-ui';

const themes = [
  { key: 'default', label: 'Default', theme: OxygenTheme },
  { key: 'orange', label: 'Orange', theme: AcrylicOrangeTheme },
  { key: 'purple', label: 'Purple', theme: AcrylicPurpleTheme },
];

<OxygenUIThemeProvider themes={themes} initialTheme="default">
  <App />
</OxygenUIThemeProvider>
```

---

## ListingTable

Advanced data table with compound component pattern. Supports search, sort, pagination, selection, and bulk actions.

```tsx
import { ListingTable } from '@wso2/oxygen-ui';
```

### Sub-components

| Component | Description |
|-----------|-------------|
| `ListingTable.Provider` | Context provider for state management |
| `ListingTable.Container` | Paper wrapper with styling |
| `ListingTable.Toolbar` | Search, filters, and actions bar |
| `ListingTable.Head` | Table header wrapper |
| `ListingTable.Body` | Table body wrapper |
| `ListingTable.Footer` | Table footer (pagination) |
| `ListingTable.Row` | Table row |
| `ListingTable.Cell` | Table cell |
| `ListingTable.SortLabel` | Sortable column header |
| `ListingTable.DensityControl` | Density toggle button |
| `ListingTable.EmptyState` | Empty state display |
| `ListingTable.RowActions` | Row action buttons |
| `ListingTable.CellIcon` | Icon in cell |

### ListingTable Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `variant` | `'table' \| 'card'` | `'table'` | Display style |
| `density` | `'compact' \| 'standard' \| 'comfortable'` | `'standard'` | Row padding |
| `striped` | `boolean` | `false` | Alternating row colors |
| `bordered` | `boolean` | `false` | Cell borders |

### Provider Props (Context State)

```tsx
interface ListingTableProviderProps {
  // Search
  searchValue?: string;
  onSearchChange?: (value: string) => void;

  // Sort
  sortField?: string;
  sortDirection?: 'asc' | 'desc';
  onSortChange?: (field: string, direction: 'asc' | 'desc') => void;

  // Pagination
  page?: number;                    // 0-indexed
  rowsPerPage?: number;
  totalCount?: number;
  onPageChange?: (page: number) => void;
  onRowsPerPageChange?: (rowsPerPage: number) => void;

  // Selection
  selected?: readonly string[];
  onSelectionChange?: (selected: readonly string[]) => void;
  onSelectAll?: () => void;
  onClearSelection?: () => void;
  isSelected?: (id: string) => boolean;

  // Filters
  filters?: Record<string, unknown>;
  onFilterChange?: (key: string, value: unknown) => void;
  onClearFilters?: () => void;

  // Bulk Actions
  onBulkDelete?: (ids: readonly string[]) => void;
  onBulkAction?: (actionId: string, ids: readonly string[]) => void;

  // UI State
  density?: 'compact' | 'standard' | 'comfortable';
  onDensityChange?: (density: ListingTableDensity) => void;
  loading?: boolean;
}
```

### Toolbar Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `showSearch` | `boolean` | `false` | Show built-in search |
| `searchSlot` | `ReactNode` | - | Custom search component |
| `searchPlaceholder` | `string` | `'Search...'` | Search placeholder |
| `actions` | `ReactNode` | - | Right-side actions |
| `children` | `ReactNode` | - | Center content |

### Basic Table

```tsx
<ListingTable.Container>
  <ListingTable>
    <ListingTable.Head>
      <ListingTable.Row>
        <ListingTable.Cell>Name</ListingTable.Cell>
        <ListingTable.Cell>Status</ListingTable.Cell>
        <ListingTable.Cell>Actions</ListingTable.Cell>
      </ListingTable.Row>
    </ListingTable.Head>
    <ListingTable.Body>
      {data.map((row) => (
        <ListingTable.Row key={row.id}>
          <ListingTable.Cell>{row.name}</ListingTable.Cell>
          <ListingTable.Cell>{row.status}</ListingTable.Cell>
          <ListingTable.Cell>
            <ListingTable.RowActions
              actions={[
                { id: 'edit', label: 'Edit', onClick: () => edit(row.id) },
                { id: 'delete', label: 'Delete', onClick: () => delete(row.id) },
              ]}
            />
          </ListingTable.Cell>
        </ListingTable.Row>
      ))}
    </ListingTable.Body>
  </ListingTable>
</ListingTable.Container>
```

### Full-Featured Table with Context

```tsx
const [search, setSearch] = useState('');
const [sortField, setSortField] = useState('name');
const [sortDirection, setSortDirection] = useState<'asc' | 'desc'>('asc');
const [page, setPage] = useState(0);
const [rowsPerPage, setRowsPerPage] = useState(10);

<ListingTable.Provider
  searchValue={search}
  onSearchChange={setSearch}
  sortField={sortField}
  sortDirection={sortDirection}
  onSortChange={(field, dir) => {
    setSortField(field);
    setSortDirection(dir);
  }}
  page={page}
  rowsPerPage={rowsPerPage}
  totalCount={data.length}
  onPageChange={setPage}
  onRowsPerPageChange={setRowsPerPage}
>
  <ListingTable.Container>
    <ListingTable.Toolbar
      showSearch
      actions={<ListingTable.DensityControl />}
    />
    <ListingTable variant="card">
      <ListingTable.Head>
        <ListingTable.Row>
          <ListingTable.Cell>
            <ListingTable.SortLabel field="name">Name</ListingTable.SortLabel>
          </ListingTable.Cell>
          <ListingTable.Cell>
            <ListingTable.SortLabel field="status">Status</ListingTable.SortLabel>
          </ListingTable.Cell>
        </ListingTable.Row>
      </ListingTable.Head>
      <ListingTable.Body>
        {paginatedData.map((row) => (
          <ListingTable.Row key={row.id}>
            <ListingTable.Cell>{row.name}</ListingTable.Cell>
            <ListingTable.Cell>{row.status}</ListingTable.Cell>
          </ListingTable.Row>
        ))}
      </ListingTable.Body>
      <ListingTable.Footer />
    </ListingTable>
  </ListingTable.Container>
</ListingTable.Provider>
```

---

## AppShell

Application layout wrapper using compound component pattern.

```tsx
import { AppShell } from '@wso2/oxygen-ui';
```

### Sub-components

| Component | Description |
|-----------|-------------|
| `AppShell.Navbar` | Top navigation area (header) |
| `AppShell.Sidebar` | Left sidebar area |
| `AppShell.Main` | Main content area |
| `AppShell.Footer` | Bottom footer area |
| `AppShell.NotificationPanel` | Overlay notification panel |

### Usage

```tsx
<AppShell>
  <AppShell.Navbar>
    <Header>...</Header>
  </AppShell.Navbar>

  <AppShell.Sidebar>
    <Sidebar collapsed={collapsed}>...</Sidebar>
  </AppShell.Sidebar>

  <AppShell.Main>
    <Outlet />  {/* React Router outlet */}
  </AppShell.Main>

  <AppShell.Footer>
    <Footer companyName="WSO2 LLC" />
  </AppShell.Footer>

  <AppShell.NotificationPanel>
    <NotificationPanel open={open} onClose={onClose}>
      ...
    </NotificationPanel>
  </AppShell.NotificationPanel>
</AppShell>
```

---

## Layout

Lightweight layout compound component for building flex-based page structures.

```tsx
import { Layout } from '@wso2/oxygen-ui';
```

### Sub-components

| Component | Description |
|-----------|-------------|
| `Layout.Sidebar` | Sidebar area |
| `Layout.Navbar` | Top navigation area |
| `Layout.Content` | Main content area |
| `Layout.Header` | Header area |

### Props

Extends MUI `BoxProps`. The root component renders a flex row container.

### Usage

```tsx
<Layout>
  <Layout.Navbar>
    <Header>...</Header>
  </Layout.Navbar>
  <Layout.Sidebar>
    <Sidebar>...</Sidebar>
  </Layout.Sidebar>
  <Layout.Content>
    {/* Page content */}
  </Layout.Content>
</Layout>
```

---

## Header

Top navigation bar compound component.

```tsx
import { Header } from '@wso2/oxygen-ui';
```

### Sub-components

| Component | Description |
|-----------|-------------|
| `Header.Toggle` | Sidebar collapse toggle button |
| `Header.Brand` | Logo and title container |
| `Header.BrandLogo` | Logo image/icon |
| `Header.BrandTitle` | Application title |
| `Header.Switchers` | Context switcher container |
| `Header.Actions` | Right-side action buttons |
| `Header.Spacer` | Flexible spacer |

### Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `minimal` | `boolean` | `false` | Hide switchers section |
| `sx` | `SxProps` | - | Custom styles |

### Usage

```tsx
<Header>
  <Header.Toggle collapsed={collapsed} onToggle={() => setCollapsed(!collapsed)} />

  <Header.Brand>
    <Header.BrandLogo>
      <img src="/logo.svg" alt="Logo" />
    </Header.BrandLogo>
    <Header.BrandTitle>My Application</Header.BrandTitle>
  </Header.Brand>

  <Header.Switchers>
    <OrganizationSwitcher />
    <ProjectSwitcher />
  </Header.Switchers>

  <Header.Spacer />

  <Header.Actions>
    <ColorSchemeToggle />
    <NotificationButton />
    <UserMenu user={user} />
  </Header.Actions>
</Header>
```

---

## Sidebar

Collapsible navigation sidebar compound component.

```tsx
import { Sidebar, SIDEBAR_WIDTH, COLLAPSED_SIDEBAR_WIDTH } from '@wso2/oxygen-ui';
```

### Sub-components

| Component | Description |
|-----------|-------------|
| `Sidebar.Nav` | Scrollable navigation container |
| `Sidebar.Category` | Menu category group |
| `Sidebar.CategoryLabel` | Category heading text |
| `Sidebar.Item` | Navigation menu item |
| `Sidebar.ItemIcon` | Item icon |
| `Sidebar.ItemLabel` | Item text label |
| `Sidebar.ItemBadge` | Badge indicator |
| `Sidebar.Footer` | Fixed bottom section |
| `Sidebar.User` | User profile section |
| `Sidebar.UserAvatar` | User avatar |
| `Sidebar.UserName` | User name text |
| `Sidebar.UserEmail` | User email text |

### Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `collapsed` | `boolean` | `false` | Collapsed state |
| `activeItem` | `string` | - | Active menu item ID |
| `expandedMenus` | `Record<string, boolean>` | `{}` | Expanded submenu map |
| `onSelect` | `(id: string) => void` | - | Item selection handler |
| `onToggleExpand` | `(id: string) => void` | - | Submenu toggle handler |
| `width` | `number` | `250` | Expanded width (px) |
| `collapsedWidth` | `number` | `64` | Collapsed width (px) |

### Usage

```tsx
import { HomeIcon, SettingsIcon } from '@wso2/oxygen-ui-icons-react';

<Sidebar
  collapsed={collapsed}
  activeItem={activeId}
  expandedMenus={expandedMenus}
  onSelect={setActiveId}
  onToggleExpand={(id) => toggleExpand(id)}
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
        <Sidebar.ItemBadge>5</Sidebar.ItemBadge>
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
```

---

## Form

Form components namespace for building forms with card-based layouts and wizards.

```tsx
import { Form } from '@wso2/oxygen-ui';
```

### Components

| Component | Description |
|-----------|-------------|
| `Form.CardButton` | Clickable card button for selection |
| `Form.DisappearingCardButtonContent` | Content that fades when card is selected |
| `Form.CardHeader` | Card header section |
| `Form.CardContent` | Card content section |
| `Form.CardActions` | Card action buttons section |
| `Form.CardMedia` | Card media/image section |
| `Form.Stack` | Form field stack layout |
| `Form.Header` | Section header text |
| `Form.Subheader` | Section subheader text |
| `Form.Body` | Section body text |
| `Form.Section` | Form section with heading |
| `Form.Wizard` | Multi-step form wizard |
| `Form.ElementWrapper` | Wrapper for form elements |

### Wizard Props

```tsx
interface WizardStep {
  id: string;
  label: string;
  content: ReactNode;
  optional?: boolean;
}

interface WizardProps {
  steps: WizardStep[];
  activeStep: number;
  onStepChange: (step: number) => void;
  onComplete?: () => void;
}
```

### Usage

```tsx
// Card-based selection form
<Form.Stack spacing={2}>
  <Form.CardButton selected={selected === 'option1'} onClick={() => setSelected('option1')}>
    <Form.CardHeader title="Option 1" />
    <Form.CardContent>
      <Form.Body>Description of option 1</Form.Body>
    </Form.CardContent>
  </Form.CardButton>

  <Form.CardButton selected={selected === 'option2'} onClick={() => setSelected('option2')}>
    <Form.CardHeader title="Option 2" />
    <Form.CardContent>
      <Form.Body>Description of option 2</Form.Body>
    </Form.CardContent>
  </Form.CardButton>
</Form.Stack>

// Section with typography
<Form.Section>
  <Form.Header>User Details</Form.Header>
  <Form.Subheader>Enter your information</Form.Subheader>
  <Form.Body>Additional context about the form section.</Form.Body>
</Form.Section>
```

---

## NotificationPanel

Slide-out notification panel with tabs and compound component pattern.

```tsx
import { NotificationPanel } from '@wso2/oxygen-ui';
```

### Sub-components

| Component | Description |
|-----------|-------------|
| `NotificationPanel.Header` | Panel header container |
| `NotificationPanel.HeaderIcon` | Header icon |
| `NotificationPanel.HeaderTitle` | Header title text |
| `NotificationPanel.HeaderBadge` | Unread count badge |
| `NotificationPanel.HeaderClose` | Close button |
| `NotificationPanel.Tabs` | Tab navigation |
| `NotificationPanel.Actions` | Action buttons container |
| `NotificationPanel.List` | Notification list container |
| `NotificationPanel.Item` | Individual notification item |
| `NotificationPanel.ItemAvatar` | Item avatar |
| `NotificationPanel.ItemTitle` | Item title |
| `NotificationPanel.ItemMessage` | Item message text |
| `NotificationPanel.ItemTimestamp` | Item timestamp |
| `NotificationPanel.ItemAction` | Item action button |
| `NotificationPanel.EmptyState` | Empty state display |

### Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `open` | `boolean` | required | Panel visibility |
| `onClose` | `() => void` | required | Close handler |

### Tabs Props

```tsx
interface NotificationTabConfig {
  id: string;
  label: string;
  badge?: number;
}

interface NotificationTabsProps {
  tabs: NotificationTabConfig[];
  activeTab: string;
  onTabChange: (tabId: string) => void;
}
```

### Usage

```tsx
<NotificationPanel open={open} onClose={() => setOpen(false)}>
  <NotificationPanel.Header>
    <NotificationPanel.HeaderIcon />
    <NotificationPanel.HeaderTitle>Notifications</NotificationPanel.HeaderTitle>
    <NotificationPanel.HeaderBadge count={unreadCount} />
    <NotificationPanel.HeaderClose />
  </NotificationPanel.Header>

  <NotificationPanel.Tabs
    tabs={[
      { id: 'all', label: 'All' },
      { id: 'unread', label: 'Unread', badge: 3 },
    ]}
    activeTab={activeTab}
    onTabChange={setActiveTab}
  />

  <NotificationPanel.Actions>
    <Button onClick={markAllRead}>Mark all read</Button>
  </NotificationPanel.Actions>

  <NotificationPanel.List>
    {notifications.map((n) => (
      <NotificationPanel.Item key={n.id}>
        <NotificationPanel.ItemAvatar src={n.avatar} />
        <NotificationPanel.ItemTitle>{n.title}</NotificationPanel.ItemTitle>
        <NotificationPanel.ItemMessage>{n.message}</NotificationPanel.ItemMessage>
        <NotificationPanel.ItemTimestamp>{n.timestamp}</NotificationPanel.ItemTimestamp>
        <NotificationPanel.ItemAction onClick={() => handleAction(n.id)}>
          View
        </NotificationPanel.ItemAction>
      </NotificationPanel.Item>
    ))}
  </NotificationPanel.List>

  {notifications.length === 0 && (
    <NotificationPanel.EmptyState message="No notifications" />
  )}
</NotificationPanel>
```

### Hooks

```tsx
import { useNotificationPanel } from '@wso2/oxygen-ui';

// Access panel context inside NotificationPanel
const { open, onClose } = useNotificationPanel();
```

---

## SearchBar

Search input components with optional advanced filtering.

```tsx
import { SearchBar, SearchBarWithAdvancedFilter } from '@wso2/oxygen-ui';
```

### SearchBar Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `value` | `string` | - | Search input value |
| `onChange` | `(value: string) => void` | - | Change handler |
| `placeholder` | `string` | `'Search...'` | Placeholder text |

### SearchBarWithAdvancedFilter Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `value` | `string` | - | Search input value |
| `onChange` | `(value: string) => void` | - | Change handler |
| `advancedFilters` | `AdvancedFilterState` | - | Filter state |
| `onAdvancedFilterChange` | `(filters: AdvancedFilterState) => void` | - | Filter change handler |

### Usage

```tsx
// Basic search
<SearchBar
  value={search}
  onChange={setSearch}
  placeholder="Search users..."
/>

// With advanced filters
<SearchBarWithAdvancedFilter
  value={search}
  onChange={setSearch}
  advancedFilters={filters}
  onAdvancedFilterChange={setFilters}
/>
```

---

## StatCard

Statistics display card for dashboards.

```tsx
import { StatCard } from '@wso2/oxygen-ui';
```

### Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `title` | `string` | required | Stat label |
| `value` | `string \| number` | required | Stat value |
| `icon` | `ReactNode` | - | Optional icon |
| `change` | `number` | - | Percentage change |
| `trend` | `'up' \| 'down'` | - | Trend direction |

### Usage

```tsx
<StatCard
  title="Total Users"
  value="12,345"
  change={12.5}
  trend="up"
/>
```

---

## PageContent

Page content wrapper with consistent padding and max-width.

```tsx
import { PageContent } from '@wso2/oxygen-ui';
```

### Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `children` | `ReactNode` | required | Page content |
| `maxWidth` | `string \| number` | - | Maximum width |

### Usage

```tsx
<PageContent>
  <PageTitle>Dashboard</PageTitle>
  {/* Page content */}
</PageContent>
```

---

## ComplexSelect

Enhanced select component with search and multi-select capabilities.

```tsx
import { ComplexSelect } from '@wso2/oxygen-ui';
```

### Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `options` | `Option[]` | required | Select options |
| `value` | `string \| string[]` | - | Selected value(s) |
| `onChange` | `(value) => void` | - | Change handler |
| `multiple` | `boolean` | `false` | Enable multi-select |
| `searchable` | `boolean` | `false` | Enable search |

### Usage

```tsx
<ComplexSelect
  options={[
    { value: 'us', label: 'United States' },
    { value: 'uk', label: 'United Kingdom' },
    { value: 'ca', label: 'Canada' },
  ]}
  value={country}
  onChange={setCountry}
  searchable
/>
```

---

## NotificationBanner

Dismissible system alert banner for top-level announcements.

```tsx
import { NotificationBanner } from '@wso2/oxygen-ui';
```

### Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `visible` | `boolean` | `true` | Whether the banner is visible |
| `severity` | `'info' \| 'warning' \| 'error' \| 'success'` | `'info'` | Banner severity/type |
| `title` | `string` | - | Optional banner title |
| `message` | `string` | required | Banner message text |
| `actionLabel` | `string` | - | Optional action button label |
| `onAction` | `() => void` | - | Action button callback |
| `onDismiss` | `() => void` | - | Dismiss callback |
| `sx` | `SxProps` | - | Custom styles |

### Usage

```tsx
<NotificationBanner
  visible={showBanner}
  severity="info"
  title="Scheduled Maintenance"
  message="The system will be undergoing maintenance on Dec 20th."
  actionLabel="Learn More"
  onAction={() => openMaintenanceInfo()}
  onDismiss={() => setShowBanner(false)}
/>
```

---

## ParticleBackground

Animated particle canvas background with mouse interaction. Adapts to light/dark color scheme.

```tsx
import { ParticleBackground } from '@wso2/oxygen-ui';
```

### Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `opacity` | `number` | `0.5` | Canvas opacity |
| `baseDensity` | `number` | `0.12` | Particles per 10,000 px² |
| `maxSpeed` | `number` | `0.3` | Maximum speed in px per frame |
| `radius` | `[number, number]` | `[2.0, 3.2]` | Min and max particle radius |
| `linkDist` | `number` | `210` | Maximum distance to draw lines between particles |
| `linkAlpha` | `number` | `0.2` | Base opacity of the lines |
| `mouseInfluence` | `number` | `110` | Radius of mouse influence |
| `repelStrength` | `number` | `0.35` | Strength of the repel effect |
| `clickBurst` | `number` | `120` | Impulse on click |

### Usage

```tsx
// Fixed background behind content
<ParticleBackground opacity={0.3} />

// Higher density with stronger interaction
<ParticleBackground
  baseDensity={0.2}
  mouseInfluence={150}
  repelStrength={0.5}
/>
```

---

## Other Components

### ColorSchemeToggle

Dark/light mode toggle button.

```tsx
import { ColorSchemeToggle } from '@wso2/oxygen-ui';

<ColorSchemeToggle />
```

### ThemeSwitcher

Theme selection UI component.

```tsx
import { ThemeSwitcher } from '@wso2/oxygen-ui';

<ThemeSwitcher />  // Requires OxygenUIThemeProvider with themes
```

---

## Footer

Application footer compound component with composition pattern.

```tsx
import { Footer } from '@wso2/oxygen-ui';
```

### Sub-components

| Component | Description |
|-----------|-------------|
| `Footer.Copyright` | Copyright text display |
| `Footer.Version` | Version number display (monospace, muted) |
| `Footer.Link` | Footer link (auto-placed in right section) |
| `Footer.Divider` | Visual divider between footer items |

### Footer Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `children` | `ReactNode` | required | Footer sub-components |
| `sx` | `SxProps` | - | Custom styles |

### Usage

```tsx
<Footer>
  <Footer.Copyright>© 2025 WSO2 LLC. All rights reserved.</Footer.Copyright>
  <Footer.Divider />
  <Footer.Version>v2.1.0</Footer.Version>
  <Footer.Link href="/terms">Terms & Conditions</Footer.Link>
  <Footer.Link href="/privacy">Privacy Policy</Footer.Link>
</Footer>
```

> **Note:** `Footer.Copyright`, `Footer.Version`, and `Footer.Divider` are placed in the left section. `Footer.Link` elements are placed in the right section. Layout is responsive (stacks on mobile).

---

## UserMenu

User profile dropdown menu compound component.

```tsx
import { UserMenu } from '@wso2/oxygen-ui';
```

### Sub-components

| Component | Description |
|-----------|-------------|
| `UserMenu.Trigger` | Avatar button that opens the menu |
| `UserMenu.Header` | User info section (name, email, avatar, role badge) |
| `UserMenu.Item` | Menu item with icon, label, and optional badge |
| `UserMenu.Logout` | Destructive logout menu item |
| `UserMenu.Divider` | Menu divider |

### UserMenu Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `children` | `ReactNode` | required | Menu sub-components |

### UserMenuTrigger Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `name` | `string` | required | User display name |
| `avatar` | `string \| null` | - | Avatar image URL |
| `showName` | `boolean` | `false` | Show name text after avatar |

### UserMenuHeader Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `name` | `string` | required | User display name |
| `email` | `string` | required | User email address |
| `avatar` | `string \| null` | - | Avatar image URL |
| `role` | `string` | - | Role badge text (e.g., "Pro", "Admin") |

### UserMenuItem Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `icon` | `ReactNode` | - | Left-side icon |
| `label` | `string` | required | Item label text |
| `badge` | `string` | - | Right-side badge/chip |
| `onClick` | `() => void` | - | Click handler |

### UserMenuLogout Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `icon` | `ReactNode` | - | Left-side icon |
| `label` | `string` | `'Log out'` | Item label text |
| `onClick` | `() => void` | - | Click handler |

### UserMenuUser Type

```tsx
interface UserMenuUser {
  name: string;
  email: string;
  avatar?: string;
  role?: string;
}
```

### Usage

```tsx
import { UserMenu } from '@wso2/oxygen-ui';
import { UserIcon, SettingsIcon, LogOutIcon } from '@wso2/oxygen-ui-icons-react';

<UserMenu>
  <UserMenu.Trigger name="John Doe" avatar="/avatar.png" />
  <UserMenu.Header name="John Doe" email="john@example.com" role="Admin" />
  <UserMenu.Item icon={<UserIcon size={20} />} label="Profile" onClick={() => {}} />
  <UserMenu.Item icon={<SettingsIcon size={20} />} label="Settings" onClick={() => {}} />
  <UserMenu.Divider />
  <UserMenu.Logout icon={<LogOutIcon size={20} />} onClick={() => signOut()} />
</UserMenu>
```

### PageTitle

Page header with avatar and breadcrumb support.

```tsx
import { PageTitle } from '@wso2/oxygen-ui';

<PageTitle>
  <PageTitle.Avatar><UserIcon /></PageTitle.Avatar>
  <PageTitle.Header>Dashboard</PageTitle.Header>
  <PageTitle.SubHeader>Overview of your projects</PageTitle.SubHeader>
</PageTitle>
```

### CodeBlock

Syntax-highlighted code display.

```tsx
import { CodeBlock } from '@wso2/oxygen-ui';

<CodeBlock language="typescript">
  {`const greeting = "Hello, World!";`}
</CodeBlock>
```

---

## MUI X Namespaces

MUI X components are exported as namespaces to avoid naming conflicts.

```tsx
import { DataGrid, DatePickers, TreeView, AdapterDateFns } from '@wso2/oxygen-ui';
```

> **Note:** Charts are available in a separate package `@wso2/oxygen-ui-charts-react` built on Recharts.

### DataGrid

```tsx
<DataGrid.DataGrid
  rows={rows}
  columns={columns}
  pageSize={10}
  checkboxSelection
/>

<DataGrid.DataGridPro
  rows={rows}
  columns={columns}
  treeData
/>
```

### DatePickers

```tsx
<DatePickers.LocalizationProvider dateAdapter={AdapterDateFns}>
  <DatePickers.DatePicker
    value={date}
    onChange={setDate}
    label="Select date"
  />

  <DatePickers.DateTimePicker
    value={dateTime}
    onChange={setDateTime}
  />

  <DatePickers.DateRangePicker
    value={range}
    onChange={setRange}
  />
</DatePickers.LocalizationProvider>
```

### TreeView

```tsx
<TreeView.SimpleTreeView>
  <TreeView.TreeItem itemId="1" label="Parent">
    <TreeView.TreeItem itemId="2" label="Child 1" />
    <TreeView.TreeItem itemId="3" label="Child 2" />
  </TreeView.TreeItem>
</TreeView.SimpleTreeView>

<TreeView.RichTreeView
  items={treeData}
  multiSelect
/>
```

---

## Hooks

### useThemeSwitcher

Access theme switching context.

```tsx
import { useThemeSwitcher } from '@wso2/oxygen-ui';

function ThemeSelector() {
  const { currentTheme, themes, setTheme, isActive } = useThemeSwitcher();

  return (
    <Select value={currentTheme} onChange={(e) => setTheme(e.target.value)}>
      {themes.map((t) => (
        <MenuItem key={t.key} value={t.key}>{t.label}</MenuItem>
      ))}
    </Select>
  );
}
```

### useThemeContent

Get content based on current color scheme (light/dark).

```tsx
import { useThemeContent } from '@wso2/oxygen-ui';

function AdaptiveContent() {
  const content = useThemeContent({
    light: 'Light mode content',
    dark: 'Dark mode content',
  });

  return <Typography>{content}</Typography>;
}
```

### useAppShell

Manage AppShell state (sidebar collapsed, notifications open, etc.).

```tsx
import { useAppShell } from '@wso2/oxygen-ui';

function MyComponent() {
  const {
    sidebarCollapsed,
    setSidebarCollapsed,
    notificationPanelOpen,
    setNotificationPanelOpen,
  } = useAppShell();

  return (
    <Button onClick={() => setSidebarCollapsed(!sidebarCollapsed)}>
      Toggle Sidebar
    </Button>
  );
}
```

### useNotifications

Manage notification state and actions.

```tsx
import { useNotifications } from '@wso2/oxygen-ui';

function NotificationManager() {
  const {
    notifications,
    unreadCount,
    addNotification,
    markAsRead,
    markAllAsRead,
    removeNotification,
  } = useNotifications();

  return (
    <Badge badgeContent={unreadCount}>
      <NotificationIcon />
    </Badge>
  );
}
```

### useListingTable

Access ListingTable context (returns null outside Provider).

```tsx
import { useListingTable } from '@wso2/oxygen-ui';

function CustomFilter() {
  const context = useListingTable();
  if (!context) return null;

  return (
    <TextField
      value={context.searchValue}
      onChange={(e) => context.onSearchChange?.(e.target.value)}
    />
  );
}
```

### useListingTableRequired

Access ListingTable context (throws if outside Provider).

```tsx
import { useListingTableRequired } from '@wso2/oxygen-ui';

function TableControls() {
  const { density, onDensityChange } = useListingTableRequired();
  // ...
}
```

### Component Context Hooks

```tsx
// Header context
import { useHeader } from '@wso2/oxygen-ui';
const headerContext = useHeader();

// Sidebar context
import { useSidebar } from '@wso2/oxygen-ui';
const sidebarContext = useSidebar();

// NotificationPanel context
import { useNotificationPanel } from '@wso2/oxygen-ui';
const panelContext = useNotificationPanel();
```

---

## Utilities

### formatRelativeTime

Format a date as a relative time string.

```tsx
import { formatRelativeTime } from '@wso2/oxygen-ui';

formatRelativeTime(new Date(Date.now() - 1000 * 60 * 5));
// Returns: "5 minutes ago"

formatRelativeTime(new Date(Date.now() - 1000 * 60 * 60 * 2));
// Returns: "2 hours ago"
```

Returns: `"Just now"` | `"X minute(s) ago"` | `"X hour(s) ago"` | `"X day(s) ago"` | locale date string (after 7 days).

### pxToRem

Convert pixel values to rem based on 14px base font size.

```tsx
import { pxToRem } from '@wso2/oxygen-ui';

pxToRem(14); // "1rem"
pxToRem(28); // "2rem"
pxToRem(7);  // "0.5rem"
```

---

## Type Exports

Import types for TypeScript usage:

```tsx
import type {
  // Theme
  ThemeOption,
  ThemeSwitcherContextValue,

  // ListingTable
  ListingTableProps,
  ListingTableContainerProps,
  ListingTableHeadProps,
  ListingTableBodyProps,
  ListingTableFooterProps,
  ListingTableRowProps,
  ListingTableCellProps,
  ListingTableProviderProps,
  ListingTableContextValue,
  ListingTableToolbarProps,
  ListingTableEmptyStateProps,
  ListingTableDensityControlProps,
  ListingTableSortLabelProps,
  ListingTableRowActionsProps,
  ListingTableCellIconProps,
  ListingTableDensity,
  ListingTableVariant,
  ListingTableSortDirection,
  ListingSortDirection,

  // AppShell
  AppShellProps,
  AppShellNavbarProps,
  AppShellSidebarProps,
  AppShellMainProps,
  AppShellFooterProps,
  AppShellNotificationPanelProps,

  // Header
  HeaderProps,
  HeaderSpacerProps,
  HeaderToggleProps,
  HeaderBrandProps,
  HeaderBrandLogoProps,
  HeaderBrandTitleProps,
  HeaderSwitchersProps,
  HeaderActionsProps,
  HeaderContextValue,

  // Sidebar
  SidebarProps,
  SidebarNavProps,
  SidebarCategoryProps,
  SidebarCategoryLabelProps,
  SidebarItemProps,
  SidebarItemIconProps,
  SidebarItemLabelProps,
  SidebarItemBadgeProps,
  SidebarFooterProps,
  SidebarUserProps,
  SidebarUserAvatarProps,
  SidebarUserNameProps,
  SidebarUserEmailProps,
  SidebarContextValue,
  SidebarItemContextValue,

  // NotificationPanel
  NotificationPanelProps,
  NotificationHeaderProps,
  NotificationHeaderIconProps,
  NotificationHeaderTitleProps,
  NotificationHeaderBadgeProps,
  NotificationHeaderCloseProps,
  NotificationTabsProps,
  NotificationTabConfig,
  NotificationActionsProps,
  NotificationListProps,
  NotificationItemProps,
  NotificationItemAvatarProps,
  NotificationItemTitleProps,
  NotificationItemMessageProps,
  NotificationItemTimestampProps,
  NotificationItemActionProps,
  NotificationItemContextValue,
  NotificationEmptyStateProps,
  NotificationPanelContextValue,
  NotificationType,
  NotificationTypeProps,

  // Footer
  FooterProps,
  FooterCopyrightProps,
  FooterVersionProps,
  FooterLinkProps,

  // UserMenu
  UserMenuProps,
  UserMenuUser,
  UserMenuTriggerProps,
  UserMenuHeaderProps,
  UserMenuItemProps,
  UserMenuLogoutProps,

  // NotificationBanner
  NotificationBannerProps,

  // ParticleBackground
  ParticleBackgroundProps,

  // Other components
  CodeBlockProps,
  ColorSchemeImageProps,
  ColorSchemeImageAttribute,
  ComplexSelectProps,
  PageContentProps,
  PageTitleProps,
  PageTitleAvatarProps,
  PageTitleHeaderProps,
  PageTitleSubHeaderProps,
  PageTitleLinkProps,
  StatCardProps,
  ThemeSwitcherProps,
  ThemeSelectProps,
  SearchBarProps,
  SearchBarWithAdvancedFilterProps,
  AdvancedFilterState,

  // Form
  WizardProps,
  WizardStep,

  // Hooks
  AppShellState,
  AppShellActions,
  UseAppShellOptions,
  UseAppShellReturn,
  NotificationItem,
  NotificationActions,
  UseNotificationsOptions,
  UseNotificationsReturn,
} from '@wso2/oxygen-ui';
```
