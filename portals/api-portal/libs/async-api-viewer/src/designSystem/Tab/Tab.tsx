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
import { Box, Tab as MUITab, TabProps as MUITabProps } from '@material-ui/core';
import clsx from 'clsx';
import useStyles from './Tab.styles';

export type TabSize = 'small' | 'medium' | 'large';
export type IconPosition = 'left' | 'right' | 'bottom' | 'top';

interface TabProps extends MUITabProps {
  actions?: React.ReactNode;
  testId: string;
  size?: TabSize;
  iconPosition?: IconPosition;
}

const Tab = (props: TabProps) => {
  const {
    children,
    testId,
    size = 'medium',
    actions,
    iconPosition = 'left',
    ...rest
  } = props;
  const classes = useStyles();

  return (
    <Box className={classes.tabBox}>
      <MUITab
        disableFocusRipple
        disableTouchRipple
        {...rest}
        data-cyid={testId}
        className={clsx({
          [classes.tab]: true,
          [classes.tabSmall]: size === 'small',
          [classes.tabMedium]: size === 'medium',
          [classes.tabLarge]: size === 'large',
          [classes.iconLeft]: iconPosition === 'left',
          [classes.iconRight]: iconPosition === 'right',
          [classes.iconBottom]: iconPosition === 'bottom',
          [classes.iconTop]: iconPosition === 'top',
        })}
      >
        {children}
      </MUITab>
      {actions && <Box className={classes.tabAction}>{actions}</Box>}
    </Box>
  );
};

export default Tab;
