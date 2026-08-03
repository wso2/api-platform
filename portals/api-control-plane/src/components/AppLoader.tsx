import { Box, Stack, Typography } from '@wso2/oxygen-ui';

import './AppLoader.css';

type AppLoaderProps = {
  /** Text shown under the loader. */
  label?: string;
  /** Render as a centered full-viewport overlay (auth/session/suspense). */
  fullScreen?: boolean;
  /** Scale factor for the icon (1 ≈ 100px). Defaults to 1, or 0.5 inline. */
  size?: number;
};

/**
 * Branded animated loader (adapted from ai-workspace's AILoader), themed with
 * the WSO2 orange palette. Use `fullScreen` for the session/suspense overlay;
 * the inline form (default) centers within its container.
 */
export function AppLoader({ label, fullScreen = false, size }: AppLoaderProps) {
  const scale = size ?? (fullScreen ? 1 : 0.5);
  return (
    <Box
      sx={
        fullScreen
          ? {
              alignItems: 'center',
              display: 'flex',
              justifyContent: 'center',
              minHeight: '100vh',
              px: 2,
              width: '100%',
            }
          : {
              alignItems: 'center',
              display: 'flex',
              justifyContent: 'center',
              p: 4,
              width: '100%',
            }
      }
    >
      <Stack alignItems="center" spacing={2.5}>
        <Box
          sx={{
            alignItems: 'center',
            display: 'flex',
            height: 100 * scale,
            justifyContent: 'center',
            width: 100 * scale,
          }}
        >
          <div className="axl-loader" style={{ ['--axl-size' as string]: scale }}>
            <svg width="100" height="100" viewBox="0 0 100 100">
              <defs>
                <mask id="axl-clipping">
                  <polygon points="0,0 100,0 100,100 0,100" fill="black" />
                  <polygon points="25,25 75,25 50,75" fill="white" />
                  <polygon points="50,25 75,75 25,75" fill="white" />
                  <polygon points="35,35 65,35 50,65" fill="white" />
                  <polygon points="35,35 65,35 50,65" fill="white" />
                  <polygon points="35,35 65,35 50,65" fill="white" />
                  <polygon points="35,35 65,35 50,65" fill="white" />
                </mask>
              </defs>
            </svg>
            <div className="axl-box" />
          </div>
        </Box>
        {label && (
          <Typography color="text.secondary" variant="body2">
            {label}
          </Typography>
        )}
      </Stack>
    </Box>
  );
}
