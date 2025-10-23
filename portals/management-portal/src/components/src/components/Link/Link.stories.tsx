import type { Meta, StoryObj } from '@storybook/react';
import { Link } from './Link';

const meta: Meta<typeof Link> = {
  title: 'Components/Link',
  component: Link,
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
type Story = StoryObj<typeof Link>;

export const Default: Story = {
  args: {
    children: 'Link Content',
    underline: 'hover',
  },
};

export const ColorInherit: Story = {
  args: {
    children: 'Color Inherit',
    color: 'inherit',
    underline: 'hover',
  },
};

export const Disabled: Story = {
  args: {
    children: 'Disabled Link',
    disabled: true,
    underline: 'none',
  },
};
