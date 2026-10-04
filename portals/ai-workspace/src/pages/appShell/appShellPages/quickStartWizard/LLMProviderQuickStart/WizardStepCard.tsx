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

import type { ReactNode } from 'react';
import {
  Box,
  Button,
  Card,
  CardContent,
  Divider,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { ArrowLeft, ArrowRight } from '@wso2/oxygen-ui-icons-react';

type WizardStepCardProps = {
  currentStep: number;
  totalSteps: number;
  title: string;
  description: string;
  children: ReactNode;
  onSkip: () => void;
  onBack?: () => void;
  onNext?: () => void;
  nextLabel: string;
  nextDisabled?: boolean;
  nextLoading?: boolean;
  nextIcon?: ReactNode;
  hideBack?: boolean;
  backDisabled?: boolean;
  footerStart?: ReactNode;
};

export default function WizardStepCard({
  currentStep,
  totalSteps,
  title,
  description,
  children,
  onSkip,
  onBack,
  onNext,
  nextLabel,
  nextDisabled = false,
  nextLoading = false,
  nextIcon,
  hideBack = false,
  backDisabled = false,
  footerStart,
}: WizardStepCardProps) {
  const progressWidth = `${((currentStep + 1) / totalSteps) * 100}%`;

  return (
    <Card
      sx={{
        borderRadius: 1.5,
        border: '1px solid',
        borderColor: 'divider',
        backgroundColor: 'background.paper',
        boxShadow: '0 10px 30px rgba(15, 23, 42, 0.05)',
        flex: 1,
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      <Box
        sx={{
          height: 4,
          width: '100%',
          backgroundColor: '#E9EDF2',
        }}
      >
        <Box
          sx={{
            height: '100%',
            width: progressWidth,
            borderRadius: 999,
            background: 'linear-gradient(90deg, #EA6A33 0%, #FF8B4B 100%)',
            transition: 'width 0.2s ease',
          }}
        />
      </Box>

      <CardContent sx={{ p: { xs: 3, md: 5 }, flex: 1, display: 'flex', flexDirection: 'column' }}>
        <Stack spacing={4} sx={{ flex: 1 }}>
          <Stack spacing={1}>
            <Typography
              variant="h3"
              sx={{
                fontSize: { xs: '1.5rem', md: '2rem' },
                fontWeight: 700,
                lineHeight: 1,
              }}
            >
              {title}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {description}
            </Typography>
          </Stack>

          <Box sx={{ flex: 1, minHeight: 360 }}>{children}</Box>

          <Divider />

          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: 2,
              flexWrap: 'wrap',
            }}
          >
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
              <Button variant="text" onClick={onSkip} sx={{ px: 0 }}>
                Skip and go to Console
              </Button>
              {footerStart}
            </Box>

            <Stack direction="row" spacing={1.5}>
              {!hideBack ? (
                <Button
                  variant="outlined"
                  color="secondary"
                  onClick={onBack}
                  disabled={backDisabled}
                  startIcon={<ArrowLeft size={16} />}
                >
                  Back
                </Button>
              ) : null}
              <Button
                variant="contained"
                onClick={onNext}
                disabled={nextDisabled || nextLoading}
                endIcon={nextIcon ?? <ArrowRight size={16} />}
              >
                {nextLabel}
              </Button>
            </Stack>
          </Box>
        </Stack>
      </CardContent>
    </Card>
  );
}

