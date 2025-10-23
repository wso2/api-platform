import type { Meta, StoryObj } from '@storybook/react';
import { ProgressBar } from './ProgressBar';

const meta: Meta<typeof ProgressBar> = {
  title: 'Components/ProgressBar',
  component: ProgressBar,
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
type Story = StoryObj<typeof ProgressBar>;

export const Default: Story = {
  args: {
    children: 'ProgressBar Content',
  },
};

export const Disabled: Story = {
  args: {
    children: 'Disabled ProgressBar',
    disabled: true,
  },
};

export const Determinate: Story = {
  args: {
    variant: 'determinate',
    children: 'Determinate ProgressBar',
  },
};

export const Indeterminate: Story = {
  args: {
    variant: 'indeterminate',
    children: 'Indeterminate ProgressBar',
  },
};
