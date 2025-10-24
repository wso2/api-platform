import type { Meta, StoryObj } from '@storybook/react';
import { NoDataMessage } from './NoDataMessage';
import { Box, Card, Grid } from '@mui/material';
import { IntlProvider } from 'react-intl';

const meta: Meta<typeof NoDataMessage> = {
  title: 'Choreo DS/NoDataMessage',
  component: NoDataMessage,
  tags: ['autodocs'],
  argTypes: {},
};

export default meta;
type Story = StoryObj<typeof NoDataMessage>;

export const Default: Story = {
  args: {
    size: 'md',
  },
  render: (args) => {
    return (
      <Grid container spacing={3}>
        <Grid size={{ xs: 12, md: 4, lg: 4, xl: 4 }}>
          <Card>
            <IntlProvider locale="en">
              <NoDataMessage size={args.size} {...args} testId="story-1" />
            </IntlProvider>
          </Card>
        </Grid>
        <Grid size={{ xs: 12, md: 8, lg: 9, xl: 10 }}>
          <Card>
            <Box my={5}>
              <IntlProvider locale="en">
                <NoDataMessage size={args.size} {...args} testId="story-2" />
              </IntlProvider>
            </Box>
          </Card>
        </Grid>
      </Grid>
    );
  },
};
