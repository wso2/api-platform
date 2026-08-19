---
name: oxygen-component
description: Generate Oxygen UI React components following best practices. Use when creating new components, data tables, cards, or UI elements with the Oxygen UI library.
---

# Generate Oxygen UI Component

## Instructions

1. Read `.claude/oxygen-ui/CLAUDE.md` for critical rules
2. Read `.claude/oxygen-ui/components.md` for API reference
3. Generate component using Oxygen UI patterns

## Critical Rules

- Import ALL components from `@wso2/oxygen-ui` (never from `@mui/material`)
- Import icons from `@wso2/oxygen-ui-icons-react`
- Use theme tokens via `sx` prop (e.g., `p: 2`, `bgcolor: 'background.paper'`)
- Never hardcode colors or spacing values
- For data tables, prefer `ListingTable` over MUI's `Table`
- For layouts, use `AppShell`, `Header`, `Sidebar`

## Component Template

```tsx
import { Box, Typography, Button } from '@wso2/oxygen-ui';
import { IconName } from '@wso2/oxygen-ui-icons-react';

interface MyComponentProps {
  // Define props
}

function MyComponent({ ...props }: MyComponentProps) {
  return (
    <Box sx={{ p: 2, bgcolor: 'background.paper' }}>
      {/* Component content */}
    </Box>
  );
}

export default MyComponent;
```

## Common Patterns

### Data Table Component

```tsx
import { ListingTable, Chip, IconButton } from '@wso2/oxygen-ui';
import { EditIcon, TrashIcon } from '@wso2/oxygen-ui-icons-react';

<ListingTable.Container>
  <ListingTable.Toolbar showSearch actions={<Button>Add Item</Button>} />
  <ListingTable variant="card">
    <ListingTable.Head>
      <ListingTable.Row>
        <ListingTable.Cell>Name</ListingTable.Cell>
        <ListingTable.Cell>Status</ListingTable.Cell>
        <ListingTable.Cell align="right">Actions</ListingTable.Cell>
      </ListingTable.Row>
    </ListingTable.Head>
    <ListingTable.Body>
      {data.map((item) => (
        <ListingTable.Row key={item.id}>
          <ListingTable.Cell>{item.name}</ListingTable.Cell>
          <ListingTable.Cell>
            <Chip label={item.status} color="success" size="small" />
          </ListingTable.Cell>
          <ListingTable.Cell align="right">
            <IconButton size="small"><EditIcon size={18} /></IconButton>
            <IconButton size="small" color="error"><TrashIcon size={18} /></IconButton>
          </ListingTable.Cell>
        </ListingTable.Row>
      ))}
    </ListingTable.Body>
  </ListingTable>
</ListingTable.Container>
```

### Card Component

```tsx
import { Paper, Typography, Box, Button } from '@wso2/oxygen-ui';

<Paper sx={{ p: 3 }}>
  <Typography variant="h6" gutterBottom>Card Title</Typography>
  <Typography color="text.secondary" sx={{ mb: 2 }}>
    Card description text
  </Typography>
  <Box sx={{ display: 'flex', gap: 1 }}>
    <Button variant="contained">Primary</Button>
    <Button variant="outlined">Secondary</Button>
  </Box>
</Paper>
```

### Dialog Component

```tsx
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogContentText,
  DialogActions,
  Button,
} from '@wso2/oxygen-ui';

<Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
  <DialogTitle>Dialog Title</DialogTitle>
  <DialogContent>
    <DialogContentText>Dialog content goes here.</DialogContentText>
  </DialogContent>
  <DialogActions>
    <Button onClick={onClose}>Cancel</Button>
    <Button variant="contained" onClick={onConfirm}>Confirm</Button>
  </DialogActions>
</Dialog>
```

## MUI X Components (Use Namespaces)

```tsx
import { DataGrid, DatePickers, TreeView } from '@wso2/oxygen-ui';

// DataGrid
<DataGrid.DataGrid rows={rows} columns={columns} />

// DatePickers
<DatePickers.DatePicker value={date} onChange={setDate} />

// TreeView
<TreeView.SimpleTreeView>
  <TreeView.TreeItem itemId="1" label="Item 1" />
</TreeView.SimpleTreeView>
```

## Charts (Separate Package)

```tsx
// Charts are in a separate package built on Recharts
import { LineChart, BarChart, PieChart } from '@wso2/oxygen-ui-charts-react';

<LineChart series={series} />
<BarChart series={series} />
<PieChart series={series} />
```
