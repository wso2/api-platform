import { Folder } from '@mui/icons-material';
import { Avatar } from './Avatar';
import type { Meta, StoryObj } from '@storybook/react';

const meta: Meta<typeof Avatar> = {
  title: 'Components/Avatar',
  component: Avatar,
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
type Story = StoryObj<typeof Avatar>;

export const Default: Story = {
  args: {
    children: 'A',
  },
};

export const Disabled: Story = {
  args: {
    children: 'D',
    disabled: true,
  },
};

export const IconAvatar: Story = {
  args: {
    children: <Folder />,
    variant: 'circular',
  },
};

export const AvatarImage: Story = {
  args: {
    children: (
      <img src="./images/storybook-assets/user-avatar.jpg" alt="Avatar" />
    ),
    variant: 'circular',
    width: 40,
    height: 40,
  },
};
