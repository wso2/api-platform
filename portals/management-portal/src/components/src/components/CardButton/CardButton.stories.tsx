import type { Meta, StoryObj } from '@storybook/react';
import { CardButton } from './CardButton';
import { useState } from 'react';
import { Box, Grid, Typography } from '@mui/material';
import { Card, CardContent } from '../index copy';
import ChevronRight from '@design-system/Icons/generated/ChevronRight';
import Github from '@design-system/Images/generated/Github';
import Bitbucket from '@design-system/Images/generated/Bitbucket';

const meta: Meta<typeof CardButton> = {
  title: 'Components/Card/CardButton',
  component: CardButton,
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
type Story = StoryObj<typeof CardButton>;

export const Default: Story = {
  render: function RenderCardButton(args) {
    const [activeId, setActiveId] = useState<number | null>(null);

    return (
      <Box p={3}>
        <Card testId="cardbutton">
          <CardContent>
            <Grid container spacing={3}>
              <Grid size={{ xs: 12, md: 6, lg: 4, xl: 3 }}>
                <CardButton
                  {...args}
                  icon={<Github width={50} height={50} />}
                  text="Authorize with GitHub"
                  onClick={() => setActiveId(1)}
                  active={activeId === 1}
                  testId="github"
                />
              </Grid>
              <Grid size={{ xs: 12, md: 6, lg: 4, xl: 3 }}>
                <CardButton
                  {...args}
                  icon={<Bitbucket />}
                  text="Authorize with Bitbucket"
                  onClick={() => setActiveId(2)}
                  active={activeId === 2}
                  testId="bitbucket"
                />
              </Grid>
            </Grid>
            <Grid container spacing={2}>
              <Grid size={{ xs: 12 }}>
                <Box mt={2}>
                  <Typography variant="h5">With Error</Typography>
                </Box>
              </Grid>
              <Grid size={{ xs: 12, md: 6, lg: 4, xl: 3 }}>
                <CardButton
                  {...args}
                  icon={<Github />}
                  text="Authorize with GitHub"
                  onClick={() => setActiveId(1)}
                  active={activeId === 1}
                  testId="github"
                  error
                />
              </Grid>
            </Grid>
            <Grid container spacing={2}>
              <Grid size={{ xs: 12 }}>
                <Box mt={2}>
                  <Typography variant="h5">With end icon</Typography>
                </Box>
              </Grid>
              <Grid size={{ xs: 12, md: 12, lg: 12, xl: 12 }}>
                <CardButton
                  {...args}
                  icon={<Github />}
                  text="Authorize with GitHub"
                  onClick={() => setActiveId(3)}
                  active={activeId === 3}
                  testId="github"
                  endIcon={<ChevronRight fontSize="inherit" />}
                />
              </Grid>
            </Grid>
          </CardContent>
        </Card>
      </Box>
    );
  },
};
