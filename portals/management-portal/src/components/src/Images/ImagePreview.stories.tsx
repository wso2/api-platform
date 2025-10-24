import { StoryFn } from '@storybook/react';
import ImagePreview from './ImagePreview';
import { Box } from '@mui/material'; // More on default export: https://storybook.js.org/docs/react/writing-stories/introduction#default-export
export default {
  title: 'Extras/Images',
  component: ImagePreview,
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
    <ImagePreview {...args} />
  </Box>
);
export const ImagePreviewList = Template.bind({}) as StoryFn<
  typeof ImagePreview
>;
// More on args: https://storybook.js.org/docs/react/writing-stories/args
ImagePreviewList.args = {};
