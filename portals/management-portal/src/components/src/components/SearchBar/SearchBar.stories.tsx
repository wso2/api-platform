import type { Meta, StoryObj } from '@storybook/react';
import { SearchBar } from './SearchBar';
import { Card } from '../Card';
import { Box, CardContent, Grid, Typography } from '@mui/material';
import { useState } from 'react';
import { ExpandableSearch } from './ExpandableSearch/ExpandableSearch';

const meta: Meta<typeof SearchBar> = {
  title: 'Choreo DS/SearchBar',
  component: SearchBar,
  tags: ['autodocs'],
  argTypes: {},
};

export default meta;
type Story = StoryObj<typeof SearchBar>;

export const Default: Story = {
  render: () => {
    return (
      <Card testId="search-bar">
        <CardContent>
          <Grid container spacing={3}>
            <Grid size={8}>
              <Box mb={2}>
                <Typography>Size - Small</Typography>
              </Box>
              <SearchBar
                size="small"
                testId="search-bar-default"
                onChange={() => {}}
                placeholder="Search"
              />
            </Grid>
            <Grid size={8}>
              <Box mb={2}>
                <Typography>Size - Medium(default)</Typography>
              </Box>
              <SearchBar
                size="medium"
                testId="search-bar-default"
                onChange={() => {}}
                placeholder="Search"
              />
            </Grid>
            <Grid size={8}>
              <Box mb={2}>
                <Typography>Color - Secondary </Typography>
              </Box>
              <SearchBar
                size="medium"
                testId="search-bar-default"
                onChange={() => {}}
                placeholder="Search"
                color="secondary"
              />
            </Grid>
          </Grid>
        </CardContent>
      </Card>
    );
  },
};

export const DefaultRight: Story = {
  render: () => {
    return (
      <Card testId="card-default-search-bar-wrapper">
        <CardContent>
          <Grid container spacing={3}>
            <Grid size={8}>
              <SearchBar
                testId="search-bar-default-right"
                onChange={() => {}}
                placeholder="Search"
                iconPlacement="right"
              />
            </Grid>
          </Grid>
        </CardContent>
      </Card>
    );
  },
};

export const Expandable: Story = {
  render: function Expandable() {
    const [searchVal, setSearchVal] = useState('');
    return (
      <Card testId="card-default-right-search-bar-wrapper">
        <CardContent>
          <Grid container spacing={3}>
            <Grid size={10}>
              <Box mb={2}>
                <Typography>Size - Small</Typography>
              </Box>
              <ExpandableSearch
                size="small"
                testId="search-expandable"
                setSearchString={setSearchVal}
                searchString={searchVal}
              />
            </Grid>
            <Grid size={10}>
              <Box mb={2}>
                <Typography>Size - Medium(default)</Typography>
              </Box>
              <ExpandableSearch
                size="medium"
                testId="search-expandable"
                setSearchString={setSearchVal}
                searchString={searchVal}
              />
            </Grid>
          </Grid>
        </CardContent>
      </Card>
    );
  },
};

export const ExpandableRight: Story = {
  render: function ExpandableRight() {
    const [searchVal, setSearchVal] = useState('');

    return (
      <Card testId="card-expandable-search-bar-wrapper">
        <CardContent>
          <Grid container spacing={3}>
            <Grid size={10}>
              <Box mb={2}>
                <Typography>Size - Small</Typography>
              </Box>
              <ExpandableSearch
                size="small"
                testId="search-expandable-right"
                direction="right"
                setSearchString={setSearchVal}
                searchString={searchVal}
              />
            </Grid>
            <Grid size={10}>
              <Box mb={2}>
                <Typography>Size - Medium(default)</Typography>
              </Box>
              <ExpandableSearch
                size="medium"
                testId="search-expandable-right"
                direction="right"
                setSearchString={setSearchVal}
                searchString={searchVal}
              />
            </Grid>
          </Grid>
        </CardContent>
      </Card>
    );
  },
};

export const SearchBarWithFilter: Story = {
  render: function SearchBarWithFilter() {
    const [filterValue, setFilterValue] = useState('0');
    const handleFilterChange = (value: string) => {
      setFilterValue(value);
    };
    return (
      <Card testId="search-bar">
        <CardContent>
          <Grid container spacing={3}>
            <Grid size={8}>
              <SearchBar
                testId="search-bar-end-action"
                onChange={() => {}}
                placeholder="Search"
                filterValue={filterValue}
                onFilterChange={handleFilterChange}
                filterItems={[
                  { value: 0, label: 'All' },
                  { value: 1, label: 'Name' },
                  { value: 2, label: 'Description' },
                ]}
              />
            </Grid>
            <Grid size={8}>
              <SearchBar
                testId="search-bar-end-action"
                onChange={() => {}}
                placeholder="Search"
                filterValue={filterValue}
                onFilterChange={handleFilterChange}
                filterItems={[
                  { value: 0, label: 'All' },
                  { value: 1, label: 'Name' },
                  { value: 2, label: 'Description' },
                ]}
                bordered
              />
            </Grid>
          </Grid>
        </CardContent>
      </Card>
    );
  },
};
