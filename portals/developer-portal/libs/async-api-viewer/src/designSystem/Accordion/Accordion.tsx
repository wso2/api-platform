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
  Accordion as MUIAccordion,
  AccordionProps as MUIAccordionProps,
  styled,
} from '@material-ui/core';
import clsx from 'clsx';
import useStyles from './Accordion.styles';

export const StyledAccordion = styled(MUIAccordion)(({ theme }) => ({
  boxShadow: 'none',
  borderBottom: `1px solid ${theme.palette.grey[100]}`,
  '&.MuiAccordion-rounded': {
    '&:first-child': {
      borderTopLeftRadius: 8,
      borderTopRightRadius: 8,
    },
    '&:last-child': {
      borderBottomLeftRadius: 8,
      borderBottomRightRadius: 8,
    },
  },
  '&.MuiPaper-outlined': {
    border: 'none',
    borderTop: `1px solid ${theme.palette.grey[100]}`,
    backgroundColor: 'transparent',
    '&:first-child': {
      borderTop: 'none',
    },
    '& .MuiAccordionSummary-root': {
      padding: theme.spacing(1, 0),
    },
  },
  '&.Mui-expanded': {
    margin: 0,
  },
  '&:before': {
    backgroundColor: 'transparent',
  },
  '&:last-child': {
    borderBottomColor: 'transparent',
  },
}));

interface AccordionProps extends Omit<MUIAccordionProps, 'variant'> {
  contained?: boolean;
  testId: string;
  bordered?: boolean;
}

const Accordion = (props: AccordionProps) => {
  const { children, testId, contained = true, bordered, ...rest } = props;

  const variant = !contained ? 'outlined' : undefined;
  const classes = useStyles();
  return (
    <StyledAccordion
      data-cyid={testId}
      variant={variant}
      {...rest}
      classes={{
        root: clsx({
          [classes.accordion]: true,
          [classes.accordionBordered]: bordered,
        }),
      }}
    >
      {children}
    </StyledAccordion>
  );
};

export default Accordion;
