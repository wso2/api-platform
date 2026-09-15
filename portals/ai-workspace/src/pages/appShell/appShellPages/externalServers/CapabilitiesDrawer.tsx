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

import React, { useEffect, useRef, useState } from 'react';
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Drawer,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Info, Upload, X } from '@wso2/oxygen-ui-icons-react';
import Editor from '@monaco-editor/react';
import { parse as parseYaml } from 'yaml';

type Props = {
  open: boolean;
  initialValue: string;
  onClose: () => void;
  onApply: (value: string) => void;
};

export default function CapabilitiesDrawer({
  open,
  initialValue,
  onClose,
  onApply,
}: Props): JSX.Element {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [drawerEditorValue, setDrawerEditorValue] = useState(initialValue);
  const [editorError, setEditorError] = useState('');
  const [isUploadModalOpen, setIsUploadModalOpen] = useState(false);
  const [uploadFileName, setUploadFileName] = useState('');
  const [uploadError, setUploadError] = useState('');

  useEffect(() => {
    if (open) {
      setDrawerEditorValue(initialValue);
      setEditorError('');
    }
  }, [open, initialValue]);

  const handleFileContent = (text: string, fileName: string) => {
    try {
      const ext = fileName.split('.').pop()?.toLowerCase();
      const rawParsed =
        ext === 'yaml' || ext === 'yml'
          ? (parseYaml(text) as Record<string, unknown>)
          : (JSON.parse(text) as Record<string, unknown>);

      const caps =
        rawParsed.capabilities &&
        typeof rawParsed.capabilities === 'object' &&
        !Array.isArray(rawParsed.capabilities)
          ? (rawParsed.capabilities as Record<string, unknown>)
          : rawParsed;

      const normalized = {
        tools: Array.isArray(caps.tools) ? caps.tools : [],
        resources: Array.isArray(caps.resources) ? caps.resources : [],
        prompts: Array.isArray(caps.prompts) ? caps.prompts : [],
      };

      setDrawerEditorValue(JSON.stringify(normalized, null, 2));
      setUploadFileName(fileName);
      setUploadError('');
      setIsUploadModalOpen(false);
    } catch {
      setUploadError(
        `Could not parse "${fileName}". Ensure it is valid JSON or YAML.`
      );
      setUploadFileName('');
    }
  };

  const handleFileChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = (e) =>
      handleFileContent(e.target?.result as string, file.name);
    reader.readAsText(file);
    event.target.value = '';
  };

  const handleFileDrop = (event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    const file = event.dataTransfer.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = (e) =>
      handleFileContent(e.target?.result as string, file.name);
    reader.readAsText(file);
  };

  const handleApply = () => {
    try {
      JSON.parse(drawerEditorValue);
    } catch {
      setEditorError('Editor contains invalid JSON. Fix it before applying.');
      return;
    }
    onApply(drawerEditorValue);
  };

  const handleOpenUploadModal = () => {
    setUploadFileName('');
    setUploadError('');
    setIsUploadModalOpen(true);
  };

  return (
    <>
      <Drawer
        anchor="right"
        open={open}
        onClose={onClose}
        sx={{
          '& .MuiDrawer-paper': {
            width: { xs: '100%', sm: 500 },
            maxWidth: '100%',
          },
        }}
      >
        <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
          {/* Header */}
          <Stack
            direction="row"
            alignItems="center"
            justifyContent="space-between"
            sx={{
              px: 2.5,
              py: 1.5,
              borderBottom: '1px solid',
              borderColor: 'divider',
            }}
          >
            <Typography variant="h6" sx={{ fontWeight: 600 }}>
              Add Capabilities
            </Typography>
            <IconButton size="small" onClick={onClose}>
              <X size={18} />
            </IconButton>
          </Stack>

          {/* Editor area */}
          <Box sx={{ flex: 1, overflow: 'auto', p: 2.5 }}>
            <Stack spacing={1.5} sx={{ height: '100%' }}>
              <Stack
                direction="row"
                alignItems="center"
                justifyContent="space-between"
              >
                <Typography variant="body2" color="text.secondary">
                  Enter capabilities as JSON. Optionally include{' '}
                  <code>tools</code>, <code>resources</code>, and{' '}
                  <code>prompts</code> arrays.{' '}
                  <Tooltip
                    title="Define the tools, resources, and prompts your MCP server exposes. Each entry should match the actual server response with a name, description, and input schema."
                    placement="right"
                    arrow
                  >
                    <Info
                      size={14}
                      color="#8D91A3"
                      style={{ cursor: 'pointer', verticalAlign: 'middle' }}
                    />
                  </Tooltip>
                </Typography>
                <Button
                  variant="outlined"
                  size="small"
                  startIcon={<Upload size={14} />}
                  onClick={handleOpenUploadModal}
                  sx={{ flexShrink: 0, ml: 1 }}
                >
                  Upload File
                </Button>
              </Stack>
              {uploadFileName ? (
                <Typography variant="caption" color="success.main">
                  ✓ Loaded: {uploadFileName}
                </Typography>
              ) : null}
              <Box
                sx={{
                  border: '1px solid',
                  borderColor: 'divider',
                  borderRadius: 1,
                  overflow: 'hidden',
                }}
              >
                <Editor
                  height="calc(100vh - 280px)"
                  language="json"
                  value={drawerEditorValue}
                  onChange={(value) => {
                    setDrawerEditorValue(value ?? '');
                    setEditorError('');
                  }}
                  options={{
                    minimap: { enabled: false },
                    scrollBeyondLastLine: false,
                    fontSize: 12,
                    lineHeight: 20,
                    wordWrap: 'on',
                    automaticLayout: true,
                  }}
                  theme="vs-dark"
                  loading={<Box sx={{ p: 2 }}>Loading editor...</Box>}
                />
              </Box>
              {editorError ? (
                <Typography variant="caption" color="error.main">
                  {editorError}
                </Typography>
              ) : null}
            </Stack>
          </Box>

          {/* Footer */}
          <Stack
            direction="row"
            spacing={1}
            justifyContent="flex-end"
            sx={{
              px: 2.5,
              py: 1.5,
              borderTop: '1px solid',
              borderColor: 'divider',
            }}
          >
            <Button variant="outlined" color="secondary" onClick={onClose}>
              Cancel
            </Button>
            <Button variant="contained" onClick={handleApply}>
              Apply
            </Button>
          </Stack>
        </Box>
      </Drawer>

      {/* Upload file modal */}
      <Dialog
        open={isUploadModalOpen}
        onClose={() => setIsUploadModalOpen(false)}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>
          <Stack
            direction="row"
            alignItems="center"
            justifyContent="space-between"
          >
            Upload File
            <IconButton
              size="small"
              onClick={() => setIsUploadModalOpen(false)}
            >
              <X size={18} />
            </IconButton>
          </Stack>
        </DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ pt: 0.5 }}>
            <Typography variant="body2" color="text.secondary">
              Upload a JSON or YAML file containing your MCP server capabilities
              (tools, resources, prompts).
            </Typography>
            <Box
              onDragOver={(e: React.DragEvent<HTMLDivElement>) =>
                e.preventDefault()
              }
              onDrop={handleFileDrop}
              onClick={() => fileInputRef.current?.click()}
              sx={{
                border: '2px dashed',
                borderColor: uploadError ? 'error.main' : 'primary.main',
                borderRadius: 2,
                p: 4,
                textAlign: 'center',
                cursor: 'pointer',
                bgcolor: uploadError ? 'error.50' : 'action.hover',
                transition: 'background-color 0.2s',
                '&:hover': { bgcolor: 'action.selected' },
              }}
            >
              <input
                ref={fileInputRef}
                type="file"
                accept=".json,.yaml,.yml"
                style={{ display: 'none' }}
                onChange={handleFileChange}
              />
              <Stack spacing={1} alignItems="center">
                <Upload size={32} />
                <Typography variant="body2" sx={{ fontWeight: 500 }}>
                  Drag &amp; drop a file here, or click to browse
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  Supported formats: .json, .yaml, .yml
                </Typography>
              </Stack>
            </Box>
            {uploadError ? (
              <Typography variant="caption" color="error.main">
                {uploadError}
              </Typography>
            ) : null}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button
            variant="outlined"
            color="secondary"
            onClick={() => setIsUploadModalOpen(false)}
          >
            Cancel
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
}
