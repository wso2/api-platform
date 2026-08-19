---
name: oxygen-migrate
description: Migrate existing MUI code to Oxygen UI patterns. Use when converting @mui/material imports, replacing MUI components, or updating to Oxygen UI conventions.
---

# Migrate to Oxygen UI

## Instructions

1. Read `.claude/oxygen-ui/migration.md` for full guide
2. Update imports from `@mui/*` to `@wso2/oxygen-ui`
3. Replace `lucide-react` icons with `@wso2/oxygen-ui-icons-react`
4. Add `OxygenUIThemeProvider` wrapper

## Migration Checklist

- [ ] Replace `@mui/material` imports with `@wso2/oxygen-ui`
- [ ] Replace `@mui/icons-material` or `lucide-react` with `@wso2/oxygen-ui-icons-react`
- [ ] Update MUI X imports to use namespaced exports
- [ ] Wrap root with `OxygenUIThemeProvider`
- [ ] Replace `ThemeProvider` with `OxygenUIThemeProvider`
- [ ] Update custom theme to use `OxygenThemeBase`

## Import Migration

### MUI Components

**Before:**
```tsx
import { Box, Stack, Button } from '@mui/material';
import Typography from '@mui/material/Typography';
import { styled } from '@mui/material/styles';
```

**After:**
```tsx
import { Box, Stack, Button, Typography, styled } from '@wso2/oxygen-ui';
```

### Icons

**Before (MUI Icons):**
```tsx
import SettingsIcon from '@mui/icons-material/Settings';
import HomeIcon from '@mui/icons-material/Home';
import DeleteIcon from '@mui/icons-material/Delete';
```

**Before (Lucide React):**
```tsx
import { Settings, Home, Trash2 } from 'lucide-react';
```

**After:**
```tsx
import { SettingsIcon, HomeIcon, TrashIcon } from '@wso2/oxygen-ui-icons-react';
```

### MUI X Components

**Before:**
```tsx
import { DataGrid } from '@mui/x-data-grid';
import { DatePicker } from '@mui/x-date-pickers';
import { BarChart } from '@mui/x-charts';
import { SimpleTreeView } from '@mui/x-tree-view';
```

**After:**
```tsx
import { DataGrid, DatePickers, TreeView } from '@wso2/oxygen-ui';

// Use as namespaces
<DataGrid.DataGrid rows={rows} columns={columns} />
<DatePickers.DatePicker value={date} onChange={setDate} />
<TreeView.SimpleTreeView items={items} />

// For Charts, use the separate package
import { BarChart } from '@wso2/oxygen-ui-charts-react';
<BarChart series={series} />
```

## Theme Provider Migration

**Before:**
```tsx
import { ThemeProvider, createTheme } from '@mui/material';

const theme = createTheme({
  palette: {
    primary: { main: '#1976d2' },
  },
});

<ThemeProvider theme={theme}>
  <App />
</ThemeProvider>
```

**After:**
```tsx
import { OxygenUIThemeProvider, OxygenTheme } from '@wso2/oxygen-ui';

// Use default theme
<OxygenUIThemeProvider theme={OxygenTheme}>
  <App />
</OxygenUIThemeProvider>

// Or with custom theme
import { extendTheme } from '@mui/material/styles';
import { OxygenThemeBase } from '@wso2/oxygen-ui';

const customTheme = extendTheme({
  ...OxygenThemeBase,
  colorSchemes: {
    light: {
      palette: { primary: { main: '#1976d2' } },
    },
  },
});

<OxygenUIThemeProvider theme={customTheme}>
  <App />
</OxygenUIThemeProvider>
```

## Layout Migration

**Before (Custom Layout):**
```tsx
import { AppBar, Toolbar, Drawer, Box } from '@mui/material';

<Box sx={{ display: 'flex' }}>
  <AppBar position="fixed">
    <Toolbar>
      <Typography>My App</Typography>
    </Toolbar>
  </AppBar>
  <Drawer variant="permanent">
    {/* Menu items */}
  </Drawer>
  <Box component="main" sx={{ flexGrow: 1, p: 3 }}>
    {/* Content */}
  </Box>
</Box>
```

**After (Oxygen AppShell):**
```tsx
import { AppShell, Header, Sidebar, Footer } from '@wso2/oxygen-ui';

<AppShell>
  <AppShell.Navbar>
    <Header>
      <Header.Brand>
        <Header.BrandTitle>My App</Header.BrandTitle>
      </Header.Brand>
    </Header>
  </AppShell.Navbar>

  <AppShell.Sidebar>
    <Sidebar>
      <Sidebar.Nav>
        {/* Menu items */}
      </Sidebar.Nav>
    </Sidebar>
  </AppShell.Sidebar>

  <AppShell.Main>
    {/* Content */}
  </AppShell.Main>

  <AppShell.Footer>
    <Footer companyName="WSO2 LLC" />
  </AppShell.Footer>
</AppShell>
```

## Table Migration

**Before (MUI Table):**
```tsx
import {
  Table,
  TableHead,
  TableBody,
  TableRow,
  TableCell,
  Paper,
} from '@mui/material';

<Paper>
  <Table>
    <TableHead>
      <TableRow>
        <TableCell>Name</TableCell>
        <TableCell>Status</TableCell>
      </TableRow>
    </TableHead>
    <TableBody>
      {data.map((row) => (
        <TableRow key={row.id}>
          <TableCell>{row.name}</TableCell>
          <TableCell>{row.status}</TableCell>
        </TableRow>
      ))}
    </TableBody>
  </Table>
</Paper>
```

**After (Oxygen ListingTable):**
```tsx
import { ListingTable } from '@wso2/oxygen-ui';

<ListingTable.Container>
  <ListingTable.Toolbar showSearch />
  <ListingTable>
    <ListingTable.Head>
      <ListingTable.Row>
        <ListingTable.Cell>Name</ListingTable.Cell>
        <ListingTable.Cell>Status</ListingTable.Cell>
      </ListingTable.Row>
    </ListingTable.Head>
    <ListingTable.Body>
      {data.map((row) => (
        <ListingTable.Row key={row.id}>
          <ListingTable.Cell>{row.name}</ListingTable.Cell>
          <ListingTable.Cell>{row.status}</ListingTable.Cell>
        </ListingTable.Row>
      ))}
    </ListingTable.Body>
    <ListingTable.Footer />
  </ListingTable>
</ListingTable.Container>
```

## Color Mode Migration

**Before:**
```tsx
import { useColorScheme } from '@mui/material';

const { mode, setMode } = useColorScheme();
```

**After:**
```tsx
import { ColorSchemeToggle, useTheme } from '@wso2/oxygen-ui';

// Simple toggle component
<ColorSchemeToggle />

// Check current mode
const theme = useTheme();
const isDark = theme.palette.mode === 'dark';
```

## ESLint Plugin Setup

After migration, add the ESLint plugin to prevent regressions:

```bash
pnpm add -D @wso2/eslint-plugin-oxygen-ui
```

```js
// eslint.config.js
import oxygenUIPlugin from '@wso2/eslint-plugin-oxygen-ui';

export default [
  oxygenUIPlugin.configs.recommended,
  // ... other configs
];
```

This will:
- Error on direct `@mui/material` imports
- Error on direct `lucide-react` imports
- Auto-fix imports with `eslint --fix`
