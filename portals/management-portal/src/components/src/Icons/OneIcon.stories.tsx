import { StoryFn } from '@storybook/react';
import { AddIcon } from './generated'; // More on default export: https://storybook.js.org/docs/react/writing-stories/introduction#default-export
export default {
  title: 'Extras/Icons',
  component: AddIcon,
  argTypes: {
    fontSize: {
      control: {
        type: 'select',
        options: ['medium', 'small', 'default', 'inherit', 'large'],
      },
    },
    color: {
      control: {
        type: 'select',
        options: [
          'inherit',
          'disabled',
          'action',
          'primary',
          'secondary',
          'error',
        ],
      },
    },
  },
};
const PreviewTemplate: StoryFn = (args) => <AddIcon {...args} />;
export const SingleIcon = PreviewTemplate.bind({}) as StoryFn<typeof AddIcon>;
