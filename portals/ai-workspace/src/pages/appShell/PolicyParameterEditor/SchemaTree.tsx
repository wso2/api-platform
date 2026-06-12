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

import React, {
  useState,
  useCallback,
  useMemo,
  useRef,
  useEffect,
} from 'react';
import { Box, Typography, Collapse, IconButton, Button } from '@wso2/oxygen-ui';
import {
  ChevronRight,
  ChevronDown,
  Plus,
  Trash2,
} from '@wso2/oxygen-ui-icons-react';
import { SchemaTreeNode, ParameterSchema, ParameterValues } from './types';
import { schemaToTreeNodes, getValueByPath } from './schemaUtils';
import { getFieldRenderer } from './FieldRenderers';
import { useStyles } from './styles';
import { FormattedMessage } from 'react-intl';

const MAX_DESCRIPTION_LINES = 5;

/**
 * Component to render description with show more/less
 */
const TruncatedDescription: React.FC<{
  description: string;
  classes: ReturnType<typeof useStyles>;
}> = ({ description, classes }) => {
  const [isExpanded, setIsExpanded] = useState(false);
  const trimmed = description.trim();
  const lineCount = trimmed.split('\n').length;
  const needsTruncation = lineCount > MAX_DESCRIPTION_LINES;

  if (!needsTruncation) {
    return <Typography sx={classes.nodeDescription}>{trimmed}</Typography>;
  }

  return (
    <>
      <Typography
        sx={
          isExpanded
            ? classes.nodeDescription
            : classes.nodeDescriptionTruncated
        }
      >
        {trimmed}
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

interface ChildrenWithAdvancedProps {
  children: SchemaTreeNode[];
  values: ParameterValues;
  onChange: (path: string, value: unknown) => void;
  onAddArrayItem: (arrayPath: string, itemSchema: ParameterSchema) => void;
  onDeleteArrayItem: (arrayPath: string, index: number) => void;
  errors: Record<string, string>;
  disabled?: boolean;
}

/**
 * Splits children into normal and advanced groups based on x-wso2-policy-advanced-param
 */
const ChildrenWithAdvanced: React.FC<ChildrenWithAdvancedProps> = ({
  children,
  values,
  onChange,
  onAddArrayItem,
  onDeleteArrayItem,
  errors,
  disabled,
}) => {
  const classes = useStyles();
  const [showAdvanced, setShowAdvanced] = useState(false);

  const { normalChildren, advancedChildren } = useMemo(() => {
    const normal: SchemaTreeNode[] = [];
    const advanced: SchemaTreeNode[] = [];
    children.forEach((child) => {
      if (child.isAdvanced) {
        advanced.push(child);
      } else {
        normal.push(child);
      }
    });
    return { normalChildren: normal, advancedChildren: advanced };
  }, [children]);

  const hasAdvanced = advancedChildren.length > 0;

  return (
    <Box sx={classes.nodeChildren}>
      {normalChildren.map((child) => (
        <SchemaTreeNodeComponent
          key={child.id}
          node={child}
          values={values}
          onChange={onChange}
          onAddArrayItem={onAddArrayItem}
          onDeleteArrayItem={onDeleteArrayItem}
          errors={errors}
          disabled={disabled}
        />
      ))}
      {hasAdvanced && (
        <Box
          sx={[
            classes.advancedSettingsToggle,
            normalChildren.length === 0
              ? classes.advancedSettingsToggleNoBorder
              : null,
          ]}
        >
          <Button
            variant="text"
            size="small"
            onClick={() => setShowAdvanced(!showAdvanced)}
            startIcon={
              showAdvanced ? (
                <ChevronDown size={14} />
              ) : (
                <ChevronRight size={14} />
              )
            }
          >
            Advanced Settings
          </Button>
        </Box>
      )}
      {hasAdvanced && (
        <Collapse in={showAdvanced}>
          <Box sx={classes.optionalFieldsSection}>
            {advancedChildren.map((child) => (
              <SchemaTreeNodeComponent
                key={child.id}
                node={child}
                values={values}
                onChange={onChange}
                onAddArrayItem={onAddArrayItem}
                onDeleteArrayItem={onDeleteArrayItem}
                errors={errors}
                disabled={disabled}
              />
            ))}
          </Box>
        </Collapse>
      )}
    </Box>
  );
};

interface SchemaTreeNodeProps {
  node: SchemaTreeNode;
  values: ParameterValues;
  onChange: (path: string, value: unknown) => void;
  onAddArrayItem: (arrayPath: string, itemSchema: ParameterSchema) => void;
  onDeleteArrayItem: (arrayPath: string, index: number) => void;
  errors: Record<string, string>;
  disabled?: boolean;
}

/**
 * Renders a single tree node with its children
 */
const SchemaTreeNodeComponent: React.FC<SchemaTreeNodeProps> = ({
  node,
  values,
  onChange,
  onAddArrayItem,
  onDeleteArrayItem,
  errors,
  disabled,
}) => {
  const classes = useStyles();
  const [isExpanded, setIsExpanded] = useState(node.isExpanded ?? false);

  const value = getValueByPath(values, node.path);
  const error = errors[node.path];
  const hasChildren = node.children && node.children.length > 0;
  const isObjectOrArray =
    node.schema.type === 'object' || node.schema.type === 'array';
  const isComplexArray =
    node.schema.type === 'array' && node.schema.items?.type === 'object';

  // Determine if this node has expandable content
  const arrayValue = Array.isArray(value) ? value : [];
  const hasExpandableContent =
    (hasChildren && !isComplexArray) || // Objects with children
    (isComplexArray && arrayValue.length > 0); // Complex arrays with items

  const toggleExpand = useCallback(() => {
    setIsExpanded((prev) => !prev);
  }, []);

  // Wrap onAddArrayItem to also expand the array
  const handleAddArrayItem = useCallback(
    (arrayPath: string, itemSchema: ParameterSchema) => {
      onAddArrayItem(arrayPath, itemSchema);
      // Expand this array if we're adding to it
      if (arrayPath === node.path) {
        setIsExpanded(true);
      }
    },
    [onAddArrayItem, node.path]
  );

  const prevArrayLengthRef = useRef(arrayValue.length);

  const newItemIndex =
    arrayValue.length > prevArrayLengthRef.current ? arrayValue.length - 1 : -1;

  const FieldRenderer = getFieldRenderer(node);

  // Get array items for complex arrays
  const arrayItems = useMemo(() => {
    if (!isComplexArray) return [];
    return arrayValue.map((_, index) => {
      const itemPath = `${node.path}.${index}`;
      // New items should start expanded
      const isNewItem = index === newItemIndex;
      const itemNode: SchemaTreeNode = {
        id: itemPath,
        path: itemPath,
        name: `Item ${index + 1}`,
        schema: node.schema.items!,
        depth: node.depth + 1,
        isRequired: false,
        isExpanded: isNewItem,
        isArrayItem: true,
        arrayIndex: index,
        parentArrayPath: node.path,
        children:
          node.schema.items?.type === 'object' && node.schema.items?.properties
            ? schemaToTreeNodes(
                node.schema.items,
                itemPath,
                node.depth + 2,
                node.schema.items.required || []
              )
            : undefined,
      };
      return itemNode;
    });
  }, [isComplexArray, arrayValue, node, newItemIndex]);

  useEffect(() => {
    prevArrayLengthRef.current = arrayValue.length;
  });

  return (
    <Box sx={classes.treeNode}>
      {/* Node Row */}
      <Box
        sx={[
          classes.nodeRow,
          { pl: 0 },
          isExpanded && hasExpandableContent ? classes.nodeRowExpanded : null,
        ]}
      >
        {/* Expand/Collapse Icon - only show if there's expandable content */}
        {hasExpandableContent ? (
          <Box sx={classes.expandIcon} onClick={toggleExpand}>
            {isExpanded ? (
              <ChevronDown size={14} />
            ) : (
              <ChevronRight size={14} />
            )}
          </Box>
        ) : (
          <Box sx={classes.expandIconPlaceholder} />
        )}

        {/* Node Content */}
        <Box sx={classes.nodeContent}>
          {/* Label Row */}
          <Box sx={classes.nodeLabelRow}>
            <Typography sx={classes.nodeLabel}>{node.name}</Typography>
            {node.isRequired && (
              <Typography sx={classes.requiredBadge}>*</Typography>
            )}
            {!node.isRequired && node.depth === 0 && (
              <Typography
                variant="body2"
                sx={{ fontSize: '0.75rem', marginLeft: 0.5 }}
              >
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.PolicyParameterEditor.SchemaTree.optional"
                  defaultMessage={'(Optional)'}
                />
              </Typography>
            )}
            {/* Delete button for array items */}
            {node.isArrayItem && node.parentArrayPath !== undefined && (
              <IconButton
                size="small"
                sx={classes.arrayItemDeleteButton}
                onClick={() =>
                  onDeleteArrayItem(node.parentArrayPath!, node.arrayIndex!)
                }
                disabled={disabled}
                aria-label={`Delete ${node.name}`}
              >
                <Trash2 size={14} />
              </IconButton>
            )}
          </Box>

          {/* Description */}
          {node.schema.description && (
            <TruncatedDescription
              description={node.schema.description}
              classes={classes}
            />
          )}

          {/* Field Renderer (for leaf nodes) */}
          {FieldRenderer && (
            <FieldRenderer
              node={node}
              value={value}
              onChange={onChange}
              error={error}
              disabled={disabled}
            />
          )}

          {/* Add button for complex arrays */}
          {isComplexArray && (
            <Box sx={classes.arrayContainer}>
              <Button
                variant="outlined"
                size="small"
                startIcon={<Plus size={14} />}
                onClick={() =>
                  handleAddArrayItem(node.path, node.schema.items!)
                }
                disabled={disabled}
                data-testid={`add-${node.path}-item`}
                sx={classes.arrayAddButton}
              >
                Add Item
              </Button>
            </Box>
          )}
        </Box>
      </Box>

      {/* Children (for objects) - split into normal and advanced */}
      {hasChildren && !isComplexArray && (
        <Collapse in={isExpanded}>
          <ChildrenWithAdvanced
            children={node.children!}
            values={values}
            onChange={onChange}
            onAddArrayItem={handleAddArrayItem}
            onDeleteArrayItem={onDeleteArrayItem}
            errors={errors}
            disabled={disabled}
          />
        </Collapse>
      )}

      {/* Array Items (for complex arrays) */}
      {isComplexArray && (
        <Collapse in={isExpanded}>
          <Box sx={classes.nodeChildren}>
            {arrayItems.map((itemNode) => (
              <Box key={itemNode.id} sx={classes.arrayItem}>
                <SchemaTreeNodeComponent
                  node={itemNode}
                  values={values}
                  onChange={onChange}
                  onAddArrayItem={handleAddArrayItem}
                  onDeleteArrayItem={onDeleteArrayItem}
                  errors={errors}
                  disabled={disabled}
                />
              </Box>
            ))}
          </Box>
        </Collapse>
      )}
    </Box>
  );
};

interface SchemaTreeProps {
  schema: ParameterSchema;
  values: ParameterValues;
  onChange: (path: string, value: unknown) => void;
  onAddArrayItem: (arrayPath: string, itemSchema: ParameterSchema) => void;
  onDeleteArrayItem: (arrayPath: string, index: number) => void;
  errors: Record<string, string>;
  disabled?: boolean;
}

/**
 * Main Schema Tree component that renders the full parameter tree
 * Splits first-level parameters into required and optional (advanced) sections
 */
const SchemaTree: React.FC<SchemaTreeProps> = ({
  schema,
  values,
  onChange,
  onAddArrayItem,
  onDeleteArrayItem,
  errors,
  disabled,
}) => {
  const classes = useStyles();

  const treeNodes = useMemo(() => {
    return schemaToTreeNodes(schema, '', 0, schema.required || []);
  }, [schema]);

  return (
    <Box sx={classes.treeContainer}>
      <ChildrenWithAdvanced
        children={treeNodes}
        values={values}
        onChange={onChange}
        onAddArrayItem={onAddArrayItem}
        onDeleteArrayItem={onDeleteArrayItem}
        errors={errors}
        disabled={disabled}
      />
    </Box>
  );
};

export default SchemaTree;
