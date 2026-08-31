/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import type { FC } from 'react';
import { Box } from '@wso2/oxygen-ui';

/**
 * A thin horizontal link between two pipeline cards. Cards vary in height
 * (collapsed/expanded gateway rows), so centering this against "the tallest
 * card" would shift depending on which sibling happens to be tallest at any
 * moment. Instead it pins to a fixed offset from the top that lines up with
 * the middle of every card's header, regardless of the cards' own heights.
 */
const PipelineConnector: FC = () => (
  <Box
    aria-hidden
    sx={{
      flex: '0 0 32px',
      width: 32,
      height: '1px',
      bgcolor: 'divider',
      alignSelf: 'flex-start',
      mt: '28px',
    }}
  />
);

export default PipelineConnector;
