import type { Meta, StoryObj } from '@storybook/react';
import { Toggler } from './Toggler';
import { Card, CardContent } from '../Card';
import { Box } from '@mui/material';

const meta: Meta<typeof Toggler> = {
  title: 'Components/Toggler',
  component: Toggler,
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
type Story = StoryObj<typeof Toggler>;

export const Default: Story = {
  args: {
    children: 'Toggler Content',
  },
};

export const Variants: Story = {
  render: () => (
    <Card testId="toggle">
      <CardContent>
        <Box display="flex" gap="1rem">
          <Box>
            <Toggler testId="default-toggle" />
          </Box>
          <Box>
            <Toggler size="small" testId="small-toggle" />
          </Box>
          <Box>
            <Toggler disabled testId="disable-toggle" />
          </Box>
          <Box>
            <Toggler
              disabled
              checked
              color="default"
              testId="disable-checked-toggle"
            />
          </Box>
        </Box>
      </CardContent>
    </Card>
  ),
};
