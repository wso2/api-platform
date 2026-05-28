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

// InsightsSection.tsx (updated: LineChart uses Charts.LineChart)

import React, { useMemo } from 'react';
import {
  Box,
  Card,
  CardContent,
  CardHeader,
  Divider,
  Grid,
  Stack,
  StatCard,
  Typography,
} from '@wso2/oxygen-ui';
// import { Charts } from '@wso2/oxygen-ui';
import { Activity, Clock, CheckCircle2 } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';

type InsightPoint = {
  label: string;
  value: number;
};

type InsightSplit = {
  label: string;
  value: number;
  color: string;
};

type InsightSummary = {
  label: string;
  value: string;
};

type Insights = {
  requestTrend: InsightPoint[];
  trafficSplit: InsightSplit[];
  summary: InsightSummary[];
};

const summaryIconMap: Record<string, React.ReactElement> = {
  'Total Requests': <Activity size={24} />,
  'Avg Latency': <Clock size={24} />,
  'Success Rate': <CheckCircle2 size={24} />,
};

const DonutChart = ({ data }: { data: InsightSplit[] }) => {
  const total = data.reduce((sum, slice) => sum + slice.value, 0) || 1;
  let cumulative = 0;

  return (
    <Box
      component="svg"
      viewBox="0 0 42 42"
      sx={{ width: '100%', height: '100%' }}
    >
      <circle
        cx="21"
        cy="21"
        r="15.9155"
        fill="none"
        stroke="#e2e8f0"
        strokeWidth="6"
      />
      {data.map((slice) => {
        const percent = (slice.value / total) * 100;
        const dashArray = `${percent} ${100 - percent}`;
        const dashOffset = 25 - cumulative;
        cumulative += percent;

        return (
          <circle
            key={slice.label}
            cx="21"
            cy="21"
            r="15.9155"
            fill="none"
            stroke={slice.color}
            strokeWidth="6"
            strokeDasharray={dashArray}
            strokeDashoffset={dashOffset}
          />
        );
      })}
    </Box>
  );
};

export default function InsightsSection({ insights }: { insights: Insights }) {
  // Convert your existing requestTrend [{label,value}] -> dataset [{label, requests}]
  const requestTrendDataset = useMemo(
    () =>
      insights.requestTrend.map((p) => ({ day: p.label, requests: p.value })),
    [insights.requestTrend]
  );

  return (
    <Grid size={{ xs: 12 }} sx={{ width: '100%' }}>
      <Card sx={{ width: '100%' }}>
        <CardHeader
          title="Insights"
          subheader="Sample usage for the last 7 days"
          slotProps={{
            title: { sx: { fontSize: '1rem', fontWeight: 700, marginBottom: 0.3 } },
            subheader: { sx: { fontSize: '0.82rem' } },
          }}
        />
        <CardContent>
          <Grid container spacing={3} sx={{ width: '100%', m: 0 }}>
            <Grid size={{ xs: 12 }} sx={{ width: '100%' }}>
              <Grid container spacing={2} sx={{ width: '100%', m: 0 }}>
                {insights.summary.map((item) => (
                  <Grid key={item.label} size={{ xs: 12, sm: 6, md: 6, lg: 4 }}>
                    <StatCard
                      value={item.value}
                      label={item.label}
                      icon={summaryIconMap[item.label]}
                      iconColor="primary"
                    />
                  </Grid>
                ))}
              </Grid>
            </Grid>
            <Grid size={{ xs: 12, md: 7 }} sx={{ width: '100%' }}>
              <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 1 }}>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.overview.InsightsSection.request.trend"
                  defaultMessage="Request trend"
                />
              </Typography>

              <Box sx={{ width: '100%' }}>
                {/* <Charts.LineChart
                  dataset={requestTrendDataset}
                  xAxis={[{ scaleType: 'band', dataKey: 'day' }]}
                  series={[{ dataKey: 'requests', label: 'Requests' }]}
                  height={260}
                /> */}
              </Box>
            </Grid>
            <Grid size={{ xs: 12, md: 5 }} sx={{ width: '100%' }}>
              <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 1 }}>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.overview.InsightsSection.traffic.split"
                  defaultMessage="Traffic split"
                />
              </Typography>

              <Box
                sx={{
                  display: 'flex',
                  gap: 3,
                  alignItems: 'center',
                  flexWrap: 'wrap',
                }}
              >
                <Box sx={{ height: 200, width: 200 }}>
                  <DonutChart data={insights.trafficSplit} />
                </Box>

                <Stack spacing={1.5}>
                  {insights.trafficSplit.map((slice) => (
                    <Stack
                      key={slice.label}
                      direction="row"
                      spacing={1.5}
                      alignItems="center"
                    >
                      <Box
                        sx={{
                          height: 12,
                          width: 12,
                          borderRadius: '50%',
                          backgroundColor: slice.color,
                        }}
                      />
                      <Typography variant="body2" sx={{ fontWeight: 600 }}>
                        {slice.label}
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.overview.InsightsSection.traffic.percentage"
                          defaultMessage="{value}%"
                          values={{ value: slice.value }}
                        />
                      </Typography>
                    </Stack>
                  ))}
                </Stack>
              </Box>
            </Grid>
          </Grid>
        </CardContent>
      </Card>
    </Grid>
  );
}
