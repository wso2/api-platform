import type { Meta, StoryObj } from '@storybook/react';
import { Select } from './Select';
import React from 'react';
import { Box, Grid, Typography } from '@mui/material';
import Branch from '@design-system/Icons/generated/Branch';
import { Chip } from '../Chip';
import { Button } from '../Button';
import Tools from '@design-system/Icons/generated/Tools';

const meta: Meta<typeof Select> = {
  title: 'Components/Select',
  component: Select,
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

const optionList = [
  { label: 'The Shawshank Redemption', value: '1994' },
  { label: 'The Godfather', value: '1972' },
  { label: 'The Godfather: Part II', value: '1974' },
  { label: 'The Dark Knight', value: '2008' },
  { label: '12 Angry Men', value: '1957' },
  { label: "Schindler's List", value: '1993' },
  { label: 'Pulp Fiction', value: '1994' },
  { label: 'The Lord of the Rings: The Return of the King', value: '2003' },
  { label: 'The Good, the Bad and the Ugly', value: '1966' },
  { label: 'Fight Club', value: '1999' },
  { label: 'The Lord of the Rings: The Fellowship of the Ring', value: '2001' },
  { label: 'Star Wars: Episode V - The Empire Strikes Back', value: '1980' },
  { label: 'Forrest Gump', value: '1994' },
  { label: 'Inception', value: '2010' },
  { label: 'The Lord of the Rings: The Two Towers', value: '2002' },
  { label: "One Flew Over the Cuckoo's Nest", value: '1975' },
];

const optionListWithIcon = [
  {
    label: 'Amazon S3',
    value: 'amazon',
    icon: 'https://bcentral-dev-packageicons.azureedge.net/images/ballerinax_aws.s3_3.1.0.png',
  },
  {
    label: 'GitHub',
    value: 'github',
    icon: 'https://bcentral-dev-packageicons.azureedge.net/images/ballerinax_github_4.4.0.png',
  },
];

// Define proper types for the option objects
interface OptionType {
  label: string;
  value: string;
}

interface OptionWithIconType extends OptionType {
  icon: string;
}

export default meta;
type Story = StoryObj<typeof Select>;

export const Default: Story = {
  args: {
    options: optionList,
    label: 'Select a movie',
    placeholder: 'Choose a movie',
  },
  render: function DefaultSelect(args) {
    // Remove unused onChange destructuring
    const [value, setValue] = React.useState<OptionType>(optionList[0]);
    const handleChange = (val: unknown) => {
      setValue(val as OptionType);
    };

    return (
      <Box sx={{ maxWidth: 800 }}>
        <Grid container spacing={3}>
          <Grid size={{ xs: 12, md: 6 }}>
            <Box sx={{ mb: 1 }}>
              <Typography variant="h3">Select</Typography>
            </Box>
            <Box sx={{ mb: 3 }}>
              <Select
                label="Select List"
                {...args}
                name="selectFromList"
                labelId="SelectFromList"
                options={optionList}
                getOptionLabel={(option: unknown) =>
                  (option as OptionType).label
                }
                onChange={handleChange}
                value={value}
                testId="select-from-list"
              />
            </Box>
          </Grid>
        </Grid>
      </Box>
    );
  },
};

export const WithPlaceholder: Story = {
  args: {
    options: optionList,
    label: 'Select a movie',
    placeholder: 'Choose a movie',
  },
  render: function WithPlaceholderSelect(args) {
    const [value, setValue] = React.useState<OptionType>(optionList[0]);
    const handleChange = (val: unknown) => {
      setValue(val as OptionType);
    };

    return (
      <Box sx={{ maxWidth: 800 }}>
        <Grid container spacing={3}>
          <Grid size={{ xs: 12, md: 6 }}>
            <Box sx={{ mb: 1 }}>
              <Typography variant="h3">Select</Typography>
            </Box>
            <Box sx={{ mb: 3 }}>
              <Select
                label="Select List"
                {...args}
                name="selectFromList"
                labelId="SelectFromList"
                options={optionList}
                getOptionLabel={(option: unknown) =>
                  (option as OptionType).label
                }
                onChange={handleChange}
                value={value}
                testId="select-from-list"
              />
            </Box>
          </Grid>
        </Grid>
      </Box>
    );
  },
};

export const WithSelectError: Story = {
  args: {
    options: optionList,
    label: 'Select a movie',
    placeholder: 'Choose a movie',
    error: true,
  },
  render: function WithSelectErrorSelect(args) {
    const [value, setValue] = React.useState<OptionType>(optionList[0]);
    const handleChange = (val: unknown) => {
      setValue(val as OptionType);
    };

    return (
      <Box sx={{ maxWidth: 800 }}>
        <Grid container spacing={3}>
          <Grid size={{ xs: 12, md: 6 }}>
            <Box sx={{ mb: 1 }}>
              <Typography variant="h3">Select</Typography>
            </Box>
            <Box sx={{ mb: 3 }}>
              <Select
                label="Select List"
                {...args}
                name="selectFromList"
                labelId="SelectFromList"
                options={optionList}
                getOptionLabel={(option: unknown) =>
                  (option as OptionType).label
                }
                onChange={handleChange}
                helperText="This is an error message"
                value={value}
                testId="select-from-list"
              />
            </Box>
          </Grid>
        </Grid>
      </Box>
    );
  },
};

export const SelectDisabled: Story = {
  args: {
    options: optionList,
    label: 'Select Disabled',
    placeholder: 'Choose a movie',
    disabled: true,
  },
  render: function SelectDisabled(args) {
    const { onChange, ...restArgs } = args;
    const [value, setValue] = React.useState(optionList[0]);
    const handleChange = (val: any) => {
      setValue(val);
    };

    return (
      <Box sx={{ maxWidth: 800 }}>
        <Grid container spacing={3}>
          <Grid size={{ xs: 12, md: 6 }}>
            <Box sx={{ mb: 1 }}>
              <Typography variant="h3">Select</Typography>
            </Box>
            <Box sx={{ mb: 3 }}>
              <Select
                label="Select List"
                {...restArgs}
                name="selectDisabled"
                labelId="SelectDisabled"
                options={optionList}
                getOptionLabel={(option: any) => option.label}
                onChange={handleChange}
                value={value}
                disabled={true}
                placeholder="Choose a movie"
                testId="select-disabled"
              />
            </Box>
          </Grid>
        </Grid>
      </Box>
    );
  },
};

export const WithOptional: Story = {
  args: {
    options: optionList,
    label: 'Select with optional',
    placeholder: 'Choose a movie',
    optional: true,
  },
  render: function SelectWithOptional(args) {
    const { onChange, ...restArgs } = args;
    const [value, setValue] = React.useState(optionList[0]);
    const handleChange = (val: any) => {
      setValue(val);
    };

    return (
      <Box sx={{ maxWidth: 800 }}>
        <Grid container spacing={3}>
          <Grid size={{ xs: 12, md: 6 }}>
            <Box sx={{ mb: 1 }}>
              <Typography variant="h3">Select</Typography>
            </Box>
            <Box sx={{ mb: 3 }}>
              <Select
                label="Select List"
                {...restArgs}
                name="selectWithOptional"
                labelId="SelectWithOptional"
                options={optionList}
                getOptionLabel={(option: any) => option.label}
                onChange={handleChange}
                value={value}
                optional={true}
                placeholder="Choose a movie"
                testId="select-with-optional"
              />
            </Box>
          </Grid>
        </Grid>
      </Box>
    );
  },
};

export const WithTooltip: Story = {
  args: {
    options: optionList,
    label: 'Select with tooltip',
    placeholder: 'Choose a movie',
    tooltip: 'This is a tooltip for the select component',
  },
  render: function SelectWithTooltip(args) {
    const { onChange, ...restArgs } = args;
    const [value, setValue] = React.useState(optionList[0]);
    const handleChange = (val: any) => {
      setValue(val);
    };

    return (
      <Box sx={{ maxWidth: 800 }}>
        <Grid container spacing={3}>
          <Grid size={{ xs: 12, md: 6 }}>
            <Box sx={{ mb: 1 }}>
              <Typography variant="h3">Select</Typography>
            </Box>
            <Box sx={{ mb: 3 }}>
              <Select
                label="Select List"
                {...restArgs}
                name="selectWithOptional"
                labelId="SelectWithOptional"
                options={optionList}
                getOptionLabel={(option: any) => option.label}
                onChange={handleChange}
                value={value}
                optional={true}
                tooltip="This is a tooltip for the select component"
                placeholder="Choose a movie"
                testId="select-with-optional"
              />
            </Box>
          </Grid>
        </Grid>
      </Box>
    );
  },
};

export const WithCreateButton: Story = {
  args: {
    options: optionList,
    label: 'Select with create button',
    placeholder: 'Choose a movie',
    addBtnText: 'Add Movie',
  },
  render: function SelectWithCreateButton(args) {
    const { onChange, ...restArgs } = args;
    const [value, setValue] = React.useState(optionList[0]);
    const handleChange = (val: any) => {
      setValue(val);
    };

    return (
      <Box sx={{ maxWidth: 800 }}>
        <Grid container spacing={3}>
          <Grid size={{ xs: 12, md: 6 }}>
            <Box sx={{ mb: 1 }}>
              <Typography variant="h3">Select</Typography>
            </Box>
            <Box sx={{ mb: 3 }}>
              <Select
                label="Select List"
                {...restArgs}
                name="selectWithOptional"
                labelId="SelectWithOptional"
                options={optionList}
                getOptionLabel={(option: any) => option.label}
                onChange={handleChange}
                value={value}
                optional={true}
                tooltip="This is a tooltip for the select component"
                placeholder="Choose a movie"
                testId="select-with-optional"
                onAddClick={() => {}}
              />
            </Box>
          </Grid>
        </Grid>
      </Box>
    );
  },
};

export const WithStartIcon: Story = {
  args: {
    options: optionListWithIcon,
    label: 'Select with start icon',
    placeholder: 'Choose a service',
  },
  render: function SelectWithStartIcon(args) {
    const { onChange, ...restArgs } = args;
    const [value, setValue] = React.useState(optionList[0]);
    const handleChange = (val: any) => {
      setValue(val);
    };

    return (
      <Box sx={{ maxWidth: 800 }}>
        <Grid container spacing={3}>
          <Grid size={{ xs: 12, md: 6 }}>
            <Box sx={{ mb: 1 }}>
              <Typography variant="h3">Select</Typography>
            </Box>
            <Box sx={{ mb: 3 }}>
              <Select
                label="Select List"
                {...restArgs}
                name="selectWithStartIcon"
                labelId="SelectWithStartIcon"
                options={optionList}
                getOptionLabel={(option: any) => option.label}
                onChange={handleChange}
                value={value}
                optional={true}
                tooltip="This is a tooltip for the select component"
                placeholder="Choose a movie"
                testId="select-with-start-icon"
                startIcon={<Branch />}
              />
            </Box>
          </Grid>
        </Grid>
      </Box>
    );
  },
};

export const SelectListWithIcon: Story = {
  args: {
    options: optionListWithIcon,
    label: 'Select with icon',
    placeholder: 'Choose a service',
  },
  render: function SelectListWithIconSelect(args) {
    const [menuValue, setMenuValue] = React.useState<OptionWithIconType>(
      optionListWithIcon[0]
    );
    const handleMenuValueChange = (val: unknown) => {
      setMenuValue(val as OptionWithIconType);
    };

    return (
      <Box sx={{ maxWidth: 800 }}>
        <Grid container spacing={3}>
          <Grid size={{ xs: 12, md: 6 }}>
            <Box sx={{ mb: 1 }}>
              <Typography variant="h3">Select</Typography>
            </Box>
            <Box sx={{ mb: 3 }}>
              <Select
                {...args}
                name="selectListMenuIcon"
                label="Select List Menu Icon"
                labelId="SelectListMenuIcon"
                options={optionListWithIcon}
                getOptionLabel={(option: unknown) =>
                  (option as OptionWithIconType).label
                }
                getOptionIcon={(option: unknown) =>
                  (option as OptionWithIconType).icon
                }
                onChange={handleMenuValueChange}
                value={menuValue}
                testId="select-list-menu-icon"
              />
            </Box>
          </Grid>
        </Grid>
      </Box>
    );
  },
};

export const WithClear: Story = {
  args: {
    options: optionList,
    label: 'Select with clear',
    placeholder: 'Choose a movie',
    isClearable: true,
  },
  render: function SelectWithClear(args) {
    const [value, setValue] = React.useState(optionList[0]);
    const handleChange = (val: any) => {
      setValue(val);
    };

    return (
      <Box sx={{ maxWidth: 800 }}>
        <Grid container spacing={3}>
          <Grid size={{ xs: 12, md: 6 }}>
            <Box sx={{ mb: 1 }}>
              <Typography variant="h3">Select</Typography>
            </Box>
            <Box sx={{ mb: 3 }}>
              <Select
                {...args}
                name="selectListClearable"
                label="Select List Clearable"
                labelId="SelectListClearable"
                options={optionList}
                getOptionLabel={(option: any) => option.label}
                onChange={handleChange}
                value={value}
                testId="select-list-clearable"
              />
            </Box>
          </Grid>
        </Grid>
      </Box>
    );
  },
};

export const WithLoader: Story = {
  args: {
    options: optionList,
    label: 'Select with loader',
    placeholder: 'Choose a movie',
    isLoading: true,
  },
  render: function SelectWithLoader(args) {
    const { onChange, ...restArgs } = args;
    const [value, setValue] = React.useState(optionList[0]);
    const handleChange = (val: any) => {
      setValue(val);
    };

    return (
      <Box sx={{ maxWidth: 800 }}>
        <Grid container spacing={3}>
          <Grid size={{ xs: 12, md: 6 }}>
            <Box sx={{ mb: 1 }}>
              <Typography variant="h3">Select</Typography>
            </Box>
            <Box sx={{ mb: 3 }}>
              <Select
                label="Select List"
                {...restArgs}
                name="selectWithLoader"
                labelId="SelectWithLoader"
                options={optionList}
                getOptionLabel={(option: any) => option.label}
                onChange={handleChange}
                value={value}
                optional={true}
                tooltip="This is a tooltip for the select component"
                placeholder="Choose a movie"
                testId="select-with-loader"
                isLoading={true}
              />
            </Box>
          </Grid>
        </Grid>
      </Box>
    );
  },
};

export const WithSelectEndAction: Story = {
  args: {
    options: optionList,
    label: 'Select with end action',
    placeholder: 'Choose a movie',
  },
  render: function SelectWithEndAction(args) {
    const { onChange, ...restArgs } = args;
    const [value, setValue] = React.useState(optionList[0]);
    const handleChange = (val: any) => {
      setValue(val);
    };

    return (
      <Box sx={{ maxWidth: 800 }}>
        <Grid container spacing={3}>
          <Grid size={{ xs: 12, md: 6 }}>
            <Box sx={{ mb: 1 }}>
              <Typography variant="h3">Select End Actions</Typography>
            </Box>
            <Box sx={{ mb: 3 }}>
              <Select
                {...restArgs}
                name="selectList"
                label="Select List"
                labelId="SelectList"
                options={optionList}
                getOptionLabel={(option: any) => option.label}
                onChange={handleChange}
                testId="select-end-action"
                value={value}
                optional
                tooltip="This is tool tip"
                info={
                  <Chip
                    label="action"
                    variant="outlined"
                    size="small"
                    color="primary"
                    testId="select"
                  />
                }
                actions={
                  <Button
                    testId="select-action"
                    size="small"
                    variant="link"
                    startIcon={<Tools />}
                  >
                    Configure
                  </Button>
                }
              />
            </Box>
          </Grid>
        </Grid>
      </Box>
    );
  },
};
