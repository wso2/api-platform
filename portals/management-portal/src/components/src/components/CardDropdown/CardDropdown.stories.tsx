import type { Meta, StoryObj } from '@storybook/react';
import { CardDropdown } from './CardDropdown';
import { Box, Grid, Typography } from '@mui/material';
import { Card, CardContent } from '../index copy';
import Bitbucket from '@design-system/Images/generated/Bitbucket';
import { CardDropdownMenuItemCreate } from './CardDropdownMenuItemCreate/CardDropdownMenuItemCreate';
import CardDropdownMenuItem from './CardDropdownMenuItem';
import { useState } from 'react';
import { NoDataMessage } from '../NoDataMessage';

const meta: Meta<typeof CardDropdown> = {
  title: 'Components/Card/CardDropdown',
  component: CardDropdown,
  tags: ['autodocs'],
  argTypes: {
    size: {
      control: 'select',
      options: ['small', 'medium', 'large'],
      description: 'Size of the dropdown button',
      defaultValue: 'medium',
    },
  },
};

export default meta;
type Story = StoryObj<typeof CardDropdown>;

export const Default: Story = {
  args: {
    children: 'CardDropdown Content',
    size: 'medium',
    active: false,
  },
  render: function RenderCardDropdown(_args) {
    const [selectedItem, setSelectedItem] = useState(0);
    const handleCreate = () => {};

    const handleClick = (selectedNo: number) => {
      setSelectedItem(selectedNo);
    };
    return (
      <Box p={3}>
        <Card testId="dropdown">
          <CardContent>
            <Grid container spacing={3}>
              <Grid size={{ xs: 12, md: 12 }}>
                <CardDropdown
                  icon={<Bitbucket />}
                  text="Authorized Via bitbucket"
                  testId="bitbucket"
                  size={_args.size}
                  active={_args.active}
                  disabled={_args.disabled}
                  fullHeight
                >
                  <CardDropdownMenuItemCreate
                    createText="Create"
                    onClick={handleCreate}
                    testId="create"
                  />
                  <CardDropdownMenuItem
                    selected={selectedItem === 1}
                    // button
                    onClick={() => handleClick(1)}
                  >
                    Profile
                  </CardDropdownMenuItem>
                  <CardDropdownMenuItem
                    selected={selectedItem === 2}
                    // button
                    onClick={() => handleClick(2)}
                  >
                    My account
                  </CardDropdownMenuItem>
                  <CardDropdownMenuItem
                    selected={selectedItem === 3}
                    // button
                    onClick={() => handleClick(3)}
                  >
                    Logout
                  </CardDropdownMenuItem>
                </CardDropdown>
              </Grid>
              <Grid size={{ xs: 12, md: 6 }}>
                <CardDropdown
                  icon={<Bitbucket />}
                  text="Authorized Via bitbucket"
                  active
                  testId="bitbucket"
                  size={_args.size}
                  disabled={_args.disabled}
                  fullHeight
                >
                  <CardDropdownMenuItem
                    selected={selectedItem === 1}
                    // button
                    onClick={() => handleClick(1)}
                  >
                    Profile
                  </CardDropdownMenuItem>
                  <CardDropdownMenuItem
                    selected={selectedItem === 2}
                    // button
                    onClick={() => handleClick(2)}
                  >
                    My account
                  </CardDropdownMenuItem>
                  <CardDropdownMenuItem
                    selected={selectedItem === 3}
                    // button
                    onClick={() => handleClick(3)}
                  >
                    Logout
                  </CardDropdownMenuItem>
                </CardDropdown>
              </Grid>
            </Grid>
            <Grid container spacing={2}>
              <Grid size={{ xs: 12 }}>
                <Box mt={3}>
                  <Typography variant="h5">No data message</Typography>
                </Box>
              </Grid>
              <Grid size={{ xs: 12, md: 12 }}>
                <CardDropdown
                  icon={<Bitbucket />}
                  text="Authorized Via bitbucket"
                  active
                  size={_args.size}
                  disabled={_args.disabled}
                  testId="bitbucket"
                  fullHeight
                >
                  <NoDataMessage
                    size="sm"
                    message="No App passwords are configured. Contact the admin for assistance."
                    testId="card-dropdown"
                  />
                </CardDropdown>
              </Grid>
            </Grid>
          </CardContent>
        </Card>
      </Box>
    );
  },
};
