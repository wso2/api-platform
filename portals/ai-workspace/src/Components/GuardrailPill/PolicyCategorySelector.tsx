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
  Checkbox,
  Chip,
  IconButton,
  Paper,
  Typography,
} from '@wso2/oxygen-ui';
import { POLICY_HUB_WEB_URL } from '../../config.env';
import { ChevronDown, ChevronUp, ExternalLink, X } from '@wso2/oxygen-ui-icons-react';

export const POLICY_CATEGORIES = [
  'AI',
  'Analytics & Monitoring',
  'Guardrails',
  'Logging',
  'Security',
  'Transformation',
];

export interface PolicyCategorySelectorProps {
  value: string[];
  onChange: (categories: string[]) => void;
}

export default function PolicyCategorySelector({
  value,
  onChange,
}: PolicyCategorySelectorProps) {
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    if (open) {
      document.addEventListener('mousedown', handleClickOutside);
    }
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [open]);

  const policyHubUrl = POLICY_HUB_WEB_URL || 'https://wso2.com/api-platform/policy-hub/';

  const handleToggle = (cat: string) => {
    onChange(value.includes(cat) ? value.filter((c) => c !== cat) : [...value, cat]);
  };

  return (
    <Box ref={containerRef} sx={{ position: 'relative' }}>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 0.5 }}>
        <Typography variant="caption" color="text.secondary">
          Categories
        </Typography>
        <Button
          size="small"
          variant='outlined'
          startIcon={<ExternalLink size={12} />}
          onClick={() => window.open(policyHubUrl, '_blank', 'noopener,noreferrer')}
          sx={{marginBottom:'8px'}}
        >
          Policy Hub
        </Button>
      </Box>
      <Box
        onClick={() => setOpen((prev) => !prev)}
        sx={{
          border: '1px solid',
          borderColor: open ? 'primary.main' : 'divider',
          borderRadius: 1,
          p: 0.75,
          cursor: 'pointer',
          display: 'flex',
          flexWrap: 'wrap',
          gap: 0.5,
          alignItems: 'center',
          minHeight: 40,
        }}
      >
        {value.length === 0 ? (
          <Typography variant="body2" color="text.secondary" sx={{ flex: 1 }}>
            Select categories to filter
          </Typography>
        ) : (
          value.map((cat) => (
            <Chip
              key={cat}
              label={cat}
              size="small"
              onDelete={(e) => { e.stopPropagation(); handleToggle(cat); }}
              deleteIcon={<X size={12} />}
            />
          ))
        )}
        <Box sx={{ ml: 'auto', display: 'flex', alignItems: 'center' }}>
          {value.length > 0 && (
            <IconButton
              size="small"
              onClick={(e) => { e.stopPropagation(); onChange([]); }}
            >
              <X size={14} />
            </IconButton>
          )}
          <IconButton size="small">
            {open ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
          </IconButton>
        </Box>
      </Box>
      {open && (
        <Paper
          sx={{
            position: 'absolute',
            top: '100%',
            left: 0,
            right: 0,
            zIndex: 1300,
            mt: 0.5,
            border: '1px solid',
            borderColor: 'divider',
            boxShadow: 3,
          }}
        >
          {POLICY_CATEGORIES.map((cat) => (
            <Box
              key={cat}
              onClick={() => handleToggle(cat)}
              sx={{
                display: 'flex',
                alignItems: 'center',
                px: 1.5,
                py: 0.5,
                cursor: 'pointer',
                '&:hover': { bgcolor: 'action.hover' },
              }}
            >
              <Checkbox
                checked={value.includes(cat)}
                size="small"
                onChange={() => handleToggle(cat)}
                onClick={(e) => e.stopPropagation()}
                sx={{ p: 0.5, mr: 1 }}
              />
              <Typography variant="body2">{cat}</Typography>
            </Box>
          ))}
        </Paper>
      )}
    </Box>
  );
}
