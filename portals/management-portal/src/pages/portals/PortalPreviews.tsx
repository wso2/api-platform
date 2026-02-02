import * as React from 'react';
import { Box, Divider, Paper, Stack, Typography } from '@mui/material';
import LockOutlinedIcon from '@mui/icons-material/LockOutlined';
import PublicOutlinedIcon from '@mui/icons-material/PublicOutlined';
import { Button } from '../../components/src/components/Button';

export const PrivatePreview: React.FC = () => (
  <Paper variant="outlined" sx={{ p: 3, minHeight: 560 }}>
    <Stack spacing={2}>
      <Stack direction="row" alignItems="center" spacing={1}>
        <LockOutlinedIcon fontSize="small" />
        <Typography variant="h6">Internal Marketplace</Typography>
      </Stack>
      <Typography variant="body2" color="text.secondary" sx={{ maxWidth: 520 }}>
        Here you will have some good context in the subheading for your
        developer portal so users can know more about your product
      </Typography>
      <Box sx={{ height: 160, borderRadius: 2, bgcolor: 'action.hover' }} />
      <Stack direction="row" spacing={2} justifyContent="center">
        {['logo1', 'logo2', 'logo3', 'logo4', 'logo5'].map((k) => (
          <Box
            key={k}
            sx={{
              width: 64,
              height: 16,
              bgcolor: 'action.hover',
              borderRadius: 1,
            }}
          />
        ))}
      </Stack>
      <Divider />
      <Typography variant="h6">Get started</Typography>
      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
        <Box sx={{ flex: 1 }}>
          <Typography variant="subtitle1">
            Some guide title over here
          </Typography>
          <Typography variant="body2" color="text.secondary">
            A brief description for your guides. API greenfield, cache,
            container abstraction…
          </Typography>
          <Button size="small" sx={{ mt: 1 }}>
            Read more
          </Button>
        </Box>
        <Box
          sx={{
            flex: 1,
            height: 120,
            borderRadius: 2,
            bgcolor: 'action.hover',
          }}
        />
      </Stack>
      <Stack spacing={2}>
        <Box sx={{ height: 140, borderRadius: 2, bgcolor: 'action.hover' }} />
        <Box>
          <Typography variant="subtitle1">
            Another title for your guide
          </Typography>
          <Typography variant="body2" color="text.secondary">
            A brief description for your guides. API greenfield, container
            abstraction, etc.
          </Typography>
          <Button size="small" sx={{ mt: 1 }}>
            Read more
          </Button>
        </Box>
      </Stack>
      <Divider />
      <Typography variant="h6">Explore APIs</Typography>
      <Stack direction="row" spacing={2}>
        {[1, 2, 3].map((i) => (
          <Box
            key={i}
            sx={{
              flex: 1,
              height: 120,
              borderRadius: 2,
              bgcolor: 'action.hover',
            }}
          />
        ))}
      </Stack>
    </Stack>
  </Paper>
);

export const PublicPreview: React.FC = () => (
  <Paper variant="outlined" sx={{ p: 3, minHeight: 560 }}>
    <Stack spacing={2}>
      <Stack direction="row" alignItems="center" spacing={1}>
        <PublicOutlinedIcon fontSize="small" />
        <Typography variant="h6">Dev Portal</Typography>
      </Stack>
      <Typography variant="body2" color="text.secondary" sx={{ maxWidth: 520 }}>
        Anyone with the link can view your portal and APIs. Great for open docs
        and public discovery.
      </Typography>
      <Box sx={{ height: 160, borderRadius: 2, bgcolor: 'action.hover' }} />
      <Divider />
      <Typography variant="h6">Explore APIs</Typography>
      <Stack direction="row" spacing={2}>
        {[1, 2, 3].map((i) => (
          <Box
            key={i}
            sx={{
              flex: 1,
              height: 120,
              borderRadius: 2,
              bgcolor: 'action.hover',
            }}
          />
        ))}
      </Stack>
    </Stack>
  </Paper>
);
