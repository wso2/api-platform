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
import { Box, Tabs, Typography } from '@material-ui/core';
import clsx from 'clsx';
import Button from '../Button/Button';
import Tab, { IconPosition, TabSize } from './Tab';
import useStyles from './Tab.styles';

function a11yProps(index: number) {
  return {
    id: `simple-tab-${index}`,
    'aria-controls': `simple-tabpanel-${index}`,
  };
}

export interface TabItem {
  name: string;
  envId?: string;
  disabled?: boolean;
  actions?: React.ReactNode;
  onClick?: () => void;
  buttonProps?: {
    startIcon?: React.ReactNode;
    onClick: () => void;
  };
  hidden?: boolean;
  icon?: JSX.Element;
  iconPosition?: IconPosition;
}

interface TabCardProps {
  tabItems: TabItem[];
  children: React.ReactNode;
  handleChange?: (event: React.ChangeEvent<{}>, newValue: number) => void;
  value: number;
  centered?: boolean;
  disabled?: boolean;
  tabActions?: React.ReactNode;
  title?: React.ReactNode;
  testId: string;
  className?: string;
  fullHeight?: boolean;
  size?: TabSize;
}

const TabCard = (props: TabCardProps) => {
  const {
    tabItems,
    children,
    handleChange,
    value,
    centered = false,
    tabActions,
    title,
    testId,
    className,
    fullHeight,
    size = 'medium',
  } = props;
  const classes = useStyles();
  return (
    <Box
      className={clsx(classes.tabCard, className, {
        [classes.fullHeight]: fullHeight,
      })}
    >
      <Box className={classes.tabHeading}>
        {title && (
          <Box textAlign={centered && 'center'} mb={3}>
            <Typography variant="h2" gutterBottom>
              {title}
            </Typography>
          </Box>
        )}
        <Box
          className={clsx(classes.tabActionsWrap, {
            [classes.tabActionsWrapCenter]: centered,
          })}
        >
          <Tabs
            value={value}
            onChange={handleChange}
            centered={centered}
            indicatorColor="primary"
            textColor="primary"
            className={classes.tabs}
            data-cyid={testId}
          >
            {tabItems.map((tabItem, idx) => {
              const testIdSuffix = tabItem.name
                .toLowerCase()
                .replace(/\s+/g, '-');

              if (tabItem.hidden) {
                return null;
              }

              if (tabItem.buttonProps) {
                return (
                  <Button
                    key={tabItem.name}
                    variant="link"
                    startIcon={tabItem.buttonProps.startIcon}
                    onClick={tabItem.buttonProps.onClick}
                    color="primary"
                    testId={`${testId}-${testIdSuffix}`}
                    disabled={tabItem.disabled}
                  >
                    {tabItem.name}
                  </Button>
                );
              }

              return (
                <Tab
                  label={tabItem.name}
                  {...a11yProps(idx)}
                  key={tabItem.name}
                  disabled={tabItem.disabled}
                  actions={tabItem.actions}
                  testId={`${testId}-${testIdSuffix}`}
                  onClick={tabItem.onClick}
                  size={size}
                  icon={tabItem?.icon}
                  iconPosition={tabItem?.iconPosition}
                />
              );
            })}
          </Tabs>
          {tabActions && <Box className={classes.tabActions}>{tabActions}</Box>}
        </Box>
      </Box>
      <Box className={classes.tabBody}> {children}</Box>
    </Box>
  );
};

export default TabCard;
