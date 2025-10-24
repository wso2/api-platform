import { Add } from '@mui/icons-material';
import { IconButton } from './IconButton';
import type { Meta, StoryObj } from '@storybook/react';

const meta: Meta<typeof IconButton> = {
  title: 'Components/IconButton',
  component: IconButton,
  tags: ['autodocs'],
  argTypes: {
    disabled: {
      control: 'boolean',
      description: 'Disables the component',
    },
    onClick: { action: 'clicked' },
  },
};

export default meta;
type Story = StoryObj<typeof IconButton>;

export const Default: Story = {
  args: {
    children: <Add />,
    color: 'primary',
  },
};

export const Disabled: Story = {
  args: {
    children: <Add />,
    color: 'primary',
    disabled: true,
  },
};
