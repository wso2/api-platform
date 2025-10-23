import type { Meta, StoryObj } from '@storybook/react';
import { Chip } from './Chip';
import { OrganizationIcon } from '@design-system/Icons';

const meta: Meta<typeof Chip> = {
  title: 'Components/Chip',
  component: Chip,
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
      control: 'radio',
      options: ['small', 'medium', 'large'],
      description: 'Size of the chip',
      table: {
        type: { summary: 'small | medium | large' },
        defaultValue: { summary: 'medium' },
      },
    },
    variant: {
      control: 'radio',
      options: ['filled', 'outlined'],
      description: 'Variant of the chip',
      table: {
        type: { summary: 'filled | outlined' },
        defaultValue: { summary: 'filled' },
      },
    },
    color: {
      control: 'select',
      options: [
        'default',
        'primary',
        'secondary',
        'error',
        'warning',
        'info',
        'success',
      ],
      description: 'Color of the chip',
      table: {
        type: {
          summary:
            'default | primary | secondary | error | warning | info | success',
        },
        defaultValue: { summary: 'default' },
      },
    },
    onClick: {
      action: 'clicked',
      description: 'Click event handler',
    },
  },
};

export default meta;
type Story = StoryObj<typeof Chip>;

export const Default: Story = {
  args: {
    label: 'Default Chip',
    testId: 'default-chip',
    size: 'medium',
    variant: 'filled',
    color: 'default',
  },
};

export const Disabled: Story = {
  args: {
    children: 'Disabled Chip',
    disabled: true,
    label: 'Disabled Chip',
    testId: 'disabled-chip',
  },
};

export const WithIcon: Story = {
  args: {
    children: 'Chip with Icon',
    variant: 'filled',
    testId: 'icon-chip',
  },
  render: (args) => (
    <Chip {...args} icon={<OrganizationIcon />} label="with icon" />
  ),
  parameters: {
    docs: {
      description: {
        story: 'This chip includes an icon alongside the text.',
      },
    },
  },
};

export const Sizes: Story = {
  render: () => (
    <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
      <Chip size="small" label="Small" testId="small-chip" />
      <Chip size="medium" label="Medium" testId="medium-chip" />
      <Chip size="large" label="Large" testId="large-chip" />
    </div>
  ),
  parameters: {
    docs: {
      description: {
        story: 'Chips in different sizes: small, medium, and large.',
      },
    },
  },
};

export const SizesWithIcons: Story = {
  render: () => (
    <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
      <Chip
        size="small"
        label="Small"
        testId="small-icon-chip"
        icon={<OrganizationIcon />}
      />
      <Chip
        size="medium"
        label="Medium"
        testId="medium-icon-chip"
        icon={<OrganizationIcon />}
      />
      <Chip
        size="large"
        label="Large"
        testId="large-icon-chip"
        icon={<OrganizationIcon />}
      />
    </div>
  ),
  parameters: {
    docs: {
      description: {
        story:
          'Chips with icons in different sizes. Notice how the icon scales proportionally with the chip size.',
      },
    },
  },
};

export const Variants: Story = {
  render: () => (
    <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
      <Chip variant="filled" label="Filled" testId="filled-chip" />
      <Chip variant="outlined" label="Outlined" testId="outlined-chip" />
    </div>
  ),
  parameters: {
    docs: {
      description: {
        story: 'Chips with different variants: filled and outlined.',
      },
    },
  },
};

export const Colors: Story = {
  render: () => (
    <div
      style={{
        display: 'flex',
        gap: '8px',
        alignItems: 'center',
        flexWrap: 'wrap',
      }}
    >
      <Chip color="default" label="Default" testId="default-color-chip" />
      <Chip color="primary" label="Primary" testId="primary-color-chip" />
      <Chip color="secondary" label="Secondary" testId="secondary-color-chip" />
      <Chip color="success" label="Success" testId="success-color-chip" />
      <Chip color="error" label="Error" testId="error-color-chip" />
      <Chip color="warning" label="Warning" testId="warning-color-chip" />
      <Chip color="info" label="Info" testId="info-color-chip" />
    </div>
  ),
  parameters: {
    docs: {
      description: {
        story: 'Chips with different color variants.',
      },
    },
  },
};

export const ColorVariantsMatrix: Story = {
  render: () => (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
      <div>
        <h4>Filled Variant</h4>
        <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
          <Chip
            variant="filled"
            color="default"
            label="Default"
            testId="filled-default"
          />
          <Chip
            variant="filled"
            color="primary"
            label="Primary"
            testId="filled-primary"
          />
          <Chip
            variant="filled"
            color="secondary"
            label="Secondary"
            testId="filled-secondary"
          />
          <Chip
            variant="filled"
            color="success"
            label="Success"
            testId="filled-success"
          />
          <Chip
            variant="filled"
            color="error"
            label="Error"
            testId="filled-error"
          />
          <Chip
            variant="filled"
            color="warning"
            label="Warning"
            testId="filled-warning"
          />
          <Chip
            variant="filled"
            color="info"
            label="Info"
            testId="filled-info"
          />
        </div>
      </div>
      <div>
        <h4>Outlined Variant</h4>
        <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
          <Chip
            variant="outlined"
            color="default"
            label="Default"
            testId="outlined-default"
          />
          <Chip
            variant="outlined"
            color="primary"
            label="Primary"
            testId="outlined-primary"
          />
          <Chip
            variant="outlined"
            color="secondary"
            label="Secondary"
            testId="outlined-secondary"
          />
          <Chip
            variant="outlined"
            color="success"
            label="Success"
            testId="outlined-success"
          />
          <Chip
            variant="outlined"
            color="error"
            label="Error"
            testId="outlined-error"
          />
          <Chip
            variant="outlined"
            color="warning"
            label="Warning"
            testId="outlined-warning"
          />
          <Chip
            variant="outlined"
            color="info"
            label="Info"
            testId="outlined-info"
          />
        </div>
      </div>
    </div>
  ),
  parameters: {
    docs: {
      description: {
        story: 'Matrix showing all color and variant combinations.',
      },
    },
  },
};
