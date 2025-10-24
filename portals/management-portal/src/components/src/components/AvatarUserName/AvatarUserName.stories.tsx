import type { Meta, StoryObj } from '@storybook/react';
import { AvatarUserName } from './AvatarUserName';

const meta: Meta<typeof AvatarUserName> = {
  title: 'Choreo DS/AvatarUserName',
  component: AvatarUserName,
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
type Story = StoryObj<typeof AvatarUserName>;

export const Default: Story = {
  args: {
    username: 'John Doe',
    children: 'John Doe',
  },
  render: (args) => (
    <AvatarUserName {...args}>{args.username[0].toUpperCase()}</AvatarUserName>
  ),
};

export const Disabled: Story = {
  args: {
    username: 'John Doe',
    children: 'John Doe',
    disabled: true,
  },
  render: (args) => (
    <AvatarUserName {...args}>{args.username[0]}</AvatarUserName>
  ),
};

export const HideUsername: Story = {
  args: {
    username: 'John Doe',
    hideUsername: true,
    children: 'JD',
  },
  render: (args) => <AvatarUserName {...args}>{args.children}</AvatarUserName>,
};
