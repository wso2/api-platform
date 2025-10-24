import type { Meta, StoryObj } from '@storybook/react';
import { TopLevelSelector } from './TopLevelSelector';
import { LevelItem, Level } from './utils';

const sampleItems: LevelItem[] = [
  { label: 'Item 1', id: '1' },
  { label: 'Item 2', id: '2' },
  { label: 'Item 3', id: '3' },
];

const meta: Meta<typeof TopLevelSelector> = {
  title: 'Layouts/TopLevelSelector',
  component: TopLevelSelector,
  tags: ['autodocs'],
  argTypes: {
    items: {
      control: 'object',
      description: 'Array of items to display',
    },
    selectedItem: {
      control: 'object',
      description: 'Currently selected item',
    },
    level: {
      control: 'select',
      options: Object.values(Level),
      description: 'The level of the selector',
    },
    onSelect: {
      action: 'selected',
      description: 'Called when an item is selected',
    },
    onClick: {
      action: 'clicked',
      description: 'Called when the selector is clicked',
    },
    isHighlighted: {
      control: 'boolean',
      description: 'Whether the selector is highlighted',
    },
    disabled: {
      control: 'boolean',
      description: 'Whether the selector is disabled',
    },
  },
};

export default meta;
type Story = StoryObj<typeof TopLevelSelector>;

export const Default: Story = {
  args: {
    items: sampleItems,
    recentItems: sampleItems,
    selectedItem: sampleItems[0],
    level: Level.ORGANIZATION,
    isHighlighted: false,
    disabled: false,
  },
};

export const Highlighted: Story = {
  args: {
    items: sampleItems,
    recentItems: sampleItems,
    selectedItem: sampleItems[0],
    level: Level.PROJECT,
    isHighlighted: true,
    disabled: false,
  },
};

export const Disabled: Story = {
  args: {
    items: sampleItems,
    recentItems: sampleItems,
    selectedItem: sampleItems[0],
    level: Level.COMPONENT,
    isHighlighted: false,
    disabled: true,
  },
};
