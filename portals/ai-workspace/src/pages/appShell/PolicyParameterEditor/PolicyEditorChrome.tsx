/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC. and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein is strictly forbidden, unless permitted by WSO2 in accordance with
 * the WSO2 Commercial License available at http://wso2.com/licenses.
 * For specific language governing the permissions and limitations under
 * this license, please see the license as well as any agreement you've
 * entered into with WSO2 governing the purchase of this software and any
 * associated services.
 */

import React, { useMemo, useState } from 'react';
import { Alert, Box, Button, Typography } from '@wso2/oxygen-ui';
import { ParameterSchema } from './types';
import { useStyles } from './styles';

// Max lines to show before truncating description
const MAX_DESCRIPTION_LINES = 5;

/**
 * Format description text:
 * - Collapse single newlines within paragraphs to spaces
 * - Preserve paragraph breaks (double newlines)
 * - Preserve bullet points (lines starting with - or *)
 */
function formatDescriptionText(text: string): string {
  if (!text) return '';

  const trimmed = text.trim();

  // Split by double newlines (paragraph breaks)
  const paragraphs = trimmed.split(/\n\n+/);

  return paragraphs
    .map((paragraph) => {
      // Check if paragraph contains bullet points
      const lines = paragraph.split('\n');
      const hasBullets = lines.some((line) => /^\s*[-*•]/.test(line));

      if (hasBullets) {
        // Keep bullet point formatting, but collapse non-bullet continuation lines
        const result: string[] = [];
        let currentLine = '';

        for (const line of lines) {
          if (/^\s*[-*•]/.test(line)) {
            // This is a bullet point line
            if (currentLine) {
              result.push(currentLine);
            }
            currentLine = line;
          } else if (line.trim() === '') {
            // Empty line
            if (currentLine) {
              result.push(currentLine);
              currentLine = '';
            }
          } else {
            // Continuation of previous line
            currentLine = currentLine ? `${currentLine} ${line.trim()}` : line;
          }
        }
        if (currentLine) {
          result.push(currentLine);
        }
        return result.join('\n');
      } else {
        // Regular paragraph - collapse all newlines to spaces
        return paragraph.replace(/\n/g, ' ').replace(/\s+/g, ' ').trim();
      }
    })
    .join('\n\n');
}

/**
 * Component to render header description with show more/less
 */
const TruncatedHeaderDescription: React.FC<{
  description: string;
  classes: ReturnType<typeof useStyles>;
}> = ({ description, classes }) => {
  const [isExpanded, setIsExpanded] = useState(false);
  const formatted = formatDescriptionText(description);
  const lineCount = formatted.split('\n').length;
  const needsTruncation = lineCount > MAX_DESCRIPTION_LINES;

  if (!needsTruncation) {
    return <Typography sx={{ fontSize: '0.8rem' }}>{formatted}</Typography>;
  }

  return (
    <>
      <Typography
        variant="body1"
        component="div"
        sx={
          isExpanded
            ? classes.headerDescription
            : classes.headerDescriptionTruncated
        }
      >
        {formatted}
      </Typography>
      <Box
        component="span"
        sx={classes.showMoreButton}
        onClick={() => setIsExpanded(!isExpanded)}
        style={{ cursor: 'pointer' }}
      >
        {isExpanded ? 'Show less' : 'Show more'}
      </Box>
    </>
  );
};

/**
 * The "at least one of ... must be configured" hint for anyOf constraints, or
 * null when the schema has none.
 */
export function getAnyOfMessage(parameters: ParameterSchema): string | null {
  // Case 1: Parent-level anyOf (e.g. anyOf: [{ required: [request] }, { required: [response] }])
  if (parameters.anyOf && parameters.anyOf.length > 0) {
    const requiredItems = parameters.anyOf
      .flatMap((entry) => entry.required || [])
      .filter((v, i, a) => a.indexOf(v) === i);
    if (requiredItems.length > 0) {
      return `At least one of ${requiredItems.join(
        ', '
      )} must be configured.`;
    }
  }

  // Case 2: Property-level anyOf (anyOf inside child properties like request/response)
  // Collect property names that have anyOf constraints
  if (parameters.properties) {
    const propsWithAnyOf = Object.keys(parameters.properties).filter(
      (key) => {
        const propSchema = parameters.properties![key];
        return (
          propSchema.type === 'object' &&
          propSchema.anyOf &&
          propSchema.anyOf.length > 0
        );
      }
    );
    if (propsWithAnyOf.length > 0) {
      return `At least one of ${propsWithAnyOf.join(
        ', '
      )} must be configured.`;
    }
  }

  return null;
}

/**
 * Title, description and anyOf hint shown above a policy parameters form.
 */
export const PolicyEditorHeader: React.FC<{
  title: string;
  description?: string;
  parameters: ParameterSchema;
}> = ({ title, description, parameters }) => {
  const classes = useStyles();
  const anyOfMessage = useMemo(() => getAnyOfMessage(parameters), [parameters]);
  return (
    <>
      {/* Header */}
      <Box sx={classes.header}>
        <Typography variant="h5">{title}</Typography>
        {description && (
          <TruncatedHeaderDescription
            description={description}
            classes={classes}
          />
        )}
      </Box>

      {/* AnyOf Info Alert */}
      {anyOfMessage && (
        <Alert severity="info" sx={{ mb: 1 }}>
          {anyOfMessage}
        </Alert>
      )}
    </>
  );
};

/**
 * Cancel/Close and Add/Update buttons shown below a policy parameters form.
 */
export const PolicyEditorActions: React.FC<{
  onCancel: () => void;
  onSubmit: () => void;
  isEditing: boolean;
  disabled?: boolean;
  readOnly?: boolean;
  submitDisabled?: boolean;
}> = ({ onCancel, onSubmit, isEditing, disabled = false, readOnly = false, submitDisabled = false }) => {
  const classes = useStyles();
  return (
    <Box sx={classes.buttonContainer}>
      <Button
        variant="outlined"
        color="secondary"
        onClick={onCancel}
        data-testid="policy-param-cancel"
        disabled={disabled}
      >
        {readOnly ? 'Close' : 'Cancel'}
      </Button>
      {!readOnly && (
        <Button
          variant="contained"
          color="primary"
          onClick={onSubmit}
          data-testid="policy-param-submit"
          disabled={disabled || submitDisabled}
        >
          {isEditing ? 'Update' : 'Add'}
        </Button>
      )}
    </Box>
  );
};
