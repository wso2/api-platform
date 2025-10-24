import { Box, Grid, InputAdornment, Typography } from '@mui/material';
import { Card, CardContent } from '../Card';
import { TextInput } from './TextInput';
import type { Meta, StoryObj } from '@storybook/react';
import React from 'react';
import { IconButton } from '../IconButton';
import ShowPassword from '@design-system/Icons/generated/ShowPassword';
import HidePassword from '@design-system/Icons/generated/HidePassword';
import { Chip } from '../Chip';
import { Button } from '../Button';
import Tools from '@design-system/Icons/generated/Tools';

const meta: Meta<typeof TextInput> = {
  title: 'Components/TextInput',
  component: TextInput,
  tags: ['autodocs'],
  argTypes: {
    fullWidth: {
      control: 'boolean',
      description: 'If true, the text input will be full width',
    },
    size: {
      control: 'select',
      options: ['small', 'medium', 'large'],
      description: 'Size of the text input',
    },
    helperText: {
      control: 'text',
      description: 'Helper text to display',
    },
    label: {
      control: 'text',
      description: 'Label for the text input',
    },
    tooltip: {
      control: 'text',
      description: 'Optional tooltip text',
    },
    error: {
      control: 'boolean',
      description: 'Error message to display',
    },
    disabled: {
      control: 'boolean',
      description: 'Disables the component',
    },
    onChange: {
      action: 'changed',
      description: 'Callback when text changes',
    },
  },
};

const testId = 'text-input';

export default meta;
type Story = StoryObj<typeof TextInput>;

export const Default: Story = {
  args: {
    placeholder: 'Placeholder',
  },
  render: (args) => {
    return (
      <Card testId={`${testId}-default`}>
        <CardContent>
          <Box>
            <Box mb={3}>
              <TextInput {...args} testId={testId} />
            </Box>
            <Box mb={3}>
              <TextInput
                {...args}
                disabled
                value="Disabled"
                testId={`${testId}-disabled`}
              />
            </Box>
            <Box mb={3}>
              <TextInput
                {...args}
                error
                value="Error Text"
                errorMessage="Error helper text"
                tooltip="Tooltip goes here"
                label="Error Input"
                testId={`${testId}-error`}
              />
            </Box>
            <Box mb={3}>
              <TextInput
                {...args}
                readonly={true}
                value="Read Only"
                testId={`${testId}-readonly`}
              />
            </Box>
            <Box mb={3}>
              <TextInput
                {...args}
                tooltip="Tooltip goes here"
                optional
                label="Label"
                testId={`${testId}-with-label`}
              />
            </Box>
            <Box mb={3}>
              <TextInput
                {...args}
                multiline={true}
                rows={3}
                placeholder="Textarea"
                testId="text-area"
              />
            </Box>
          </Box>
        </CardContent>
      </Card>
    );
  },
};

export const textInputTypography: Story = {
  args: {
    value: 'Component Name',
  },
  render: function TextWithInput(args) {
    const [value, setValue] = React.useState('Component Name');

    const handleChange = (text: string) => {
      setValue(text);
    };
    return (
      <Card testId={`${testId}-with-label`}>
        <CardContent>
          <Box>
            <Box mb={3}>
              <TextInput
                label="default"
                {...args}
                onChange={handleChange}
                value={value}
                testId={`${testId}-default`}
              />
            </Box>
            <Box mb={3}>
              <TextInput
                label="h6"
                {...args}
                typography="h6"
                onChange={handleChange}
                value={value}
                testId={`${testId}-h6`}
              />
            </Box>
            <Box mb={3}>
              <TextInput
                label="h5"
                {...args}
                typography="h5"
                onChange={handleChange}
                value={value}
                testId={`${testId}-h5`}
              />
            </Box>
            <Box mb={3}>
              <TextInput
                label="h4"
                {...args}
                typography="h4"
                tooltip="Tooltip goes here"
                optional
                onChange={handleChange}
                value={value}
                testId={`${testId}-h4`}
              />
            </Box>
            <Box mb={3}>
              <TextInput
                label="h3"
                {...args}
                typography="h3"
                tooltip="Tooltip goes here"
                optional
                onChange={handleChange}
                value={value}
                testId={`${testId}-h3`}
              />
            </Box>
            <Box mb={3}>
              <TextInput
                label="h2"
                {...args}
                typography="h2"
                tooltip="Tooltip goes here"
                optional
                onChange={handleChange}
                value={value}
                testId={`${testId}-h2`}
              />
            </Box>
            <Box mb={3}>
              <TextInput
                label="h1"
                {...args}
                typography="h1"
                tooltip="Tooltip goes here"
                optional
                onChange={handleChange}
                value={value}
                testId={`${testId}-h1`}
              />
            </Box>
            <Box mb={3}>
              <TextInput
                label="default"
                {...args}
                multiline
                rows={3}
                typography="h1"
                placeholder="Textarea"
                onChange={handleChange}
                value={value}
                testId="text-area"
              />
            </Box>
          </Box>
        </CardContent>
      </Card>
    );
  },
};

export const textInputEndIconButton: Story = {
  render: function TextInputEndIcons(args) {
    const [value, setValue] = React.useState('123456789');

    const handleChange = (text: string) => {
      setValue(text);
    };

    const [showPassword, toggleInputType] = React.useState(false);

    const handleEndButtonClick = () => {
      toggleInputType(!showPassword);
    };

    return (
      <Card testId={`${testId}-with-icon-button`}>
        <CardContent>
          <Box>
            <Box mb={3}>
              <TextInput
                label="Secret"
                {...args}
                onChange={handleChange}
                value={value}
                type={showPassword ? 'text' : 'password'}
                endAdornment={
                  <InputAdornment position="end">
                    <IconButton
                      onClick={handleEndButtonClick}
                      size="small"
                      variant="text"
                      color="primary"
                      testId="secret"
                    >
                      {showPassword ? <ShowPassword /> : <HidePassword />}
                    </IconButton>
                  </InputAdornment>
                }
                testId={`${testId}-secret`}
              />
            </Box>
            <Box mb={3}>
              <TextInput
                label="Secret"
                {...args}
                onChange={handleChange}
                value={value}
                type={showPassword ? 'text' : 'password'}
                size="small"
                endAdornment={
                  <InputAdornment position="end">
                    <IconButton
                      onClick={handleEndButtonClick}
                      size="tiny"
                      variant="text"
                      color="primary"
                      testId="secret-small"
                    >
                      {showPassword ? <ShowPassword /> : <HidePassword />}
                    </IconButton>
                  </InputAdornment>
                }
                testId={`${testId}-small-secret`}
              />
            </Box>
          </Box>
        </CardContent>
      </Card>
    );
  },
};

export const textInputEndActions: Story = {
  args: {},
  render: function TextInputEndActions(args) {
    const [value, setValue] = React.useState('API Proxy');

    const handleChange = (text: string) => {
      setValue(text);
    };

    return (
      <Card testId={`${testId}-end-action`}>
        <CardContent>
          <Box>
            <Box mb={3}>
              <TextInput
                {...args}
                label="Component Name"
                onChange={handleChange}
                value={value}
                optional
                info={
                  <Chip
                    label="string"
                    variant="outlined"
                    size="small"
                    color="primary"
                    testId="text-input"
                  />
                }
                tooltip="This is tool tip"
                actions={
                  <Button
                    testId={`${testId}-action-button`}
                    size="small"
                    variant="link"
                    startIcon={<Tools />}
                  >
                    Configure
                  </Button>
                }
                testId={`${testId}-outline`}
              />
            </Box>
          </Box>
        </CardContent>
      </Card>
    );
  },
};

export const textInputRoundedEndActions: Story = {
  args: {},
  render: function TextInputRoundedEndActions(args) {
    const [value, setValue] = React.useState('123456789');

    const handleChange = (text: string) => {
      setValue(text);
    };

    return (
      <Card testId={`${testId}-with-rounded-button`}>
        <CardContent>
          <Box>
            <Box mb={3}>
              <TextInput
                label="Secret"
                {...args}
                onChange={handleChange}
                value={value}
                rounded
                endAdornment={
                  <InputAdornment position="end">
                    <Button size="small" color="primary" testId="execute" pill>
                      Execute
                    </Button>
                  </InputAdornment>
                }
                testId={`${testId}-secret`}
              />
            </Box>
            <Box mb={3}>
              <TextInput
                label="Secret"
                {...args}
                onChange={handleChange}
                value={value}
                size="small"
                rounded
                endAdornment={
                  <InputAdornment position="end">
                    <Button size="tiny" color="primary" testId="execute" pill>
                      Execute
                    </Button>
                  </InputAdornment>
                }
                testId={`${testId}-small-secret`}
              />
            </Box>
          </Box>
        </CardContent>
      </Card>
    );
  },
};

export const textInputSizes: Story = {
  args: {},
  render: function TextInputSizes(args) {
    return (
      <Grid container spacing={2}>
        <Grid size={{ xs: 12, md: 4 }}>
          <Card testId={`${testId}-sizes-small`}>
            <CardContent>
              <Box>
                <Box mb={3}>
                  <Typography variant="h3">Size - Small</Typography>
                </Box>
                <Box mb={3}>
                  <TextInput size="small" {...args} testId={testId} />
                </Box>
                <Box mb={3}>
                  <TextInput
                    size="small"
                    {...args}
                    disabled
                    placeholder="Disabled"
                    testId={`${testId}-disabled`}
                  />
                </Box>
                <Box mb={3}>
                  <TextInput
                    size="small"
                    {...args}
                    error
                    value="Error Text"
                    helperText="Error helper text"
                    tooltip="Tooltip goes here"
                    label="Error Input"
                    testId={`${testId}-error`}
                  />
                </Box>
                <Box mb={3}>
                  <TextInput
                    size="small"
                    {...args}
                    readonly
                    value="Read Only"
                    testId={`${testId}-readonly`}
                  />
                </Box>
                <Box mb={3}>
                  <TextInput
                    size="small"
                    {...args}
                    tooltip="Tooltip goes here"
                    optional
                    label="Label"
                    testId={`${testId}-with-label`}
                  />
                </Box>
                <Box mb={3}>
                  <TextInput
                    size="small"
                    {...args}
                    multiline
                    rows={3}
                    placeholder="Textarea"
                    testId="text-area"
                  />
                </Box>
              </Box>
            </CardContent>
          </Card>
        </Grid>
        <Grid size={{ xs: 12, md: 4 }}>
          <Card testId={`${testId}-sizes--medium`}>
            <CardContent>
              <Box>
                <Box mb={3}>
                  <Typography variant="h3">Size - Medium(Default)</Typography>
                </Box>
                <Box mb={3}>
                  <TextInput {...args} testId={testId} />
                </Box>
                <Box mb={3}>
                  <TextInput
                    {...args}
                    disabled
                    size="medium"
                    placeholder="Disabled"
                    testId={`${testId}-disabled`}
                  />
                </Box>
                <Box mb={3}>
                  <TextInput
                    {...args}
                    error
                    value="Error Text"
                    helperText="Error helper text"
                    tooltip="Tooltip goes here"
                    label="Error Input"
                    size="medium"
                    testId={`${testId}-error`}
                  />
                </Box>
                <Box mb={3}>
                  <TextInput
                    {...args}
                    readonly
                    size="medium"
                    value="Read Only"
                    testId={`${testId}-readonly`}
                  />
                </Box>
                <Box mb={3}>
                  <TextInput
                    {...args}
                    tooltip="Tooltip goes here"
                    optional
                    size="medium"
                    label="Label"
                    testId={`${testId}-with-label`}
                  />
                </Box>
                <Box mb={3}>
                  <TextInput
                    {...args}
                    multiline
                    rows={3}
                    size="medium"
                    placeholder="Textarea"
                    testId="text-area"
                  />
                </Box>
              </Box>
            </CardContent>
          </Card>
        </Grid>
        <Grid size={{ xs: 12, md: 4 }}>
          <Card testId={`${testId}-sizes--large`}>
            <CardContent>
              <Box>
                <Box mb={3}>
                  <Typography variant="h3">Size - Large</Typography>
                </Box>
                <Box mb={3}>
                  <TextInput size="large" {...args} testId={testId} />
                </Box>
                <Box mb={3}>
                  <TextInput
                    size="large"
                    {...args}
                    disabled
                    placeholder="Disabled"
                    testId={`${testId}-disabled`}
                  />
                </Box>
                <Box mb={3}>
                  <TextInput
                    size="large"
                    {...args}
                    error
                    value="Error Text"
                    helperText="Error helper text"
                    tooltip="Tooltip goes here"
                    label="Error Input"
                    testId={`${testId}-error`}
                  />
                </Box>
                <Box mb={3}>
                  <TextInput
                    size="large"
                    {...args}
                    readonly
                    value="Read Only"
                    testId={`${testId}-readonly`}
                  />
                </Box>
                <Box mb={3}>
                  <TextInput
                    size="large"
                    {...args}
                    tooltip="Tooltip goes here"
                    optional
                    label="Label"
                    testId={`${testId}-with-label`}
                  />
                </Box>
                <Box mb={3}>
                  <TextInput
                    size="large"
                    {...args}
                    multiline
                    rows={3}
                    placeholder="Textarea"
                    testId="text-area"
                  />
                </Box>
              </Box>
            </CardContent>
          </Card>
        </Grid>
      </Grid>
    );
  },
};
