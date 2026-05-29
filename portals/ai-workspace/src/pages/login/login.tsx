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

import { useAuthContext } from '@asgardeo/auth-react';
import {
  Box,
  ColorSchemeImage,
  ColorSchemeToggle,
  Divider,
  Grid,
  Link,
  Paper,
  ParticleBackground,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import loginImage from '../../assets/images/login.svg';
import loginImageInverted from '../../assets/images/login-inverted.svg';
import LoginBox from '../../Components/login/LoginBox';
import {
  AppWindow,
  Cloud,
  Cog,
  FlaskConical,
} from '@wso2/oxygen-ui-icons-react';
import Logo from '../../Components/Logo';
import { Outlet } from 'react-router-dom';
import { FormattedMessage } from 'react-intl';

export default function Login() {
  const sloganListItemsAIWorkspace: {
    icon: JSX.Element;
    title: string;
  }[] = [
    {
      icon: <Cloud className="text-muted-foreground" />,
      title: 'Connect multiple LLM providers and models',
    },
    {
      icon: <AppWindow className="text-muted-foreground" />,
      title: 'Build, run, and iterate AI workflows',
    },
    {
      icon: <Cog className="text-muted-foreground" />,
      title: 'Configure prompts, tools, and runtime settings',
    },
    {
      icon: <FlaskConical className="text-muted-foreground" />,
      title: 'Test, evaluate, and compare model responses',
    },
  ];

  const Slogan = () => (
    <>
      <Typography variant="h3" sx={{ fontWeight: 'bold', mb: 0 }}>
        <FormattedMessage
          id="aiWorkspace.pages.login.login.ai.workspace.for.building.and.running.intelligent.applications"
          defaultMessage={
            'AI Workspace for Building and Running Intelligent Applications'
          }
        />
      </Typography>

      <Typography variant="body1" sx={{ color: 'text.secondary' }}>
        <FormattedMessage
          id="aiWorkspace.pages.login.login.a.unified.workspace.to.experiment.with.llms.manage.providers.and.build.ai.powere"
          defaultMessage={
            'A unified workspace to experiment with LLMs, manage providers, and build AI-powered solutions'
          }
        />
      </Typography>

      <Stack sx={{ gap: 2 }}>
        {sloganListItemsAIWorkspace.map((item) => (
          <Stack
            key={item.title}
            direction="row"
            sx={{ gap: 2, alignItems: 'flex-start' }}
          >
            {item.icon}
            <Box>
              <Typography gutterBottom sx={{ fontWeight: 'medium' }}>
                {item.title}
              </Typography>
            </Box>
          </Stack>
        ))}
      </Stack>
    </>
  );

  const BackgroundImage = () => (
    <ColorSchemeImage
      src={{
        light: loginImage,
        dark: loginImageInverted,
      }}
      alt={{
        light: 'Login Screen Image (Light)',
        dark: 'Login Screen Image (Dark)',
      }}
      height={450}
      width="auto"
      sx={{ position: 'absolute', bottom: 50, right: -100 }}
    />
  );

  return (
    <Box sx={{ height: '100vh', display: 'flex' }}>
      <ParticleBackground opacity={0.5} />
      <Grid container sx={{ flex: 1 }}>
        <Grid
          size={{ xs: 12, md: 8 }}
          sx={{
            display: 'flex',
            alignItems: 'flex-start',
            justifyContent: 'flex-start',
            padding: 18,
            textAlign: 'left',
            position: 'relative',
          }}
        >
          <Box>
            <Stack
              direction="column"
              alignItems="flex-start"
              gap={2}
              maxWidth={580}
              display={{ xs: 'none', md: 'flex' }}
            >
              <Box sx={{ my: 1 }}>
                <Logo height={60}/>
              </Box>
              <Slogan />
            </Stack>
          </Box>
          <BackgroundImage />
        </Grid>

        <Grid size={{ xs: 12, md: 4 }}>
          <Paper
            sx={{
              display: 'flex',
              padding: 4,
              width: '100%',
              height: '100%',
              flexDirection: 'column',
              position: 'relative',
              textAlign: 'left',
            }}
          >
            <Box display="flex" justifyContent="flex-end">
              <ColorSchemeToggle />
            </Box>
            <Box
              sx={{
                alignItems: 'center',
                justifyContent: 'center',
                padding: 4,
                width: '100%',
                maxWidth: 500,
                margin: 'auto',
              }}
            >
              <LoginBox />
              <Box component="footer" sx={{ mt: 8 }}>
                <Typography sx={{ textAlign: 'center' }}>
                  <FormattedMessage
                    id="aiWorkspace.pages.login.login.copyright"
                    defaultMessage={'© Copyright'}
                  />{' '}
                  {new Date().getFullYear()}
                </Typography>
                <Stack
                  direction="row"
                  justifyContent="center"
                  sx={{ mt: 2 }}
                  spacing={1}
                >
                  <Link
                    href="https://wso2.com/bijira/privacy-policy"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    <FormattedMessage
                      id="aiWorkspace.pages.login.login.privacy.policy"
                      defaultMessage={'Privacy Policy'}
                    />
                  </Link>
                  <Divider orientation="vertical" flexItem sx={{ mx: 1 }} />
                  <Link
                    href="https://wso2.com/bijira/terms-of-use"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    <FormattedMessage
                      id="aiWorkspace.pages.login.login.terms.of.use"
                      defaultMessage={'Terms of Use'}
                    />
                  </Link>
                </Stack>
              </Box>
            </Box>
          </Paper>
        </Grid>
      </Grid>
    </Box>
  );
}
