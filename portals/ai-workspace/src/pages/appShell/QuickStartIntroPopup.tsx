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

import React, { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import { Box, Button, Chip, IconButton, Typography } from '@wso2/oxygen-ui';
import { Rocket, X, Zap, Shield, Network } from '@wso2/oxygen-ui-icons-react';

export const QS_INTRO_STORAGE_KEY = 'qs_intro_seen_v1';

interface SpotRect {
  top: number;
  left: number;
  width: number;
  height: number;
}

interface QuickStartIntroPopupProps {
  anchorRef: React.RefObject<HTMLDivElement>;
  onDismiss: () => void;
}

export default function QuickStartIntroPopup({
  anchorRef,
  onDismiss,
}: QuickStartIntroPopupProps) {
  const [spotRect, setSpotRect] = useState<SpotRect | null>(null);

  useEffect(() => {
    const update = () => {
      if (anchorRef.current) {
        const r = anchorRef.current.getBoundingClientRect();
        setSpotRect({
          top: r.top,
          left: r.left,
          width: r.width,
          height: r.height,
        });
      }
    };
    update();
    window.addEventListener('resize', update);
    return () => window.removeEventListener('resize', update);
  }, [anchorRef]);

  const sidebarRight = spotRect ? spotRect.left + spotRect.width : 240;
  const popupLeft = spotRect ? sidebarRight + 20 : 260;
  const popupTop = spotRect ? spotRect.top + spotRect.height / 2 + 120 : 100;

  return createPortal(
    <>
      <div
        style={{
          position: 'fixed',
          top: 0,
          left: sidebarRight,
          right: 0,
          bottom: 0,
          zIndex: 1300,
          background: 'rgba(0,0,0,0.72)',
          cursor: 'pointer',
        }}
        onClick={onDismiss}
      />
      <Box
        sx={{
          position: 'fixed',
          top: popupTop,
          left: popupLeft,
          transform: 'translateY(-50%)',
          zIndex: 1400,
          width: 300,
          bgcolor: '#fff',
          borderRadius: '14px',
          overflow: 'hidden',
          boxShadow: '0 16px 48px rgba(0,0,0,0.3)',
        }}
      >
        <IconButton
          size="small"
          onClick={onDismiss}
          sx={{ position: 'absolute', top: 8, right: 8, zIndex: 1, color: '#6a6a6c' }}
        >
          <X size={14} />
        </IconButton>
        <Box
          sx={{
            position: 'absolute',
            left: -7,
            top: '50%',
            transform: 'translateY(-50%)',
            width: 0,
            height: 0,
            borderTop: '7px solid transparent',
            borderBottom: '7px solid transparent',
            borderRight: '7px solid #fff',
            filter: 'drop-shadow(-2px 0 2px rgba(0,0,0,0.08))',
          }}
        />
        <Box
          sx={{
            background: 'linear-gradient(135deg, #fff4ef 0%, #fde8dc 100%)',
            px: 2.5,
            pt: 2.5,
            pb: 2,
            display: 'flex',
            justifyContent: 'center',
            gap: 1.5,
          }}
        >
          <Box
            sx={{
              width: 48,
              height: 48,
              borderRadius: '12px',
              bgcolor: '#fff',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              boxShadow: '0 4px 12px rgba(248,118,68,0.2)',
            }}
          >
            <Rocket size={24} style={{ color: '#F87644' }} />
          </Box>
          <Box
            sx={{
              width: 44,
              height: 44,
              borderRadius: '10px',
              bgcolor: '#fff',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              boxShadow: '0 4px 12px rgba(0,0,0,0.07)',
            }}
          >
            <Zap size={20} style={{ color: '#F87644' }} />
          </Box>
          <Box
            sx={{
              width: 44,
              height: 44,
              borderRadius: '10px',
              bgcolor: '#fff',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              boxShadow: '0 4px 12px rgba(0,0,0,0.07)',
            }}
          >
            <Network size={20} style={{ color: '#64748b' }} />
          </Box>
          <Box
            sx={{
              width: 44,
              height: 44,
              borderRadius: '10px',
              bgcolor: '#fff',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              boxShadow: '0 4px 12px rgba(0,0,0,0.07)',
            }}
          >
            <Shield size={20} style={{ color: '#64748b' }} />
          </Box>
        </Box>
        <Box sx={{ px: 2.5, pt: 2, pb: 2.5 }}>
          <Typography
            variant="subtitle1"
            sx={{
              fontWeight: 700,
              mb: 1,
              lineHeight: 1.2,
              color: '#2b2b2b',
            }}
          >
            Get up and running with Quick Start
          </Typography>

          <Typography
            variant="body2"
            sx={{
              lineHeight: 1.5,
              mb: 2,
              fontSize: '0.8rem',
              fontWeight: 500,
              color: '#6a6a6c',
            }}
          >
            Step-by-step guided flows for AI Providers, MCP Proxies, and more —
            set up your AI Gateway in minutes.
          </Typography>

          <Box sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
            <Button variant="contained" size="small" onClick={onDismiss}>
              Got it
            </Button>
            <Button
              variant="text"
              size="small"
              component="a"
              href="https://wso2.com/bijira/docs/ai-workspace/getting-started/"
              target="_blank"
              rel="noopener noreferrer"
            >
              Learn more
            </Button>
          </Box>
        </Box>
      </Box>
    </>,
    document.body
  );
}
