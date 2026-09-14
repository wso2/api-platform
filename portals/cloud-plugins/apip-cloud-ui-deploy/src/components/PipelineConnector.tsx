/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
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
