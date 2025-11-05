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
  AccordionDetails as MUIAccordionDetails,
  AccordionDetailsProps as MUIAccordionDetailsProps,
  styled,
} from '@material-ui/core';

export const StyledAccordionDetails = styled(MUIAccordionDetails)(() => ({
  padding: 0,
}));

interface AccordionDetailsProps extends MUIAccordionDetailsProps {
  testId: string;
}

const AccordionDetails = (props: AccordionDetailsProps) => {
  const { testId, children, ...rest } = props;

  return (
    <StyledAccordionDetails data-cyid={testId} {...rest}>
      {children}
    </StyledAccordionDetails>
  );
};

export default AccordionDetails;
