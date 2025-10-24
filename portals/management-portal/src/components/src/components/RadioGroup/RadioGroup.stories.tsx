import type { Meta, StoryObj } from '@storybook/react';
import { RadioGroup } from './RadioGroup';
import { Radio } from '../Radio/Radio';

const meta: Meta<typeof RadioGroup> = {
  title: 'Choreo DS/RadioGroup',
  component: RadioGroup,
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
type Story = StoryObj<typeof RadioGroup>;

export const Default: Story = {
  args: {
    children: 'RadioGroup Content',
  },
  render: (args) => (
    <RadioGroup {...args}>
      <Radio value="option1" color="primary">
        Primary
      </Radio>
      <Radio value="option2" color="secondary">
        Secondary
      </Radio>
      <Radio value="option3" color="error">
        Error
      </Radio>
      <Radio value="option4" color="warning">
        Warning
      </Radio>
      <Radio value="option5" color="info">
        Info
      </Radio>
      <Radio value="option6" color="success">
        Success
      </Radio>
      <Radio value="option7" color="default">
        Default
      </Radio>
    </RadioGroup>
  ),
};

export const Disabled: Story = {
  args: {
    children: 'Disabled RadioGroup',
    disabled: true,
  },
  render: (args) => (
    <RadioGroup {...args}>
      <Radio value="option1" color="primary">
        Primary
      </Radio>
      <Radio value="option2" color="secondary">
        Secondary
      </Radio>
      <Radio value="option3" color="error">
        Error
      </Radio>
      <Radio value="option4" color="warning">
        Warning
      </Radio>
      <Radio value="option5" color="info">
        Info
      </Radio>
      <Radio value="option6" color="success">
        Success
      </Radio>
      <Radio value="option7" color="default">
        Default
      </Radio>
    </RadioGroup>
  ),
};

export const Horizontal: Story = {
  args: {
    children: 'Horizontal RadioGroup',
    row: true,
  },
  render: (args) => (
    <RadioGroup {...args}>
      <Radio value="option1" color="primary">
        Primary
      </Radio>
      <Radio value="option2" color="secondary">
        Secondary
      </Radio>
      <Radio value="option3" color="error">
        Error
      </Radio>
      <Radio value="option4" color="warning">
        Warning
      </Radio>
      <Radio value="option5" color="info">
        Info
      </Radio>
      <Radio value="option6" color="success">
        Success
      </Radio>
      <Radio value="option7" color="default">
        Default
      </Radio>
    </RadioGroup>
  ),
};
