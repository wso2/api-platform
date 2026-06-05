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

import React, { useState } from 'react';
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Button,
  Card,
  Chip,
  Collapse,
  Drawer,
  IconButton,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {
  Check,
  ChevronDown,
  ChevronUp,
  Copy,
  Eye,
  X,
} from '@wso2/oxygen-ui-icons-react';
import Editor from '@monaco-editor/react';
import type { EndpointValidationResponse } from './externalServersValidationTypes';

type Props = {
  validationResult: EndpointValidationResponse;
  showHeader?: boolean;
  showInputSchema?: boolean;
  showSchemaInline?: boolean;
};

const truncateText = (value: string, maxLength = 35): string =>
  value.length > maxLength
    ? `${value.slice(0, maxLength - 3).trimEnd()}...`
    : value;

const itemRowSx = {
  minHeight: 52,
  px: 1.5,
  py: 0.5,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  gap: 1,
  borderRadius: 1,
  border: '1px solid',
  borderColor: '#E1C38E',
  bgcolor: '#F5EFE3',
  cursor: 'pointer',
};

const itemDetailsSx = {
  mt: 0.5,
  px: 1.5,
  py: 1.25,
  borderTopLeftRadius: 0,
  borderTopRightRadius: 0,
  borderBottomLeftRadius: 8,
  borderBottomRightRadius: 8,
  border: '1px solid',
  borderColor: 'divider',
  bgcolor: 'rgba(245, 239, 227, 0.10)',
};

const accordionDetailsSx = {
  maxHeight: 320,
  overflowY: 'auto',
};

const itemListSx = {
  pr: 0.5,
};

export default function ExternalServersValidationDetails({
  validationResult,
  showHeader = true,
  showInputSchema = false,
  showSchemaInline = false,
}: Props): JSX.Element {
  const [openToolName, setOpenToolName] = useState<string | null>(null);
  const [openResourceUri, setOpenResourceUri] = useState<string | null>(null);
  const [openPromptName, setOpenPromptName] = useState<string | null>(null);
  const [schemaDrawerToolName, setSchemaDrawerToolName] = useState<
    string | null
  >(null);
  const [copiedToolName, setCopiedToolName] = useState<string | null>(null);
  const tools = validationResult.tools ?? [];
  const resources = validationResult.resources ?? [];
  const prompts = validationResult.prompts ?? [];
  const hasTools = tools.length > 0;
  const hasResources = resources.length > 0;
  const hasPrompts = prompts.length > 0;
  const expandToolsByDefault = hasTools && !hasResources && !hasPrompts;

  const content = (
    <Stack spacing={2}>
      {showHeader ? (
        <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap">
          <Typography variant="h6" sx={{ fontWeight: 600 }}>
            {validationResult.serverInfo.name}
          </Typography>
          <Chip
            label={`V ${validationResult.serverInfo.version}`}
            size="small"
            variant="outlined"
          />
        </Stack>
      ) : null}

      {hasTools ? (
        <Accordion
          defaultExpanded={expandToolsByDefault}
          sx={{ borderRadius: 1, '&:before': { display: 'none' } }}
        >
          <AccordionSummary expandIcon={<ChevronDown size={18} />}>
            <Stack direction="row" spacing={1} alignItems="center">
              <Typography sx={{ fontWeight: 600 }}>Tools</Typography>
              <Chip
                label={`Total: ${tools.length}`}
                size="small"
                variant="outlined"
                color="primary"
              />
            </Stack>
          </AccordionSummary>
          <AccordionDetails sx={accordionDetailsSx}>
            <Stack spacing={1.5} sx={itemListSx}>
              {tools.map((tool) => {
                const isOpen = openToolName === tool.name;

                return (
                  <Box key={tool.name} sx={{ minHeight: 44, mb: 0.8 }}>
                    <Box
                      onClick={() => setOpenToolName(isOpen ? null : tool.name)}
                      sx={itemRowSx}
                    >
                      <Typography
                        variant="subtitle2"
                        sx={{ fontWeight: 600, color: '#40404B' }}
                      >
                        {truncateText(tool.name)}
                      </Typography>
                      <IconButton
                        size="small"
                        onClick={(event) => {
                          event.stopPropagation();
                          setOpenToolName(isOpen ? null : tool.name);
                        }}
                      >
                        {isOpen ? (
                          <ChevronUp size={18} color="#8D91A3" />
                        ) : (
                          <ChevronDown size={18} color="#8D91A3" />
                        )}
                      </IconButton>
                    </Box>
                    <Collapse in={isOpen} timeout="auto" unmountOnExit>
                      <Box sx={itemDetailsSx}>
                        <Typography variant="body2" color="text.secondary">
                          {tool.description || 'No description available.'}
                        </Typography>
                        {showSchemaInline && tool.inputSchema ? (
                          <Box sx={{ mt: 1.5 }}>
                            <Typography
                              variant="subtitle2"
                              sx={{ fontWeight: 600, mb: 1 }}
                            >
                              Input Schema
                            </Typography>
                            <Box sx={{ position: 'relative' }}>
                              <IconButton
                                size="small"
                                sx={{
                                  position: 'absolute',
                                  top: 8,
                                  right: 8,
                                  zIndex: 1,
                                }}
                                onClick={() => {
                                  navigator.clipboard.writeText(
                                    JSON.stringify(tool.inputSchema, null, 2)
                                  );
                                  setCopiedToolName(tool.name);
                                  setTimeout(
                                    () => setCopiedToolName(null),
                                    2000
                                  );
                                }}
                              >
                                {copiedToolName === tool.name ? (
                                  <Check size={14} />
                                ) : (
                                  <Copy size={14} />
                                )}
                              </IconButton>
                              <Editor
                                height="400px"
                                language="json"
                                value={JSON.stringify(
                                  tool.inputSchema,
                                  null,
                                  2
                                )}
                                options={{
                                  readOnly: true,
                                  minimap: { enabled: false },
                                  scrollBeyondLastLine: false,
                                  fontSize: 12,
                                  lineHeight: 20,
                                  wordWrap: 'on',
                                  automaticLayout: true,
                                }}
                                theme="vs-dark"
                                loading={
                                  <Box sx={{ p: 1 }}>Loading editor...</Box>
                                }
                              />
                            </Box>
                          </Box>
                        ) : showInputSchema ? (
                          <Button
                            variant="contained"
                            size="small"
                            startIcon={<Eye size={16} />}
                            sx={{ mt: 1, textTransform: 'none' }}
                            onClick={() => setSchemaDrawerToolName(tool.name)}
                          >
                            View Schema
                          </Button>
                        ) : null}
                      </Box>
                    </Collapse>
                  </Box>
                );
              })}
            </Stack>
          </AccordionDetails>
        </Accordion>
      ) : null}

      {hasResources ? (
        <Accordion sx={{ borderRadius: 1, '&:before': { display: 'none' } }}>
          <AccordionSummary expandIcon={<ChevronDown size={18} />}>
            <Stack direction="row" spacing={1} alignItems="center">
              <Typography sx={{ fontWeight: 600 }}>Resources</Typography>
              <Chip
                label={`Total: ${resources.length}`}
                size="small"
                variant="outlined"
                color="primary"
              />
            </Stack>
          </AccordionSummary>
          <AccordionDetails sx={accordionDetailsSx}>
            <Stack spacing={1.5} sx={itemListSx}>
              {resources.map((resource) => {
                const isOpen = openResourceUri === resource.uri;

                return (
                  <Box key={resource.uri} sx={{ minHeight: 44, mb: 0.8 }}>
                    <Box
                      onClick={() =>
                        setOpenResourceUri(isOpen ? null : resource.uri)
                      }
                      sx={itemRowSx}
                    >
                      <Typography
                        variant="subtitle2"
                        sx={{ fontWeight: 600, color: '#40404B' }}
                      >
                        {truncateText(resource.name || resource.uri)}
                      </Typography>
                      <IconButton
                        size="small"
                        onClick={(event) => {
                          event.stopPropagation();
                          setOpenResourceUri(isOpen ? null : resource.uri);
                        }}
                      >
                        {isOpen ? (
                          <ChevronUp size={18} color="#8D91A3" />
                        ) : (
                          <ChevronDown size={18} color="#8D91A3" />
                        )}
                      </IconButton>
                    </Box>
                    <Collapse in={isOpen} timeout="auto" unmountOnExit>
                      <Box sx={itemDetailsSx}>
                        <Typography variant="body2" color="text.secondary">
                          {truncateText(resource.uri, 75)}
                        </Typography>
                        {resource.title ? (
                          <Typography
                            variant="body2"
                            color="text.secondary"
                            sx={{ mt: 0.5 }}
                          >
                            {truncateText(resource.title, 75)}
                          </Typography>
                        ) : null}
                      </Box>
                    </Collapse>
                  </Box>
                );
              })}
            </Stack>
          </AccordionDetails>
        </Accordion>
      ) : null}

      {hasPrompts ? (
        <Accordion sx={{ borderRadius: 1, '&:before': { display: 'none' } }}>
          <AccordionSummary expandIcon={<ChevronDown size={18} />}>
            <Stack direction="row" spacing={1} alignItems="center">
              <Typography sx={{ fontWeight: 600 }}>Prompts</Typography>
              <Chip
                label={`Total: ${prompts.length}`}
                size="small"
                variant="outlined"
                color="primary"
              />
            </Stack>
          </AccordionSummary>
          <AccordionDetails sx={accordionDetailsSx}>
            <Stack spacing={1.5} sx={itemListSx}>
              {prompts.map((prompt) => {
                const isOpen = openPromptName === prompt.name;

                return (
                  <Box key={prompt.name} sx={{ minHeight: 44, mb: 0.8 }}>
                    <Box
                      onClick={() =>
                        setOpenPromptName(isOpen ? null : prompt.name)
                      }
                      sx={itemRowSx}
                    >
                      <Typography
                        variant="subtitle2"
                        sx={{ fontWeight: 600, color: '#40404B' }}
                      >
                        {truncateText(prompt.name)}
                      </Typography>
                      <IconButton
                        size="small"
                        onClick={(event) => {
                          event.stopPropagation();
                          setOpenPromptName(isOpen ? null : prompt.name);
                        }}
                      >
                        {isOpen ? (
                          <ChevronUp size={18} color="#8D91A3" />
                        ) : (
                          <ChevronDown size={18} color="#8D91A3" />
                        )}
                      </IconButton>
                    </Box>
                    <Collapse in={isOpen} timeout="auto" unmountOnExit>
                      <Box sx={itemDetailsSx}>
                        <Typography variant="body2" color="text.secondary">
                          {truncateText(
                            prompt.description || 'No description available.',
                            75
                          )}
                        </Typography>
                      </Box>
                    </Collapse>
                  </Box>
                );
              })}
            </Stack>
          </AccordionDetails>
        </Accordion>
      ) : null}
    </Stack>
  );

  const schemaDrawerTool = schemaDrawerToolName
    ? tools.find((t) => t.name === schemaDrawerToolName)
    : null;

  const schemaDrawer = showInputSchema ? (
    <Drawer
      anchor="right"
      open={!!schemaDrawerTool}
      onClose={() => setSchemaDrawerToolName(null)}
      sx={{
        '& .MuiDrawer-paper': {
          width: { xs: '100%', sm: 500 },
          maxWidth: '100%',
          height: '100vh',
        },
      }}
    >
      {schemaDrawerTool ? (
        <Box sx={{ p: 3, height: '100%', overflowY: 'auto' }}>
          <Stack
            direction="row"
            justifyContent="space-between"
            alignItems="center"
            sx={{ mb: 1 }}
          >
            <Typography variant="h6" sx={{ fontWeight: 600 }}>
              {schemaDrawerTool.name}
            </Typography>
            <IconButton
              size="small"
              onClick={() => setSchemaDrawerToolName(null)}
            >
              <X size={18} />
            </IconButton>
          </Stack>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            {schemaDrawerTool.description || 'No description available.'}
          </Typography>
          <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 1 }}>
            Input Schema
          </Typography>
          <Box sx={{ position: 'relative' }}>
            <IconButton
              size="small"
              sx={{ position: 'absolute', top: 8, right: 8, zIndex: 1 }}
              onClick={() => {
                navigator.clipboard.writeText(
                  JSON.stringify(schemaDrawerTool.inputSchema, null, 2)
                );
                setCopiedToolName(schemaDrawerTool.name);
                setTimeout(() => setCopiedToolName(null), 2000);
              }}
            >
              {copiedToolName === schemaDrawerTool.name ? (
                <Check size={14} color='white' />
              ) : (
                <Copy size={14} color='white' />
              )}
            </IconButton>
            <Editor
              height="80vh"
              language="json"
              value={JSON.stringify(schemaDrawerTool.inputSchema, null, 2)}
              options={{
                readOnly: true,
                minimap: { enabled: false },
                scrollBeyondLastLine: false,
                fontSize: 12,
                lineHeight: 20,
                wordWrap: 'on',
                automaticLayout: true,
              }}
              theme="vs-dark"
              loading={<Box sx={{ p: 1 }}>Loading editor...</Box>}
            />
          </Box>
        </Box>
      ) : null}
    </Drawer>
  ) : null;

  if (!showHeader) {
    return (
      <>
        {content}
        {schemaDrawer}
      </>
    );
  }

  return (
    <>
      <Card sx={{ p: { xs: 2.5, sm: 3 } }}>{content}</Card>
      {schemaDrawer}
    </>
  );
}
