/* eslint-disable react/jsx-props-no-spreading */
/*
 * Copyright (c) 2023, WSO2 LLC. (http://www.wso2.com). All Rights Reserved.
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

import { Box, Divider, Link, Typography } from '@material-ui/core';
import { TooltipProps } from '@material-ui/core/Tooltip';
import clsx from 'clsx';
import { useStyles, useToolTipBaseStyles } from './Tooltip.styles';
import TooltipBase from './TooltipBase';

interface InfoTooltipProps extends Omit<TooltipProps, 'content'> {
  darken?: boolean;
  disabled?: boolean;
  heading?: string | React.ReactNode;
  content?: string | React.ReactNode;
  example?: string;
  action?: { link: string; text: string };
}

export default function Tooltip(props: Partial<InfoTooltipProps>) {
  const {
    children,
    heading,
    disabled,
    content,
    example,
    action,
    darken = true,
    ...restProps
  } = props;
  const classes = useStyles();
  const baseClasses = useToolTipBaseStyles();

  const infoTooltipFragment = (
    <Box p={0.5}>
      {heading && (
        <Box mb={content ? 1 : 0}>
          <Typography variant="h5">{heading}</Typography>
        </Box>
      )}
      {content && <Typography variant="body2">{content}</Typography>}
      {(action || example) && <Divider className={classes.divider} />}
      {example && (
        <Typography
          className={clsx({
            [classes.exampleContent]: true,
            [classes.exampleContentDark]: darken,
            [classes.exampleContentLight]: !darken,
          })}
          variant="body2"
        >
          Eg: {example}
        </Typography>
      )}
      {action && (
        <Link
          href={action.link}
          className={classes.buttonLink}
          target="_blank"
          rel="noreferrer"
        >
          {action.text}
        </Link>
      )}
    </Box>
  );

  if (!children) return null;

  return (
    <TooltipBase
      title={infoTooltipFragment}
      disabled={disabled}
      classes={
        darken
          ? {
              tooltip: baseClasses.infoTooltipDark,
              arrow: baseClasses.infoArrowDark,
            }
          : {
              tooltip: baseClasses.infoTooltipLight,
              arrow: baseClasses.infoArrowLight,
            }
      }
      {...restProps}
    >
      {children}
    </TooltipBase>
  );
}
