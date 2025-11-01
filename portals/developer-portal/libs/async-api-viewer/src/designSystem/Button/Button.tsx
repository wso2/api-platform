/* eslint-disable react/jsx-props-no-spreading */
/*
 * Copyright (c) 2022, WSO2 LLC. (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC. and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein is strictly forbidden, unless permitted by WSO2 in accordance with
 * the WSO2 Commercial License available at http://wso2.com/licenses.
 * For specific language governing the permissions and limitations under
 * this license, please see the license as well as any agreement you’ve
 * entered into with WSO2 governing the purchase of this software and any
 * associated services.
 */
import {
  Button as MUIButton,
  ButtonProps as MUIButtonProps,
} from '@material-ui/core';
import clsx from 'clsx';
import useStyles from './Button.styles';

export type ButtonColor =
  | 'primary'
  | 'secondary'
  | 'error'
  | 'success'
  | 'warning';

export type ButtonSize = 'medium' | 'small' | 'tiny';
export type ButtonProps<C extends React.ElementType = 'button'> = Omit<
  React.ComponentProps<C>,
  'variant'
> &
  Omit<
    MUIButtonProps,
    | 'color'
    | 'disableTouchRipple'
    | 'focusRipple'
    | 'centerRipple'
    | 'disableRipple'
    | 'variant'
    | 'size'
  > & {
    pill?: boolean;
    color?: ButtonColor;
    component?: C;
    testId: string;
    variant?: 'contained' | 'subtle' | 'outlined' | 'text' | 'link';
    size?: ButtonSize;
  };

const Button = <C extends React.ElementType = 'button'>(
  props: ButtonProps<C>
) => {
  const {
    children,
    color = 'primary',
    variant = 'contained',
    pill = false,
    fullWidth = false,
    size = 'medium',
    disabled = false,
    testId,
    ...rest
  } = props;
  const classes = useStyles();
  const isPrimary = color === 'primary';
  const isSecondary = color === 'secondary';
  const isError = color === 'error';
  const isSuccess = color === 'success';
  const isWarning = color === 'warning';

  const isText = variant === 'text';
  const isOutlined = variant === 'outlined';
  const isContained = variant === 'contained';
  const isSubtle = variant === 'subtle';
  const isLink = variant === 'link';

  const isSmall = size === 'small';
  const isTiny = size === 'tiny';

  return (
    <MUIButton
      classes={{
        root: clsx({
          [classes.commons]: true,
          [classes.fullWidth]: fullWidth,
          [classes.disabled]: disabled,
          [classes.outlined]: isOutlined,
          [classes.contained]: isContained,
          [classes.text]: isText,
          [classes.subtle]: isSubtle,
          [classes.link]: isLink,

          [classes.pill]: pill,

          [classes.smallPill]: pill && isSmall,
          [classes.small]: isSmall,
          [classes.smallLink]: isSmall && isLink,

          [classes.tinyPill]: pill && isTiny,
          [classes.tiny]: isTiny,
          [classes.tinyLink]: isTiny && isLink,

          [classes.primaryContained]: isPrimary && isContained,
          [classes.primaryText]: isPrimary && isText,
          [classes.primaryOutlined]: isPrimary && isOutlined,
          [classes.primarySubtle]: isPrimary && isSubtle,
          [classes.primaryLink]: isPrimary && isLink,

          [classes.secondaryContained]: isSecondary && isContained,
          [classes.secondaryText]: isSecondary && isText,
          [classes.secondaryOutlined]: isSecondary && isOutlined,
          [classes.secondarySubtle]: isSecondary && isSubtle,
          [classes.secondaryLink]: isSecondary && isLink,

          [classes.successContained]: isSuccess && isContained,
          [classes.successText]: isSuccess && isText,
          [classes.successOutlined]: isSuccess && isOutlined,
          [classes.successSubtle]: isSuccess && isSubtle,
          [classes.successLink]: isSuccess && isLink,

          [classes.errorContained]: isError && isContained,
          [classes.errorText]: isError && isText,
          [classes.errorOutlined]: isError && isOutlined,
          [classes.errorSubtle]: isError && isSubtle,
          [classes.errorLink]: isError && isLink,

          [classes.warningContained]: isWarning && isContained,
          [classes.warningText]: isWarning && isText,
          [classes.warningOutlined]: isWarning && isOutlined,
          [classes.warningSubtle]: isWarning && isSubtle,
          [classes.warningLink]: isWarning && isLink,
        }),
        startIcon: clsx({
          [classes.startIcon]: true,
          [classes.startIconSmall]: isSmall,
          [classes.startIconTiny]: isTiny,
        }),
        endIcon: clsx({
          [classes.endIcon]: true,
          [classes.endIconSmall]: isSmall,
          [classes.endIconTiny]: isTiny,
        }),
      }}
      disableFocusRipple
      disableRipple
      centerRipple={false}
      data-cyid={`${testId}-button`}
      disabled={disabled}
      {...rest}
    >
      {children}
    </MUIButton>
  );
};

export default Button;
