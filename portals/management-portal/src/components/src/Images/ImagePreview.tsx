import {
  Box,
  TextField,
  Typography,
  InputAdornment,
  type Theme,
} from '@mui/material';
import * as Images from '.';
import { useMemo, useState } from 'react';
import SearchIcon from './generated/SearchIcon';
import { Card, CardContent } from '../components/Card';

const GeneratedImages = Object.entries(Images);

function ImagePreview() {
  const [search, setSearch] = useState('');
  const filteredIcons = useMemo(
    () =>
      GeneratedImages.filter(([name]) =>
        name.toLowerCase().includes(search.toLowerCase())
      ),
    [search]
  );

  return (
    <Box
      display="flex"
      flexDirection="column"
      gap={(theme: Theme) => theme.spacing(3)}
      flexGrow={1}
      sx={{ p: (theme: Theme) => theme.spacing(3) }}
    >
      <TextField
        placeholder="Search Images..."
        variant="outlined"
        fullWidth
        size="medium"
        onChange={(e) => setSearch(e.target.value)}
        slotProps={{
          input: {
            startAdornment: (
              <InputAdornment position="start">
                <SearchIcon fontSize="inherit" color="action" />
              </InputAdornment>
            ),
          },
        }}
        sx={{
          maxWidth: 400,
          '& .MuiOutlinedInput-root': {
            backgroundColor: (theme: Theme) => theme.palette.background.paper,
          },
        }}
      />{' '}
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: {
            xs: '1fr',
            sm: '1fr 1fr',
            md: '1fr 1fr 1fr',
            lg: '1fr 1fr 1fr 1fr',
          },
          gap: (theme: Theme) => theme.spacing(2),
        }}
      >
        {filteredIcons.map(([name, Image]) => (
          <Card
            key={name}
            testId={`icon-preview-${name}`}
            variant="outlined"
            style={{
              height: '100%',
            }}
          >
            <CardContent paddingSize="md">
              <Box
                display="flex"
                flexDirection="column"
                alignItems="center"
                gap={(theme: Theme) => theme.spacing(2)}
              >
                <Box
                  display="flex"
                  alignItems="center"
                  justifyContent="center"
                  sx={{
                    p: (theme: Theme) => theme.spacing(2),
                    borderRadius: (theme: Theme) => theme.spacing(1),
                    backgroundColor: (theme: Theme) =>
                      theme.palette.background.default,
                    width: 60,
                    height: 60,
                    transition: 'transform 0.2s ease-in-out',
                    '&:hover': {
                      transform: 'scale(1.1)',
                    },
                  }}
                >
                  <Image />
                </Box>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  textAlign="center"
                  sx={{
                    wordBreak: 'break-word',
                    fontFamily: (theme: Theme) => theme.typography.fontFamily,
                  }}
                >
                  {name}
                </Typography>
              </Box>
            </CardContent>
          </Card>
        ))}
      </Box>
    </Box>
  );
}
export default ImagePreview;
