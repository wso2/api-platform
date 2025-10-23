import { styled, RadioGroup as MuiRadioGroup } from '@mui/material';
import { type ComponentType } from 'react';

export interface StyledRadioGroupProps {
  className?: string;
  onClick?: (event: React.MouseEvent) => void;
  disabled?: boolean;
  row?: boolean;
  children?: React.ReactNode;
}

export const StyledRadioGroup: ComponentType<StyledRadioGroupProps> = styled(
  MuiRadioGroup,
  {
    shouldForwardProp: (prop) => !['disabled', 'row'].includes(prop as string),
  }
)<StyledRadioGroupProps>(({ theme, disabled, row }) => ({
  display: 'flex',
  flexDirection: row ? 'row' : 'column',
  gap: theme.spacing(1),
  opacity: disabled ? 0.6 : 1,
  cursor: disabled ? 'not-allowed' : 'default',
  pointerEvents: disabled ? 'none' : 'auto',
  root: {
    flexWrap: 'wrap',
  },
  rootRow: {
    flexDirection: 'row',
  },
  rootDefault: {
    gap: theme.spacing(2),
  },
}));
