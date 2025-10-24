import type { Meta, StoryObj } from '@storybook/react';
import { TooltipBase } from './TooltipBase';

const meta: Meta<typeof TooltipBase> = {
  title: 'Choreo DS/TooltipBase',
  component: TooltipBase,
  tags: ['autodocs'],
  argTypes: {
    onClick: {
      action: 'clicked',
      description: 'Click event handler',
    },
  },
};

export default meta;
type Story = StoryObj<typeof TooltipBase>;

export const Default: Story = {
  args: {
    children: 'TooltipBase Content',
  },
};
