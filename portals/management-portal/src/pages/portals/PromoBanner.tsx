import * as React from 'react';
import { Box, Grid, Stack, Typography } from '@mui/material';
import { Button } from '../../components/src/components/Button';

type Props = {
  onPrimary?: () => void;
  imageSrc: string;
  imageAlt?: string;
};

const PromoBanner: React.FC<Props> = ({
  onPrimary,
  imageSrc,
  imageAlt = 'AI theming illustration',
}) => {
  return (
    <Box
      sx={{
        p: { xs: 2, md: 3 },
        pr: { xs: 2, md: 6 },
        borderRadius: 3,
        border: '1px solid',
        borderColor: '#069668',
        backgroundImage:
          'linear-gradient(90deg, rgba(6,150,104,0.12) 0%, rgba(6,150,104,0.06) 50%, rgba(6,150,104,0.10) 100%)',
        position: 'relative',
        overflow: 'hidden',
      }}
    >
      <Grid container alignItems="center" spacing={2} wrap="nowrap">
        <Grid>
          <Stack spacing={1.5}>
            <Typography fontSize={18} fontWeight={800}>
              Theme your Devportal with AI
            </Typography>
            <Typography fontSize={14}>
              Generate a polished color palette, typography and layout presets
              for your portal. You can fine-tune everything afterwards.
            </Typography>
            <Box>
              <Button variant="contained" onClick={onPrimary}>
                Start theming
              </Button>
            </Box>
          </Stack>
        </Grid>

        <Grid>
          <Box
            component="img"
            src={imageSrc}
            alt={imageAlt}
            sx={{
              width: 240,
              height: 140,
              objectFit: 'contain',
              display: 'block',
              borderRadius: 2,
            }}
          />
        </Grid>
      </Grid>
    </Box>
  );
};

export default PromoBanner;
