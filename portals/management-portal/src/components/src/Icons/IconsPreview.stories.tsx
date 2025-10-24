import { StoryFn } from '@storybook/react';
import IconsPreview from './IconsPreview';
import { Box } from '@mui/material'; // More on default export: https://storybook.js.org/docs/react/writing-stories/introduction#default-export
export default {
  title: 'Extras/Icons',
  component: IconsPreview,
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
}; // More on component templates: https://storybook.js.org/docs/react/writing-stories/introduction#using-args
const Template: StoryFn = (args) => (
  <Box height={1} width={1} display="flex">
    <IconsPreview {...args} />
  </Box>
);
export const IconPreviewList = Template.bind({}) as StoryFn<
  typeof IconsPreview
>;
// More on args: https://storybook.js.org/docs/react/writing-stories/args
IconPreviewList.args = {};
