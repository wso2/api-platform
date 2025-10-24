import type { Meta, StoryObj } from '@storybook/react';
import { SplitButton } from './SplitButton';
import React from 'react';
import { Box, CircularProgress, MenuList } from '@mui/material';
import SplitMenuItem from './SplitMenuItem';

const meta: Meta<typeof SplitButton> = {
  title: 'Choreo DS/SplitButton',
  component: SplitButton,
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
type Story = StoryObj<typeof SplitButton>;

const options = [
  'Create a merge commit',
  'Squash and merge',
  'Rebase and merge',
];

const SplitButtonWrapper = (props: any) => {
  const [open, setOpen] = React.useState(false);
  const [selectedIndex, setSelectedIndex] = React.useState(1);

  const handleMenuItemClick = (
    _event: React.MouseEvent<HTMLLIElement, MouseEvent>,
    index: number
  ) => {
    setSelectedIndex(index);
    setOpen(false);
  };

  return (
    <Box>
      <SplitButton
        testId="split-button"
        selectedValue={options[selectedIndex]}
        onClick={() => {}}
        label="Deploy"
        setOpen={setOpen}
        open={open}
        {...props}
      >
        <MenuList dense disablePadding>
          {options.map((option, index) => (
            <SplitMenuItem
              key={option}
              disabled={index === 1}
              selected={index === selectedIndex}
              onClick={(event) => handleMenuItemClick(event, index)}
              colorVariant={props.color}
            >
              {option}
            </SplitMenuItem>
          ))}
        </MenuList>
      </SplitButton>
    </Box>
  );
};

const SplitButtonWrapperWithStartIcon = (props: any) => {
  const [open, setOpen] = React.useState(false);
  const [selectedIndex, setSelectedIndex] = React.useState(1);

  const handleMenuItemClick = (
    _event: React.MouseEvent<HTMLLIElement, MouseEvent>,
    index: number
  ) => {
    setSelectedIndex(index);
    setOpen(false);
  };

  return (
    <Box>
      <SplitButton
        testId="split-button"
        selectedValue={options[selectedIndex]}
        onClick={() => {}}
        label="Deploy"
        setOpen={setOpen}
        open={open}
        startIcon={<CircularProgress color={'inherit'} size={16} />}
        {...props}
      >
        <MenuList dense disablePadding>
          {options.map((option, index) => (
            <SplitMenuItem
              key={option}
              disabled={index === 1}
              selected={index === selectedIndex}
              onClick={(event) => handleMenuItemClick(event, index)}
              colorVariant={props.color}
            >
              {option}
            </SplitMenuItem>
          ))}
        </MenuList>
      </SplitButton>
    </Box>
  );
};

const SplitButtonWrapperOutlined = (props: any) => {
  const [open, setOpen] = React.useState(false);
  const [selectedIndex, setSelectedIndex] = React.useState(1);

  const handleMenuItemClick = (
    _event: React.MouseEvent<HTMLLIElement, MouseEvent>,
    index: number
  ) => {
    setSelectedIndex(index);
    setOpen(false);
  };

  return (
    <Box>
      <SplitButton
        testId="split-button"
        selectedValue={options[selectedIndex]}
        onClick={() => {}}
        label="Deploy"
        setOpen={setOpen}
        variant={'outlined'}
        open={open}
        {...props}
      >
        <MenuList dense disablePadding>
          {options.map((option, index) => (
            <SplitMenuItem
              key={option}
              disabled={index === 1}
              selected={index === selectedIndex}
              onClick={(event) => handleMenuItemClick(event, index)}
              colorVariant={props.color}
            >
              {option}
            </SplitMenuItem>
          ))}
        </MenuList>
      </SplitButton>
    </Box>
  );
};

export const Default: Story = {
  args: {
    children: 'SplitButton Content',
  },
  render: (args) => <SplitButtonWrapper {...args} />,
};

export const Disabled: Story = {
  args: {
    disabled: true,
    selectedValue: 'Disabled option',
    setOpen: () => {},
    open: false,
  },
  render: (args) => (
    <SplitButton {...args}>
      <MenuList dense disablePadding>
        <SplitMenuItem selected>Disabled Item</SplitMenuItem>
      </MenuList>
    </SplitButton>
  ),
};

export const WithStartIcon: Story = {
  render: (args) => <SplitButtonWrapperWithStartIcon {...args} />,
};

export const Outlined: Story = {
  render: (args) => <SplitButtonWrapperOutlined {...args} />,
};
