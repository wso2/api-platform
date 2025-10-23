import React from 'react';
import { StyledRadioIndicator } from './Radio.styled';

export const RadioIndicator = React.forwardRef<HTMLDivElement>((props) => {
  return (
    <StyledRadioIndicator
      {...props}
      disableRipple={true}
      disableFocusRipple={true}
      disableTouchRipple={true}
    />
  );
});

RadioIndicator.displayName = 'RadioIndicator';
