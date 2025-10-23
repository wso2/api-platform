import {
  Box,
  TextField,
  Typography,
  InputAdornment,
  Theme,
  Dialog,
  DialogTitle,
  DialogContent,
} from '@mui/material';
import * as Generated from './generated/index';
import { Card, CardContent } from '@design-system/components';
import { SearchIcon } from '@design-system/Icons';
import { useMemo, useState } from 'react';

const GeneratedIcons = Object.entries(Generated);
function IconsPreview() {
  const [search, setSearch] = useState('');
  const [isOpen, setIsOpen] = useState(false);
  const [selectedIcon, setSelectedIcon] = useState('');
  const filteredIcons = useMemo(
    () =>
      GeneratedIcons.filter(([name]) =>
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
      <Box
        display="flex"
        flexDirection="row"
        justifyContent="space-between"
        alignItems="center"
      >
        <TextField
          placeholder="Search icons..."
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
        <Typography variant="body1">{`Total Icons = ${GeneratedIcons.length}`}</Typography>
      </Box>
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
        {filteredIcons.map(([name, Icon]) => (
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
                onClick={() => {
                  setSelectedIcon(name);
                  setIsOpen(true);
                }}
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
                  <Icon fontSize="large" color="primary" />
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
        <Dialog
          open={isOpen}
          onClose={() => setIsOpen(false)}
          fullWidth
          maxWidth="md"
        >
          <DialogTitle>
            <Box mt={2}>
              <Typography variant="h3">Import Code</Typography>
            </Box>
          </DialogTitle>
          <DialogContent>
            <Box height={100}>
              <code className="importCode">
                {`import ${selectedIcon} from 'Icons/generated/${selectedIcon}.tsx';`}
              </code>
            </Box>
          </DialogContent>
        </Dialog>
      </Box>
    </Box>
  );
}
export default IconsPreview;
