import type { Meta, StoryObj } from '@storybook/react';
import { NavItemExpandable } from './NavItemExpandable';
import { MenuHomeFilledIcon, MenuHomeIcon } from '@design-system/Icons';

const meta: Meta<typeof NavItemExpandable> = {
  title: 'Choreo DS/NavItemExpandable',
  component: NavItemExpandable,
  tags: ['autodocs'],
  argTypes: {
    disabled: {
      control: 'boolean',
      description: 'Disables the element',
      table: {
        type: { summary: 'boolean' },
        defaultValue: { summary: 'false' },
      },
    },
    onClick: {
      action: 'clicked',
      description: 'Click event handler',
    },
  },
};

export default meta;
type Story = StoryObj<typeof NavItemExpandable>;

export const Default: Story = {
  args: {
    title: 'NavItemExpandable Content',
    icon: <MenuHomeIcon fontSize='inherit' />,
    selectedIcon: <MenuHomeFilledIcon fontSize='inherit' />,
    id: 'nav-item-1',
    subMenuItems: [
      {
        title: 'Sub Item 1',
        id: 'sub-item-1',
        icon: <MenuHomeIcon fontSize='inherit' />,
        selectedIcon: <MenuHomeFilledIcon fontSize='inherit' />,
      },
      {
        title: 'Sub Item 2',
        id: 'sub-item-2',
        icon: <MenuHomeIcon fontSize='inherit' />,
        selectedIcon: <MenuHomeFilledIcon fontSize='inherit' />,
      },
      {
        title: 'Sub Item 3',
        id: 'sub-item-3',
        icon: <MenuHomeIcon fontSize='inherit' />,
        selectedIcon: <MenuHomeFilledIcon fontSize='inherit' />,
      },
    ],
  },
};

export const Expanded: Story = {
  args: {
    isExpanded: true,
    title: 'NavItemExpandable Content',
    icon: <MenuHomeIcon fontSize='inherit' />,
    selectedIcon: <MenuHomeFilledIcon fontSize='inherit' />,
    id: 'nav-item-1',
    selectedId: 'sub-item-1',
    subMenuItems: [
      {
        title: 'Sub Item 1',
        id: 'sub-item-1',
        icon: <MenuHomeIcon fontSize='inherit' />,
        selectedIcon: <MenuHomeFilledIcon fontSize='inherit' />,
      },
      {
        title: 'Sub Item 2',
        id: 'sub-item-2',
        icon: <MenuHomeIcon fontSize='inherit' />,
        selectedIcon: <MenuHomeFilledIcon fontSize='inherit' />,
      },
      {
        title: 'Sub Item 3',
        id: 'sub-item-3',
        icon: <MenuHomeIcon fontSize='inherit' />,
        selectedIcon: <MenuHomeFilledIcon fontSize='inherit' />,
      },
    ],
  },
};

export const NoSubMenuItems: Story = {
  args: {
    isExpanded: true,
    title: 'NavItemExpandable Content',
    icon: <MenuHomeIcon fontSize='inherit' />,
    selectedIcon: <MenuHomeFilledIcon fontSize='inherit' />,
    id: 'nav-item-1',
  },
};

export const Disabled: Story = {
  args: {
    title: 'Disabled NavItemExpandable',
    disabled: true,
    icon: <MenuHomeIcon fontSize='inherit' />,
    selectedIcon: <MenuHomeFilledIcon fontSize='inherit' />,
  },
};

export const SimpleNavItem: Story = {
  args: {
    title: 'Simple Navigation Item',
    icon: <MenuHomeIcon fontSize='inherit' />,
    selectedIcon: <MenuHomeFilledIcon fontSize='inherit' />,
    id: 'simple-nav-item',
  },
};

export const SimpleNavItemSelected: Story = {
  args: {
    title: 'Selected Simple Navigation Item',
    icon: <MenuHomeIcon fontSize='inherit' />,
    selectedIcon: <MenuHomeFilledIcon fontSize='inherit' />,
    id: 'simple-nav-item',
    selectedId: 'simple-nav-item',
  },
};

export const SimpleNavItemExpanded: Story = {
  args: {
    title: 'Simple Navigation Item (Expanded)',
    icon: <MenuHomeIcon fontSize='inherit' />,
    selectedIcon: <MenuHomeFilledIcon fontSize='inherit' />,
    id: 'simple-nav-item',
    isExpanded: true,
  },
};

export const SimpleNavItemExpandedSelected: Story = {
  args: {
    title: 'Selected Simple Navigation Item (Expanded)',
    icon: <MenuHomeIcon fontSize='inherit' />,
    selectedIcon: <MenuHomeFilledIcon fontSize='inherit' />,
    id: 'simple-nav-item',
    selectedId: 'simple-nav-item',
    isExpanded: true,
  },
};

export const SimpleNavItemDisabled: Story = {
  args: {
    title: 'Disabled Simple Navigation Item',
    icon: <MenuHomeIcon fontSize='inherit' />,
    selectedIcon: <MenuHomeFilledIcon fontSize='inherit' />,
    id: 'simple-nav-item',
    disabled: true,
  },
};

export const SimpleNavItemDisabledExpanded: Story = {
  args: {
    title: 'Disabled Simple Navigation Item (Expanded)',
    icon: <MenuHomeIcon fontSize='inherit' />,
    selectedIcon: <MenuHomeFilledIcon fontSize='inherit' />,
    id: 'simple-nav-item',
    disabled: true,
    isExpanded: true,
  },
};

export const EmptySubMenuItems: Story = {
  args: {
    title: 'NavItem with Empty Sub Menu',
    icon: <MenuHomeIcon fontSize='inherit' />,
    selectedIcon: <MenuHomeFilledIcon fontSize='inherit' />,
    id: 'nav-item-empty',
    subMenuItems: [],
  },
};

export const EmptySubMenuItemsExpanded: Story = {
  args: {
    title: 'NavItem with Empty Sub Menu (Expanded)',
    icon: <MenuHomeIcon fontSize='inherit' />,
    selectedIcon: <MenuHomeFilledIcon fontSize='inherit' />,
    id: 'nav-item-empty',
    subMenuItems: [],
    isExpanded: true,
  },
};
