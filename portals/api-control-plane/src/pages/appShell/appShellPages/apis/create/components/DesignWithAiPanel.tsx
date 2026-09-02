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

import { Box, Typography } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage } from 'react-intl';

import { ComingSoon } from '../../../../../../components/ComingSoon';

const messages = defineMessages({
  detail: {
    id: 'api.create.designWithAi.detail',
    defaultMessage:
      'The skeleton on the right is a starting point — carry on and edit its operations by hand.',
  },
  feature: {
    id: 'api.create.designWithAi.feature',
    defaultMessage: 'Designing an API by chatting with AI',
  },
  title: {
    id: 'api.create.designWithAi.title',
    defaultMessage: 'Design with AI',
  },
});

/**
 * Left-hand panel of the "design from scratch" approach.
 *
 * A placeholder for now: the definition the step carries forward is the
 * skeleton the panel beside it already shows, and this is where the chat that
 * refines it will live.
 */
export const DesignWithAiPanel = () => (
  <Box>
    <Typography sx={{ fontWeight: 700 }} variant="subtitle1">
      <FormattedMessage {...messages.title} />
    </Typography>

    {/* `ComingSoon` sizes itself for a whole page (60vh); inside a panel it
        takes the height it is given instead. */}
    <Box sx={{ '& > *': { minHeight: 0, py: 4 } }}>
      <ComingSoon
        detail={<FormattedMessage {...messages.detail} />}
        feature={<FormattedMessage {...messages.feature} />}
      />
    </Box>
  </Box>
);
