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

import { Box, Button, Typography } from '@wso2/oxygen-ui';

const STYLES = `
.cs-sand{fill:#ff6a2b;transform-box:fill-box;transform-origin:50% 100%}
.cs-sand-top{animation:cs-top 3s ease-in infinite}
.cs-sand-bot{animation:cs-bot 3s ease-in infinite}
.cs-stream{fill:#ff7a3d;animation:cs-stream 3s linear infinite}
.cs-glass{fill:none;stroke:currentColor;stroke-opacity:.55;stroke-width:2.5;stroke-linecap:round}
.cs-cap{fill:currentColor;fill-opacity:.2;stroke:currentColor;stroke-opacity:.45;stroke-width:1.5}
.cs-hourglass{overflow:visible;animation:cs-flip 3s cubic-bezier(.6,0,.3,1) infinite}
.cs-glow{position:absolute;inset:0;border-radius:50%;background:radial-gradient(circle,#ff6a2b33 0%,transparent 65%);animation:cs-glow 3.5s ease-in-out infinite}
.cs-art{animation:cs-rise .6s ease-out both}
.cs-bar span{animation:cs-bar 1.8s ease-in-out infinite}
@keyframes cs-flip{0%,88%{transform:rotate(0)}100%{transform:rotate(180deg)}}
@keyframes cs-top{0%{transform:scaleY(1)}85%,100%{transform:scaleY(0)}}
@keyframes cs-bot{0%{transform:scaleY(0)}85%,100%{transform:scaleY(1)}}
@keyframes cs-stream{0%,84%{opacity:1}86%,100%{opacity:0}}
@keyframes cs-glow{0%,100%{opacity:.45;transform:scale(.92)}50%{opacity:.9;transform:scale(1.05)}}
@keyframes cs-rise{from{opacity:0;transform:translateY(14px)}to{opacity:1;transform:none}}
@keyframes cs-bar{0%{transform:translateX(-100%)}100%{transform:translateX(250%)}}
@media (prefers-reduced-motion:reduce){.cs-page *{animation:none!important}}
`;

export interface ComingSoonProps {
  title?: string;
  message?: string;
  /** Shows a "Go to home" button that calls this; omitted, there is no button. */
  onGoHome?: () => void;
}

export default function ComingSoon({
  title = 'Coming soon',
  message = "We're building this feature right now. It will be available here once it's ready.",
  onGoHome,
}: ComingSoonProps) {
  return (
    <Box
      className="cs-page"
      sx={{
        alignItems: 'center',
        color: 'text.primary',
        display: 'flex',
        justifyContent: 'center',
        minHeight: '60vh',
        px: 3,
        py: 7,
        width: '100%',
      }}
    >
      <style>{STYLES}</style>
      <Box
        sx={{
          alignItems: 'center',
          display: 'flex',
          flexDirection: 'column',
          gap: 2.5,
          maxWidth: 560,
          textAlign: 'center',
          width: '100%',
        }}
      >
        <Box
          className="cs-art"
          aria-hidden="true"
          sx={{
            alignItems: 'center',
            display: 'flex',
            height: { xs: 180, sm: 220 },
            justifyContent: 'center',
            mb: 1,
            position: 'relative',
            width: { xs: 180, sm: 220 },
          }}
        >
          <div className="cs-glow" />
          <svg className="cs-hourglass" width="110" height="160" viewBox="0 0 100 150">
            <defs>
              <clipPath id="csTopGlass">
                <path d="M18 14 H82 C82 44 56 60 52 73 H48 C44 60 18 44 18 14Z" />
              </clipPath>
              <clipPath id="csBotGlass">
                <path d="M48 77 H52 C56 90 82 106 82 136 H18 C18 106 44 90 48 77Z" />
              </clipPath>
            </defs>
            <g clipPath="url(#csTopGlass)">
              <rect className="cs-sand cs-sand-top" x="0" y="34" width="100" height="40" />
            </g>
            <g clipPath="url(#csBotGlass)">
              <rect className="cs-sand cs-sand-bot" x="0" y="96" width="100" height="40" />
            </g>
            <rect className="cs-stream" x="49" y="72" width="2" height="60" rx="1" />
            <path
              className="cs-glass"
              d="M18 14 C18 44 44 60 48 75 C44 90 18 106 18 136 M82 14 C82 44 56 60 52 75 C56 90 82 106 82 136"
            />
            <rect className="cs-cap" x="8" y="4" width="84" height="10" rx="4" />
            <rect className="cs-cap" x="8" y="136" width="84" height="10" rx="4" />
          </svg>
        </Box>
        <Typography variant="h4" sx={{ mb: 0.5, fontWeight: 500 }}>
          {title}
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 4, opacity: 0.65 }}>
          {message}
        </Typography>
        <Box
          className="cs-bar"
          sx={{
            bgcolor: 'action.hover',
            borderRadius: '2px',
            height: 4,
            mt: 0.5,
            overflow: 'hidden',
            width: 220,
            '& span': {
              background: 'linear-gradient(90deg, transparent, #ff7a3d, transparent)',
              borderRadius: '2px',
              display: 'block',
              height: '100%',
              width: '40%',
            },
          }}
        >
          <span />
        </Box>
        {onGoHome && (
          <Button
            sx={{
              bgcolor: '#ff6a2b',
              borderRadius: '22px',
              color: '#fff',
              height: 44,
              mt: 1.5,
              px: 2.75,
              textTransform: 'none',
              '&:hover': { bgcolor: '#ff7d45' },
            }}
            onClick={onGoHome}
          >
            Go to home
          </Button>
        )}
      </Box>
    </Box>
  );
}
