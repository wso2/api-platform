import { Box, Divider } from '@mui/material';
import { ButtonContainer } from './ButtonContainer';
import type { Meta, StoryObj } from '@storybook/react';
import { Card, CardContent } from '../Card';
import { TextInput } from '../TextInput';
import { Button } from '../Button/Button';

const meta: Meta<typeof ButtonContainer> = {
  title: 'Choreo DS/ButtonContainer',
  component: ButtonContainer,
  tags: ['autodocs'],
  argTypes: {
    disabled: {
      control: 'boolean',
      description: 'Disables the component',
    },
    onClick: { action: 'clicked' },
  },
};

export default meta;
type Story = StoryObj<typeof ButtonContainer>;

const testId = 'button-container';

export const Default: Story = {
  args: {
    children: 'ButtonContainer Content',
  },
  render: (args) => (
    <Box mb={3} maxWidth={500}>
      <Card testId={testId}>
        <CardContent>
          <Box mb={3}></Box>
          <Box>
            <TextInput
              label="First Name"
              testId="first-name-input"
              value={''}
              onChange={function (_text: string): void {
                throw new Error('Function not implemented.');
              }}
            />
          </Box>
          <Box mt={3}>
            <Divider />
          </Box>
          <ButtonContainer {...args}>
            <Button testId="cancel" variant="outlined" color="secondary">
              Cancel
            </Button>
            <Button testId="save" variant="contained" color="primary">
              Save
            </Button>
          </ButtonContainer>
        </CardContent>
      </Card>
    </Box>
  ),
};

export const Disabled: Story = {
  args: {
    children: 'Disabled ButtonContainer',
    disabled: true,
  },
  render: (args) => (
    <Box mb={3} maxWidth={500}>
      <Card testId={testId}>
        <CardContent>
          <Box mb={3}></Box>
          <Box>
            <TextInput
              label="First Name"
              testId="first-name-input"
              value={''}
              onChange={function (_text: string): void {
                throw new Error('Function not implemented.');
              }}
            />
          </Box>
          <Box mt={3}>
            <Divider />
          </Box>
          <ButtonContainer {...args}>
            <Button testId="cancel" variant="outlined" color="secondary">
              Cancel
            </Button>
            <Button testId="save" variant="contained" color="primary">
              Save
            </Button>
          </ButtonContainer>
        </CardContent>
      </Card>
    </Box>
  ),
};
