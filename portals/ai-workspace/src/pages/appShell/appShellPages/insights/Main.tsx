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

import {
  Button,
  Box,
  Grid,
  PageContent,
  PageTitle,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';

export default function Insights(): JSX.Element {
  return (
    <PageContent fullWidth>
      <Grid container spacing={2} sx={{ width: '100%', m: 0 }}>
        <Grid size={{ xs: 12 }}>
          <PageTitle>
            <PageTitle.Header>Insights</PageTitle.Header>
            <PageTitle.SubHeader>
              Usage analytics and traffic insights.
            </PageTitle.SubHeader>
          </PageTitle>
        </Grid>

        <Grid size={{ xs: 12 }}>
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              minHeight: 300,
              px: 3,
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: 2,
              backgroundColor: 'background.paper',
            }}
          >
            <Stack spacing={2} alignItems="center" sx={{ maxWidth: 560, textAlign: 'center' }}>
              <Typography variant="h5">
                Your API insights live in Moesif
              </Typography>

              <Typography variant="body2" color="text.secondary">
                Track usage trends, request activity, latency, and customer behavior from your
                Moesif analytics workspace.
              </Typography>

              <Button
                variant="contained"
                component="a"
                href="https://www.moesif.com/wrap/basic"
                target="_blank"
                rel="noopener noreferrer"
              >
                Open Moesif Insights
              </Button>
            </Stack>
          </Box>
        </Grid>
      </Grid>
    </PageContent>
  );
}
