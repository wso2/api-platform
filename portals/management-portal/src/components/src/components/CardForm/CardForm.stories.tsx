import type { Meta, StoryObj } from '@storybook/react';
import { CardForm } from './CardForm';
import { Box } from '@mui/material';
import { Button, Card, CardContent, TextInput } from '../index copy';
import { ButtonContainer } from '../ButtonContainer';
import { CardHeading } from '../Card/SubComponents/CardHeading';
import { useState } from 'react';

const meta: Meta<typeof CardForm> = {
  title: 'Components/Card/CardForm',
  component: CardForm,
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
type Story = StoryObj<typeof CardForm>;

// Create a wrapper component to use hooks
const CardFormStory = (_args: any) => {
  const [apiName, setApiName] = useState('');
  const [apiDescription, setApiDescription] = useState('');

  return (
    <Box>
      <Card testId="form">
        <CardHeading
          testId="card-form-heading"
          title="Create API Proxy"
          onClose={() => {}}
          isForm={true}
        />
        <CardContent>
          <Box maxWidth="30rem">
            <Box mb={3}>
              <TextInput
                placeholder="API Name"
                testId="api-name"
                value={apiName}
                onChange={(text: string) => setApiName(text)}
              />
            </Box>
            <Box>
              <TextInput
                placeholder="API Proxy Description"
                testId="api-description"
                value={apiDescription}
                onChange={(text: string) => setApiDescription(text)}
              />
            </Box>
          </Box>
        </CardContent>
        <CardContent>
          <ButtonContainer align="left" testId="card-form">
            <Button
              variant="contained"
              color="primary"
              onClick={() => {}}
              testId="btn-create"
            >
              Create
            </Button>
            <Button
              onClick={() => {}}
              variant="contained"
              color="secondary"
              testId="btn-back"
            >
              Back
            </Button>
          </ButtonContainer>
        </CardContent>
      </Card>
    </Box>
  );
};

export const Default: Story = {
  args: {
    children: 'CardForm Content',
  },
  render: (args) => <CardFormStory {...args} />,
};
