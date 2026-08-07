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

import {
  AccordionSummary as MUIAccordionSummary,
  AccordionSummaryProps as MUIAccordionSummaryProps,
} from '@material-ui/core';
import clsx from 'clsx';
import useStyles from './Accordion.styles';
import { ChevronRight } from '@material-ui/icons';

interface AccordionSummaryProps extends MUIAccordionSummaryProps {
  testId: string;
  noPadding?: boolean;
  backgroundFilled?: boolean;
}

const AccordionSummary = (props: AccordionSummaryProps) => {
  const {
    children,
    testId,
    noPadding = false,
    expandIcon,
    backgroundFilled = false,
    className,
    ...rest
  } = props;
  const classes = useStyles();

  return (
    <MUIAccordionSummary
      expandIcon={expandIcon || <ChevronRight fontSize="inherit" rotate={90} />}
      data-cyid={testId}
      className={clsx(
        {
          [classes.accordionSummaryRoot]: true,
          [classes.accordionSummaryNoPaddingRoot]: noPadding,
          [classes.accordionSummaryBackgroundFilled]: backgroundFilled,
        },
        className
      )}
      {...rest}
    >
      {children}
    </MUIAccordionSummary>
  );
};

export default AccordionSummary;
