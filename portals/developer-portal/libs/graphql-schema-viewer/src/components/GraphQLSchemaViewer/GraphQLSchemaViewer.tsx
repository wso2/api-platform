/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC. and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein is strictly forbidden, unless permitted by WSO2 in accordance with
 * the WSO2 Commercial License available at http://wso2.com/licenses.
 * For specific language governing the permissions and limitations under
 * this license, please see the license as well as any agreement you’ve
 * entered into with WSO2 governing the purchase of this software and any
 * associated services.
 */

import React, { useState, useMemo } from 'react';
import { styles, getTypeColor, getTypeIcon } from './styles';

interface GraphQLSchemaViewerProps {
  schema: string; 
  apiMetadata?: {
    apiInfo?: {
      apiName?: string;
      apiVersion?: string;
      apiDescription?: string;
    };
    endPoints?: {
      productionURL?: string;
      sandboxURL?: string;
    };
  };
}

interface GraphQLOperation {
  name: string;
  type: 'query' | 'mutation';
  description?: string;
  parameters?: Array<{ name: string; type: string; required: boolean }>;
  returnType: string;
  content: string;
}

interface GraphQLType {
  name: string;
  kind: 'type' | 'interface' | 'enum' | 'scalar' | 'union' | 'input';
  description?: string;
  content: string;
}

export const GraphQLSchemaViewer: React.FC<GraphQLSchemaViewerProps> = ({ schema, apiMetadata }) => {
  const [expandedOperations, setExpandedOperations] = useState<Set<string>>(new Set());
  const [expandedSections, setExpandedSections] = useState<Set<string>>(new Set());
  const [copiedEndpoint, setCopiedEndpoint] = useState<string | null>(null);

  const { operations, types } = useMemo(() => {
    if (!schema || typeof schema !== 'string') return { operations: [], types: [] };

    const ops: GraphQLOperation[] = [];
    const typeDefs: GraphQLType[] = [];
    const lines = schema.split('\n');
    
    const parseOperationsFromType = (typeName: 'Query' | 'Mutation', operationType: 'query' | 'mutation'): GraphQLOperation[] => {
      const typeOps: GraphQLOperation[] = [];
      let inTypeBlock = false;
      let braceCount = 0;
      let currentDescription = '';
      let descriptionLines: string[] = [];
      let inDescription = false;

      for (let i = 0; i < lines.length; i++) {
        const line = lines[i];
        const trimmed = line.trim();

        if (!trimmed) {
          if (!inDescription) {
            currentDescription = '';
            descriptionLines = [];
          }
          continue;
        }

        if (trimmed === `type ${typeName}` || trimmed.startsWith(`type ${typeName} `) || trimmed.startsWith(`type ${typeName}{`)) {
          inTypeBlock = true;
          braceCount = 0;
          currentDescription = '';
          descriptionLines = [];
          inDescription = false;
          if (line.includes('{')) braceCount++;
          continue;
        }

        if (inTypeBlock) {
          if (line.includes('{')) braceCount++;
          if (line.includes('}')) braceCount--;

          const isDescriptionLine = (trimmed.startsWith('"""') || trimmed.startsWith('#'));
          
          if (isDescriptionLine && braceCount > 0) {
            inDescription = true;
            const descText = trimmed.replace(/^("""|#)/, '').replace(/"""$/, '').trim();
            if (descText) {
              descriptionLines.push(descText);
            }
            if (trimmed.endsWith('"""') && trimmed.startsWith('"""') && trimmed.length > 3) {
              inDescription = false;
            }
            continue;
          }

          if (inDescription && !isDescriptionLine && braceCount > 0) {
            currentDescription = descriptionLines.join(' ').trim();
            inDescription = false;
          }

          if (braceCount > 0 && !isDescriptionLine) {
            const fieldMatch = trimmed.match(/^\s*(\w+)\s*(\([^)]*\))?\s*:\s*(.+?)(\s*\{|\s*$)/);
            
            if (fieldMatch) {
              const operationName = fieldMatch[1];
              const paramsString = fieldMatch[2] || '';
              let returnType = fieldMatch[3].trim();
              returnType = returnType.replace(/\s*\{.*$/, '').trim();

              const parameters: Array<{ name: string; type: string; required: boolean }> = [];
              if (paramsString) {
                const paramsContent = paramsString.replace(/[()]/g, '');
                const paramMatches = Array.from(paramsContent.matchAll(/(\w+)\s*:\s*([^,]+)/g));
                for (const match of paramMatches) {
                  const paramName = match[1];
                  const paramType = match[2].trim();
                  const required = paramType.includes('!');
                  parameters.push({
                    name: paramName,
                    type: paramType.replace(/!/g, '').trim(),
                    required
                  });
                }
              }

              const description = currentDescription.trim() || undefined;
              
              typeOps.push({
                name: operationName,
                type: operationType,
                description: description,
                parameters: parameters.length > 0 ? parameters : undefined,
                returnType: returnType.replace(/[!,]/g, '').trim(),
                content: line.trim()
              });

              currentDescription = '';
              descriptionLines = [];
            } else {
              if (currentDescription && !trimmed.startsWith('#') && !trimmed.startsWith('"""')) {
                currentDescription = '';
                descriptionLines = [];
              }
            }
          }

          if (braceCount === 0 && trimmed.includes('}')) {
            inTypeBlock = false;
            break;
          }
        }
      }

      return typeOps;
    };

    const queryOps = parseOperationsFromType('Query', 'query');
    ops.push(...queryOps);

    const mutationOps = parseOperationsFromType('Mutation', 'mutation');
    ops.push(...mutationOps);

    let currentType: GraphQLType | null = null;
    let currentDescription = '';
    let braceCount = 0;
    let inType = false;
    let typeContent: string[] = [];

    for (let i = 0; i < lines.length; i++) {
      const line = lines[i];
      const trimmed = line.trim();

      if (!trimmed) continue;

      if ((trimmed.startsWith('"""') || trimmed.startsWith('#')) && !inType) {
        currentDescription += trimmed.replace(/^("""|#)/, '').replace(/"""$/, '').trim() + ' ';
        continue;
      }

      if (trimmed.startsWith('type ') || trimmed.startsWith('interface ') || trimmed.startsWith('enum ') || 
          trimmed.startsWith('scalar ') || trimmed.startsWith('union ') || trimmed.startsWith('input ')) {
        const match = trimmed.match(/^(type|interface|enum|scalar|union|input)\s+(\w+)/);
        if (match) {
          const typeName = match[2];
          if (typeName === 'Query' || typeName === 'Mutation' || typeName === 'Subscription') {
            continue;
          }

          if (currentType) {
            currentType.content = typeContent.join('\n');
            typeDefs.push(currentType);
          }

          const kind = match[1] as GraphQLType['kind'];
          currentType = {
            name: typeName,
            kind,
            description: currentDescription.trim() || undefined,
            content: ''
          };
          typeContent = [line];
          currentDescription = '';
          inType = true;
          braceCount = 0;

          if (line.includes('{')) braceCount++;
          if (line.includes('}')) braceCount--;
        }
      } else if (inType && currentType) {
        typeContent.push(line);
        if (line.includes('{')) braceCount++;
        if (line.includes('}')) braceCount--;

        if (braceCount === 0 && (line.includes('}') || currentType.kind === 'scalar' || currentType.kind === 'union')) {
          currentType.content = typeContent.join('\n');
          typeDefs.push(currentType);
          currentType = null;
          typeContent = [];
          inType = false;
        }
      } else {
        currentDescription = '';
      }
    }

    if (currentType) {
      currentType.content = typeContent.join('\n');
      typeDefs.push(currentType);
    }

    return { operations: ops, types: typeDefs };
  }, [schema]);

  const queries = operations.filter(op => op.type === 'query');
  const mutations = operations.filter(op => op.type === 'mutation');

  const toggleOperation = (operationName: string) => {
    const newExpanded = new Set(expandedOperations);
    if (newExpanded.has(operationName)) {
      newExpanded.delete(operationName);
    } else {
      newExpanded.add(operationName);
    }
    setExpandedOperations(newExpanded);
  };

  const toggleSection = (sectionName: string) => {
    const newExpanded = new Set(expandedSections);
    if (newExpanded.has(sectionName)) {
      newExpanded.delete(sectionName);
    } else {
      newExpanded.add(sectionName);
    }
    setExpandedSections(newExpanded);
  };

  const truncateUrl = (url: string, max: number = 95): string => {
    if (!url || url.length <= max) return url;
    const head = url.slice(0, Math.floor(max * 0.6));
    const tail = url.slice(-Math.floor(max * 0.3));
    return `${head}...${tail}`;
  };

  const handleEndpointCopy = async (endpointType: 'production' | 'sandbox', url?: string) => {
    if (!url) {
      return;
    }
    try {
      await navigator.clipboard.writeText(url);
      setCopiedEndpoint(endpointType);
      setTimeout(() => setCopiedEndpoint(null), 2000);
    } catch (err) {
      console.error('Failed to copy endpoint URL:', err);
    }
  };

  return (
    <div style={styles.container}>
      <div style={styles.contentWrapper}>
        {/* Header */}
        <div style={styles.header}>
          <h2 style={styles.title}>
            {apiMetadata?.apiInfo?.apiName || 'GraphQL Schema Documentation'}
            {apiMetadata?.apiInfo?.apiVersion && (
              <span style={styles.version}>
                {apiMetadata.apiInfo.apiVersion}
              </span>
            )}
          </h2>
          
          {/* Endpoints */}
          {apiMetadata && (apiMetadata.endPoints?.productionURL || apiMetadata.endPoints?.sandboxURL) && (
            <div style={styles.endpointsContainer}>
              {apiMetadata.endPoints.productionURL && (
                <div
                  style={styles.endpointCard}
                  role="button"
                  tabIndex={0}
                  aria-label="Copy production endpoint URL to clipboard"
                  onMouseEnter={(e) => {
                    e.currentTarget.style.borderColor = '#22863a';
                    e.currentTarget.style.boxShadow = '0 2px 8px rgba(34, 134, 58, 0.15)';
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.borderColor = '#e1e4e8';
                    e.currentTarget.style.boxShadow = 'none';
                  }}
                  onClick={() => handleEndpointCopy('production', apiMetadata.endPoints?.productionURL)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      handleEndpointCopy('production', apiMetadata.endPoints?.productionURL);
                    }
                  }}
                  title="Click to copy"
                >
                  <div style={{ ...styles.endpointIcon, ...styles.endpointIconProduction }}>
                    <svg width="14" height="14" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                      <path d="M10 2L2 7L10 12L18 7L10 2Z" fill="white"/>
                      <path d="M2 13L10 18L18 13" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                      <path d="M2 10L10 15L18 10" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                    </svg>
                  </div>
                  <div style={styles.endpointContent}>
                    <div style={{ ...styles.endpointLabel, ...styles.endpointLabelProduction }}>
                      Endpoint
                    </div>
                    <div style={styles.endpointUrl} title={apiMetadata.endPoints.productionURL}>
                      {truncateUrl(apiMetadata.endPoints.productionURL)}
                    </div>
                  </div>
                  <div style={{ position: 'relative', flexShrink: 0 }}>
                    {copiedEndpoint === 'production' && (
                      <div style={styles.copiedTooltip}>
                        Copied!
                        <div style={styles.tooltipArrow} />
                      </div>
                    )}
                    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg" style={styles.copyIcon}>
                      <path d="M4 2H12C13.1 2 14 2.9 14 4V12C14 13.1 13.1 14 12 14H4C2.9 14 2 13.1 2 12V4C2 2.9 2.9 2 4 2Z" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
                      <path d="M6 6H10M6 10H10" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>
                    </svg>
                  </div>
                </div>
              )}
              {apiMetadata.endPoints.sandboxURL && (
                <div
                  style={styles.endpointCard}
                  role="button"
                  tabIndex={0}
                  aria-label="Copy sandbox endpoint URL to clipboard"
                  onMouseEnter={(e) => {
                    e.currentTarget.style.borderColor = '#d73a49';
                    e.currentTarget.style.boxShadow = '0 2px 8px rgba(215, 58, 73, 0.15)';
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.borderColor = '#e1e4e8';
                    e.currentTarget.style.boxShadow = 'none';
                  }}
                  onClick={() => handleEndpointCopy('sandbox', apiMetadata.endPoints?.sandboxURL)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      handleEndpointCopy('sandbox', apiMetadata.endPoints?.sandboxURL);
                    }
                  }}
                  title="Click to copy"
                >
                  <div style={{ ...styles.endpointIcon, ...styles.endpointIconSandbox }}>
                    <svg width="14" height="14" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                      <path d="M10 2L2 7L10 12L18 7L10 2Z" fill="white"/>
                      <path d="M2 13L10 18L18 13" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                      <path d="M2 10L10 15L18 10" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                    </svg>
                  </div>
                  <div style={styles.endpointContent}>
                    <div style={{ ...styles.endpointLabel, ...styles.endpointLabelSandbox }}>
                      Sandbox
                    </div>
                    <div style={styles.endpointUrl} title={apiMetadata.endPoints.sandboxURL}>
                      {truncateUrl(apiMetadata.endPoints.sandboxURL)}
                    </div>
                  </div>
                  <div style={{ position: 'relative', flexShrink: 0 }}>
                    {copiedEndpoint === 'sandbox' && (
                      <div style={styles.copiedTooltip}>
                        Copied!
                        <div style={styles.tooltipArrow} />
                      </div>
                    )}
                    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg" style={styles.copyIcon}>
                      <path d="M4 2H12C13.1 2 14 2.9 14 4V12C14 13.1 13.1 14 12 14H4C2.9 14 2 13.1 2 12V4C2 2.9 2.9 2 4 2Z" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
                      <path d="M6 6H10M6 10H10" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>
                    </svg>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>

        {/* Content */}
        <div style={styles.content}>
          {/* Queries Section */}
          <div style={styles.section}>
            <div
              role="button"
              tabIndex={0}
              aria-expanded={expandedSections.has('queries')}
              aria-controls="graphql-queries-content"
              onClick={() => toggleSection('queries')}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  toggleSection('queries');
                }
              }}
              style={{
                ...styles.sectionHeader,
                borderBottom: expandedSections.has('queries') ? '1px solid #e1e4e8' : 'none'
              }}
            >
              <div style={styles.sectionHeaderContent}>
                <div style={{ ...styles.sectionIcon, backgroundColor: '#0066cc' }}>
                  Q
                </div>
                <div>
                  <h3 style={styles.sectionTitle}>
                    Queries
                  </h3>
                  <div style={styles.sectionSubtitle}>
                    {queries.length} operation{queries.length !== 1 ? 's' : ''}
                  </div>
                </div>
              </div>
              <span style={{
                ...styles.sectionToggle,
                transform: expandedSections.has('queries') ? 'rotate(90deg)' : 'rotate(0deg)'
              }}>
                ›
              </span>
            </div>

            {expandedSections.has('queries') && (
              <div
                id="graphql-queries-content"
                style={styles.sectionContent}
              >
                {queries.length === 0 ? (
                  <div style={styles.emptyState}>
                    No queries found
                  </div>
                ) : (
                  queries.map((operation) => (
                    <div
                      key={`query-${operation.name}`}
                      style={styles.operationCard}
                      onMouseEnter={(e) => e.currentTarget.style.boxShadow = '0 2px 4px rgba(0,0,0,0.1)'}
                      onMouseLeave={(e) => e.currentTarget.style.boxShadow = 'none'}
                    >
                      <div
                        onClick={() => toggleOperation(`query-${operation.name}`)}
                        style={styles.operationHeader}
                      >
                        <div style={{ flex: 1 }}>
                          <div style={styles.operationName}>
                            {operation.name}
                          </div>
                        </div>
                        <span style={{
                          ...styles.operationToggle,
                          transform: expandedOperations.has(`query-${operation.name}`) ? 'rotate(90deg)' : 'rotate(0deg)'
                        }}>
                          ›
                        </span>
                      </div>

                      {expandedOperations.has(`query-${operation.name}`) && (
                        <div style={styles.operationDetails}>
                          {operation.parameters && operation.parameters.length > 0 && (
                            <div style={{ marginBottom: '16px' }}>
                              <h4 style={styles.parametersTitle}>
                                Parameters
                              </h4>
                              <div style={styles.parametersList}>
                                {operation.parameters.map((param, idx) => (
                                  <div key={idx} style={styles.parameterItem}>
                                    <span style={styles.parameterName}>
                                      {param.name}
                                    </span>
                                    <span style={{ color: '#6a737d' }}>:</span>
                                    <span style={styles.parameterType}>
                                      {param.type}
                                    </span>
                                    {param.required && (
                                      <span style={styles.parameterRequired}>
                                        (required)
                                      </span>
                                    )}
                                  </div>
                                ))}
                              </div>
                            </div>
                          )}
                          
                          <div>
                            <h4 style={styles.schemaTitle}>
                              Schema Definition
                            </h4>
                            <pre style={styles.schemaCode}>
                              {operation.content}
                            </pre>
                          </div>
                        </div>
                      )}
                    </div>
                  ))
                )}
              </div>
            )}
          </div>

          {/* Mutations Section */}
          <div style={styles.section}>
            <div
              role="button"
              tabIndex={0}
              aria-expanded={expandedSections.has('mutations')}
              aria-controls="graphql-mutations-content"
              onClick={() => toggleSection('mutations')}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  toggleSection('mutations');
                }
              }}
              style={{
                ...styles.sectionHeader,
                borderBottom: expandedSections.has('mutations') ? '1px solid #e1e4e8' : 'none'
              }}
            >
              <div style={styles.sectionHeaderContent}>
                <div style={{ ...styles.sectionIcon, backgroundColor: '#d73a49' }}>
                  M
                </div>
                <div>
                  <h3 style={styles.sectionTitle}>
                    Mutations
                  </h3>
                  <div style={styles.sectionSubtitle}>
                    {mutations.length} operation{mutations.length !== 1 ? 's' : ''}
                  </div>
                </div>
              </div>
              <span style={{
                ...styles.sectionToggle,
                transform: expandedSections.has('mutations') ? 'rotate(90deg)' : 'rotate(0deg)'
              }}>
                ›
              </span>
            </div>

            {expandedSections.has('mutations') && (
              <div
                id="graphql-mutations-content"
                style={styles.sectionContent}
              >
                {mutations.length === 0 ? (
                  <div style={styles.emptyState}>
                    No mutations found
                  </div>
                ) : (
                  mutations.map((operation) => (
                    <div
                      key={`mutation-${operation.name}`}
                      style={styles.operationCard}
                      onMouseEnter={(e) => e.currentTarget.style.boxShadow = '0 2px 4px rgba(0,0,0,0.1)'}
                      onMouseLeave={(e) => e.currentTarget.style.boxShadow = 'none'}
                    >
                      <div
                        onClick={() => toggleOperation(`mutation-${operation.name}`)}
                        style={styles.operationHeader}
                      >
                        <div style={{ flex: 1 }}>
                          <div style={styles.operationName}>
                            {operation.name}
                          </div>
                        </div>
                        <span style={{
                          ...styles.operationToggle,
                          transform: expandedOperations.has(`mutation-${operation.name}`) ? 'rotate(90deg)' : 'rotate(0deg)'
                        }}>
                          ›
                        </span>
                      </div>

                      {expandedOperations.has(`mutation-${operation.name}`) && (
                        <div style={styles.operationDetails}>
                          {operation.parameters && operation.parameters.length > 0 && (
                            <div style={{ marginBottom: '16px' }}>
                              <h4 style={styles.parametersTitle}>
                                Parameters
                              </h4>
                              <div style={styles.parametersList}>
                                {operation.parameters.map((param, idx) => (
                                  <div key={idx} style={styles.parameterItem}>
                                    <span style={styles.parameterName}>
                                      {param.name}
                                    </span>
                                    <span style={{ color: '#6a737d' }}>:</span>
                                    <span style={styles.parameterType}>
                                      {param.type}
                                    </span>
                                    {param.required && (
                                      <span style={styles.parameterRequired}>
                                        (required)
                                      </span>
                                    )}
                                  </div>
                                ))}
                              </div>
                            </div>
                          )}
                          
                          <div>
                            <h4 style={styles.schemaTitle}>
                              Schema Definition
                            </h4>
                            <pre style={styles.schemaCode}>
                              {operation.content}
                            </pre>
                          </div>
                        </div>
                      )}
                    </div>
                  ))
                )}
              </div>
            )}
          </div>

          {/* Types Section */}
          <div style={styles.section}>
            <div
              role="button"
              tabIndex={0}
              aria-expanded={expandedSections.has('types')}
              aria-controls="graphql-types-content"
              onClick={() => toggleSection('types')}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  toggleSection('types');
                }
              }}
              style={{
                ...styles.sectionHeader,
                borderBottom: expandedSections.has('types') ? '1px solid #e1e4e8' : 'none'
              }}
            >
              <div style={styles.sectionHeaderContent}>
                <div style={{ ...styles.sectionIcon, backgroundColor: '#6a737d' }}>
                  T
                </div>
                <div>
                  <h3 style={styles.sectionTitle}>
                    Types
                  </h3>
                  <div style={styles.sectionSubtitle}>
                    {types.length} type{types.length !== 1 ? 's' : ''}
                  </div>
                </div>
              </div>
              <span style={{
                ...styles.sectionToggle,
                transform: expandedSections.has('types') ? 'rotate(90deg)' : 'rotate(0deg)'
              }}>
                ›
              </span>
            </div>

            {expandedSections.has('types') && (
              <div
                id="graphql-types-content"
                style={styles.sectionContent}
              >
                {types.length === 0 ? (
                  <div style={styles.emptyState}>
                    No types found
                  </div>
                ) : (
                  types.map((type) => (
                    <div
                      key={`type-${type.name}`}
                      style={styles.typeCard}
                      onMouseEnter={(e) => e.currentTarget.style.boxShadow = '0 2px 4px rgba(0,0,0,0.1)'}
                      onMouseLeave={(e) => e.currentTarget.style.boxShadow = 'none'}
                    >
                      <div
                        onClick={() => toggleOperation(`type-${type.name}`)}
                        style={styles.typeHeader}
                      >
                        <div style={{
                          ...styles.typeIcon,
                          backgroundColor: getTypeColor(type.kind)
                        }}>
                          {getTypeIcon(type.kind)}
                        </div>
                        <div style={styles.typeInfo}>
                          <div style={styles.typeName}>
                            {type.name}
                          </div>
                          {type.description && (
                            <div style={styles.typeDescription}>
                              {type.description}
                            </div>
                          )}
                          <span style={styles.typeBadge}>
                            {type.kind}
                          </span>
                        </div>
                        <span style={{
                          ...styles.operationToggle,
                          transform: expandedOperations.has(`type-${type.name}`) ? 'rotate(90deg)' : 'rotate(0deg)'
                        }}>
                          ›
                        </span>
                      </div>

                      {expandedOperations.has(`type-${type.name}`) && (
                        <div style={styles.operationDetails}>
                          <pre style={styles.schemaCode}>
                            {type.content}
                          </pre>
                        </div>
                      )}
                    </div>
                  ))
                )}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

export default GraphQLSchemaViewer;
