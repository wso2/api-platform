import type { Meta, StoryObj } from '@storybook/react';
import { Tooltip } from './Tooltip';
import { Button } from '../Button';
import { QuestionMark } from '@mui/icons-material';
import { Box, Typography } from '@mui/material';
import { Card, CardContent } from '../Card';
import Question from '@design-system/Icons/generated/Question';
import Info from '@design-system/Icons/generated/Info';

const meta: Meta<typeof Tooltip> = {
  title: 'Components/Tooltip',
  component: Tooltip,
  tags: ['autodocs'],
  argTypes: {
    onClick: {
      action: 'clicked',
      description: 'Click event handler',
    },
  },
};

export default meta;
type Story = StoryObj<typeof Tooltip>;

export const Default: Story = {
  args: {
    children: 'Tooltip Content',
    title: 'This is a tooltip',
    example: 'This is an example of a tooltip',
  },
  render: (args) => (
    <Tooltip {...args}>
      <Button variant="contained" color="primary">
        Hover me
      </Button>
    </Tooltip>
  ),
};

export const WithIcon: Story = {
  args: {
    children: 'Tooltip with icon',
    title: 'This is a tooltip with an icon',
  },
  render: (args) => {
    return (
      <Tooltip {...args}>
        <QuestionMark />
      </Tooltip>
    );
  },
};

export const primary: Story = {
  render: () => {
    return (
      <Card testId="tooltip">
        <CardContent>
          <Box mb={3}>
            <Tooltip title="This is a create Button">
              <Button testId="tooltip-action-1-button">Create Tooltip</Button>
            </Tooltip>
          </Box>
          <Box mb={3}>
            <Tooltip title="This is a info icon" darken>
              <Question />
            </Tooltip>
          </Box>
          <Box mb={3}>
            <Tooltip
              title={
                <Box>
                  <Typography variant="h4">Title</Typography>
                  <Typography variant="body1">
                    Create programs that trigger via events. E.g., Business
                    automation tasks.
                  </Typography>
                </Box>
              }
            >
              <Info />
            </Tooltip>
          </Box>
          <Box mb={3}>
            <Tooltip
              heading="heading"
              example="1,2,3"
              content="Tooltip content goes here"
            >
              <Button testId="tooltip-action-3-button">Custom Content</Button>
            </Tooltip>
          </Box>
        </CardContent>
      </Card>
    );
  },
};
