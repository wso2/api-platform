import type { Meta, StoryObj } from '@storybook/react';
import { SimpleSelect } from './SimpleSelect';
import React from 'react';
import { Card } from '../Card';
import { Box, SelectChangeEvent } from '@mui/material';
import { SelectMenuItem } from './SelectMenuItem/SelectMenuItem';

const meta: Meta<typeof SimpleSelect> = {
  title: 'Components/SimpleSelect',
  component: SimpleSelect,
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
    size: {
      control: 'select',
      options: ['small', 'medium'],
      description: 'Size of the select component',
      table: {
        type: { summary: 'small | medium' },
        defaultValue: { summary: 'medium' },
      },
    },
    onClick: {
      action: 'clicked',
      description: 'Click event handler',
    },
  },
};

const testId = 'simple-select';

export default meta;
type Story = StoryObj<typeof SimpleSelect>;

export const Default: Story = {
  args: {
    children: 'SimpleSelect Content',
    size: 'small',
    disabled: false,
  },
  render: function SimpleSelectDefault(args) {
    const [age, setAge] = React.useState('20');

    const handleChange = (event: SelectChangeEvent<unknown>) => {
      setAge(event.target.value as string);
    };

    return (
      <Box>
        <SimpleSelect
          testId={`${testId}-story`}
          value={age}
          onChange={handleChange}
          size={args.size}
          disabled={args.disabled}
        >
          <SelectMenuItem testId="story-default-ten" value={10}>
            Ten thousand only
          </SelectMenuItem>
          <SelectMenuItem testId="story-default-twenty" value={20}>
            Twenty five thousand only
          </SelectMenuItem>
          <SelectMenuItem testId="story-default-thirty" value={30}>
            Thirty thousand only
          </SelectMenuItem>
        </SimpleSelect>
      </Box>
    );
  },
};

export const Small: Story = {
  args: {
    size: 'small',
    disabled: false,
  },
  render: function SimpleSelectSmall(args) {
    const [age, setAge] = React.useState('20');

    const handleChange = (event: SelectChangeEvent<unknown>) => {
      setAge(event.target.value as string);
    };

    return (
      <Card testId={testId}>
        <Box padding={3}>
          <SimpleSelect
            testId={`${testId}-small`}
            value={age}
            onChange={handleChange}
            size={args.size}
            disabled={args.disabled}
          >
            <SelectMenuItem testId="story-small-ten" value={10}>
              Ten thousand only
            </SelectMenuItem>
            <SelectMenuItem testId="story-small-twenty" value={20}>
              Twenty five thousand only
            </SelectMenuItem>
            <SelectMenuItem testId="story-small-thirty" value={30}>
              Thirty thousand only
            </SelectMenuItem>
          </SimpleSelect>
        </Box>
      </Card>
    );
  },
};

export const Medium: Story = {
  args: {
    size: 'medium',
    disabled: false,
  },
  render: function SimpleSelectMedium(args) {
    const [age, setAge] = React.useState('20');

    const handleChange = (event: SelectChangeEvent<unknown>) => {
      setAge(event.target.value as string);
    };

    return (
      <Card testId={testId}>
        <Box padding={3}>
          <SimpleSelect
            testId={`${testId}-medium`}
            value={age}
            onChange={handleChange}
            size={args.size}
            disabled={args.disabled}
          >
            <SelectMenuItem testId="story-medium-ten" value={10}>
              Ten thousand only
            </SelectMenuItem>
            <SelectMenuItem testId="story-medium-twenty" value={20}>
              Twenty five thousand only
            </SelectMenuItem>
            <SelectMenuItem testId="story-medium-thirty" value={30}>
              Thirty thousand only
            </SelectMenuItem>
          </SimpleSelect>
        </Box>
      </Card>
    );
  },
};

export const Disabled: Story = {
  args: {
    disabled: true,
    size: 'medium',
  },
  render: function SimpleSelectDisabled(args) {
    const [age, setAge] = React.useState('20');

    const handleChange = (event: SelectChangeEvent<unknown>) => {
      setAge(event.target.value as string);
    };

    return (
      <Card testId={testId}>
        <Box padding={3}>
          <SimpleSelect
            testId={`${testId}-disabled`}
            value={age}
            onChange={handleChange}
            size={args.size}
            disabled={args.disabled}
          >
            <SelectMenuItem testId="story-disabled-ten" value={10}>
              Ten thousand only
            </SelectMenuItem>
            <SelectMenuItem testId="story-disabled-twenty" value={20}>
              Twenty five thousand only
            </SelectMenuItem>
            <SelectMenuItem testId="story-disabled-thirty" value={30}>
              Thirty thousand only
            </SelectMenuItem>
          </SimpleSelect>
        </Box>
      </Card>
    );
  },
};

export const Error: Story = {
  args: {
    error: true,
    size: 'medium',
  },
  render: function SimpleSelectError(args) {
    const [age, setAge] = React.useState('20');

    const handleChange = (event: SelectChangeEvent<unknown>) => {
      setAge(event.target.value as string);
    };

    return (
      <Card testId={testId}>
        <Box padding={3}>
          <SimpleSelect
            testId={`${testId}-error`}
            value={age}
            onChange={handleChange}
            size={args.size}
            error={args.error}
            helperText="This is an error message"
          >
            <SelectMenuItem testId="story-error-ten" value={10}>
              Ten thousand only
            </SelectMenuItem>
            <SelectMenuItem testId="story-error-twenty" value={20}>
              Twenty five thousand only
            </SelectMenuItem>
            <SelectMenuItem testId="story-error-thirty" value={30}>
              Thirty thousand only
            </SelectMenuItem>
          </SimpleSelect>
        </Box>
      </Card>
    );
  },
};

export const Loading: Story = {
  args: {
    isLoading: true,
    size: 'medium',
  },
  render: function SimpleSelectLoading(args) {
    const [age, setAge] = React.useState('20');

    const handleChange = (event: SelectChangeEvent<unknown>) => {
      setAge(event.target.value as string);
    };

    return (
      <Card testId={testId}>
        <Box padding={3}>
          <SimpleSelect
            testId={`${testId}-loading`}
            value={age}
            onChange={handleChange}
            size={args.size}
            isLoading={args.isLoading}
          >
            <SelectMenuItem testId="story-loading-ten" value={10}>
              Ten thousand only
            </SelectMenuItem>
            <SelectMenuItem testId="story-loading-twenty" value={20}>
              Twenty five thousand only
            </SelectMenuItem>
            <SelectMenuItem testId="story-loading-thirty" value={30}>
              Thirty thousand only
            </SelectMenuItem>
          </SimpleSelect>
        </Box>
      </Card>
    );
  },
};

export const Other: Story = {
  args: {
    size: 'medium',
    disabled: false,
  },
  render: function SimpleSelectOther(args) {
    const [age, setAge] = React.useState('20');

    const handleChange = (event: SelectChangeEvent<unknown>) => {
      setAge(event.target.value as string);
    };

    return (
      <Card testId={testId}>
        <Box padding={3} maxWidth={230}>
          <SimpleSelect
            testId={`${testId}-other`}
            value={age}
            onChange={handleChange}
            size={args.size}
            disabled={args.disabled}
          >
            <SelectMenuItem
              testId="story-other-ten"
              value={10}
              description="This is a description for ten"
            >
              Ten thousand only
            </SelectMenuItem>
            <SelectMenuItem testId="story-other-twenty" value={20}>
              Twenty five thousand only
            </SelectMenuItem>
            <SelectMenuItem testId="story-other-thirty" value={30}>
              Thirty thousand only
            </SelectMenuItem>
          </SimpleSelect>
        </Box>
      </Card>
    );
  },
};
