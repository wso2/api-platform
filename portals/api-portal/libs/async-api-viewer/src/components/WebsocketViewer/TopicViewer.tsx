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

import React, {
  Suspense,
  useState,
  useEffect,
  useRef,
  SetStateAction,
  useMemo,
} from 'react';
import * as ReactDOM from 'react-dom';
// flushSync is in React 18 runtime but not in @types/react-dom@17
const flushSync: (fn: () => void) => void = (ReactDOM as any).flushSync;
import LogLine from '../../components/InfiniteLogsPanel/LogLine';
import OutputConsole from './OutputConsole';
import { PayloadEditor } from './PayloadEditor';
import useStyles from './style';
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  TextInput,
  CopyToClipboard,
  TabCard,
  TabPanel,
  Button,
  Chip,
} from '../../designSystem';
import { Box, CircularProgress, Tooltip, Typography, Select, MenuItem, FormControl, Grid } from '@material-ui/core';
import { PlayArrow, Send } from '@material-ui/icons';
import { APITypeEnum } from './WebsocketViewer';

export const INTERNAL_KEY_HEADER_NAME = 'choreo-test-key';
export const DEVPORTAL_TOKEN_HEADER_NAME = 'choreo-oauth2-token';

interface TopicViewerProps {
  asyncType: APITypeEnum | undefined;
  token: string;
  apiEndpoint: string;
  sandboxEndpoint: string;
  topic: string;
  publish: boolean;
  subscribe: boolean;
  parameters?: object;
  isDevportal?: boolean;
  payload?: string;
}

export interface CollectedLogLineWithKey {
  timestamp: string;
  message?: string;
  randomKey: number;
  isNewLog: boolean;
}

const TopicViewer = (props: TopicViewerProps) => {
  const { token, apiEndpoint, sandboxEndpoint, topic, publish, subscribe, parameters, isDevportal, asyncType, payload } =
    props;
  const classes = useStyles();
  const [socket, setSocket] = useState<WebSocket | null>(null);
  const [messages, setMessages] = useState<Record<string, any>[]>([]);
  const [input, setInput] = useState(payload || '');
  const [endpoint, setEndpoint] = useState<string>(apiEndpoint);
  const [selectedEndpointType, setSelectedEndpointType] = useState<string>('production');
  const [pathParams, setPathParams] = useState<{ [key: string]: string }>({});
  const [connect, setConnect] = useState(false);
  const [connectButtonText, setConnectButtonText] = useState('Connect');
  const [selectedTab, setSelectedTab] = useState(0);
  const [apiToken, setApiToken] = useState(token);
  const [webSubHeader, setWebSubHeader] = useState('<your-header-here>');
  const [webSubHeaderValue, setWebSubHeaderValue] = useState('<your-header-value-here>');
  const [callbackURL, setCallbackURL] = useState('http://example.com/callback');
  const [leaseSeconds, setLeaseSeconds] = useState('864000');
  const [secret, setSecret] = useState('xxxxxxxxx');

  const handleTabChange = (event: any, newValue: SetStateAction<number>) => {
    setSelectedTab(newValue);
  };

  function generateCurlCommand() {
    const mode = selectedTab === 0 ? 'subscribe' : selectedTab === 1 ? 'unsubscribe' : 'subscribe';
    let curlCmd = `curl -X POST ${endpoint}`;
    curlCmd += ` -H 'Content-Type: application/x-www-form-urlencoded'`;
    curlCmd += ` --data-urlencode 'hub.topic=${topic}'`;
    curlCmd += ` --data-urlencode 'hub.callback=${callbackURL}'`;
    
    curlCmd += ` --data-urlencode 'hub.mode=${mode}'`;
    
    if (mode === 'subscribe') {
      if (leaseSeconds && leaseSeconds.trim()) {
        curlCmd += ` --data-urlencode 'hub.lease_seconds=${leaseSeconds}'`;
      }
    }
    
    if (webSubHeader && webSubHeader.trim() && webSubHeaderValue && webSubHeaderValue.trim()) {
      curlCmd += ` -H '${webSubHeader}: ${webSubHeaderValue}'`;
    }
    
    return curlCmd;
  }

  useEffect(() => {
    const baseEndpoint = selectedEndpointType === 'production' ? apiEndpoint : sandboxEndpoint;
    const normalizedBase = baseEndpoint.endsWith('/') ? baseEndpoint.slice(0, -1) : baseEndpoint;
    if (asyncType === APITypeEnum.WEBSUB) {
      setEndpoint(`${normalizedBase}/hub`);
    } else if (topic === '/*') {
      setEndpoint(`${normalizedBase}/`);
    } else if (topic === '/' || topic === '') {
      setEndpoint(normalizedBase);
    } else {
      const formattedTopic = topic.startsWith('/') ? topic : `/${topic}`;
      setEndpoint(`${normalizedBase}${formattedTopic}`);
    }
    if (parameters != null) {
      setPathParams(
        Object.keys(parameters).reduce((acc, key) => {
          acc[key] = '';
          return acc;
        }, {} as { [key: string]: string })
      );
    }
  }, [apiEndpoint, sandboxEndpoint, selectedEndpointType, topic]);

  const handleEndpointTypeChange = (event: any) => {
    const newValue = event.target.value as 'production' | 'sandbox';
    
    // Prevent switching to sandbox if sandboxEndpoint is not available
    if (newValue === 'sandbox' && (!sandboxEndpoint || sandboxEndpoint === '')) {
      setSelectedEndpointType('production');
      return;
    }
    
    setSelectedEndpointType(newValue);
  };

  const sendMessage = () => {
    if (asyncType === APITypeEnum.WEBSUB) {
      return;
    }
    if (socket && input && socket.readyState === socket.OPEN) {
      socket.send(input);
      setMessages((prev) => [
        ...prev,
        {
          message: `Sent: ${input}`,
          timestamp: new Date().toString(),
          randomKey: prev.length,
        },
      ]);
      setInput('');
    }
  };

  const connectWebsocket = () => {
    setConnectButtonText('Connecting...');

    let updatedEndpoint = endpoint;

    for (const key in pathParams) {
      updatedEndpoint = updatedEndpoint.replace(`{${key}}`, pathParams[key]);
    }
    const subprotocols: string[] = [];
    if (isDevportal) {
      subprotocols.push(DEVPORTAL_TOKEN_HEADER_NAME);
    } else {
      subprotocols.push(INTERNAL_KEY_HEADER_NAME);
    }
    if (apiToken && apiToken.trim()) {
      subprotocols.push(apiToken);
    }
    const ws = new WebSocket(updatedEndpoint, subprotocols);
    setSocket(ws);
    ws.onopen = () => {
      setConnect(true);
      setMessages((prev) => [
        ...prev,
        {
          message: `Connected to ${updatedEndpoint}`,
          timestamp: new Date().toString(),
          randomKey: prev.length,
        },
      ]);
      setConnectButtonText('Disconnect');
    };

    ws.onmessage = (event) => {
      flushSync(() => {
        setMessages((prev) => [
          ...prev,
          {
            message: `Received: ${event.data}`,
            timestamp: new Date().toString(),
            randomKey: prev.length,
          },
        ]);
      });
    };

    ws.onerror = () => {
      setConnect(false);
      setMessages((prev) => [
        ...prev,
        {
          message: `Error while connecting to the server. Please check the endpoint and try again.`,
          timestamp: new Date().toString(),
          randomKey: prev.length,
        },
      ]);
      setConnectButtonText('Connect');
    };
  };

  const disconnectWebsocket = () => {
    setConnectButtonText('Disconnecting...');
    setTimeout(() => {
      setConnectButtonText('Connect');
      setConnect(false);
      socket?.close();
      if (socket?.CLOSED) {
        setMessages((prev) => [
          ...prev,
          { message: `Disconnected`, timestamp: new Date().toString() },
        ]);
        setConnectButtonText('Connect');
      }
    }, 1000);
  };

  const clearLogs = () => {
    setMessages([]);
  };

  function stoInt(s: string) {
    return s.split('').reduce((a, b, i) => {
      const r = a + b.charCodeAt(0) * i;
      return r;
    }, 1);
  }

  const colorSet = [
    classes.logColor1,
    classes.logColor2,
    classes.logColor3,
    classes.logColor4,
  ];

  const { current: colorMap } = useRef(new Map<string, string>());
  const getColorCode = (name: string) => {
    if (colorMap.has(name)) {
      return colorMap.get(name) || '';
    }
    const colorCode = colorSet[stoInt(name) % colorSet.length];
    colorMap.set(name, colorCode);
    return colorCode;
  };

  const renderItem = (logEntry: CollectedLogLineWithKey) => (
    <LogLine
      getColor={getColorCode}
      logEntry={logEntry}
      timeZone="UTC"
      onCopy={() => {}}
      isExpanded={false}
      isHeiglighted={false}
      isExpandable={false}
    />
  );

  function getButtonText(connectBtnText: string) {
    switch (connectBtnText) {
      case 'Connecting...':
        return (
          "Connecting..."
        );
      case 'Connect':
        return (
          "Connect"
        );
      case 'Disconnect':
        return (
          "Disconnect"
        );
      case 'Disconnecting...':
        return (
          "Disconnecting..."
        );
      default:
        return (
          "Connect"
        );
    }
  }

  const tooltipText = useMemo(
    () =>
      connectButtonText === 'Connect' ? (
        "Connect to the server to start sending messages."
      ) : (
        "Enter your message to send."
      ),
    [connectButtonText]
  );

  const tabItems = [{ name: 'Parameters' }, { name: 'Payload' }, { name: 'Headers' }];
  const tabItemsWithoutParams = [{ name: 'Payload' }, { name: 'Headers' }];
  const tabItemsForWebSub = [{ name: 'Subscribe' }, { name: 'Unsubscribe' }];

  if (asyncType === APITypeEnum.WEBSUB) {
    return (
      <Box className={classes.topicContainer}>
        <Accordion square testId="topic-accordion" bordered>
          <AccordionSummary
            aria-controls="panel1a-content"
            testId="topic-accordion-summary"
          >
            <Box className={classes.topicTypeContainer}>
              <Box className={classes.typeChipContainer}>
                <Chip
                  label="SUB"
                  testId="default"
                  variant="contained"
                  color="success"
                  size="medium"
                  disabled={!subscribe}
                />
              </Box>
              <Box className={classes.typeChipContainer}>
                <Typography variant="body2">{topic}</Typography>
              </Box>
            </Box>
          </AccordionSummary>
          <AccordionDetails testId="topic-accordion-details">
            <Box width="100%" style={{ maxWidth: '100%', boxSizing: 'border-box', overflow: 'hidden' }}>
              {topic && (
                <Suspense
                  fallback={
                    <Box
                      display="flex"
                      alignItems="center"
                      justifyContent="center"
                    >
                      <CircularProgress size={25} />
                    </Box>
                  }
                >
                  <Box className={classes.endpointContainer}>
                    <FormControl 
                      size="small" 
                      variant="outlined" 
                      style={{ 
                        width: 150, 
                        flexShrink: 0,
                        height: 40,
                      }}
                    >
                      <Select
                        value={selectedEndpointType}
                        onChange={handleEndpointTypeChange}
                        disabled={connect}
                        data-testid="endpoint-type-select"
                        inputProps={{ 'aria-label': 'Endpoint type' }}
                        style={{
                          height: 40,
                          borderRadius: 4,
                          backgroundColor: '#ffffff',
                          boxShadow: '0 1px 3px rgba(0, 0, 0, 0.08)',
                        }}
                      >
                        <MenuItem value="production">Production</MenuItem>
                        <MenuItem value="sandbox" disabled={!sandboxEndpoint || sandboxEndpoint === ''}>Sandbox</MenuItem>
                      </Select>
                    </FormControl>
                    <Box className={classes.textInput}>
                      <CopyToClipboard
                        value={endpoint}
                        size="small"
                        testId="endpoint-url"
                      />
                    </Box>
                    {asyncType !== APITypeEnum.WEBSUB && (
                      <Button
                        color={connect ? "secondary" : "primary"}
                        variant="contained"
                        pill={false}
                        size="small"
                        disabled={asyncType === APITypeEnum.WEBSUB}
                        testId="disconnect"
                        style={{ 
                          flexShrink: 0, 
                          height: 40, 
                          minWidth: 120,
                          fontWeight: 500,
                          fontSize: 14,
                          textTransform: 'none',
                          borderRadius: 4,
                        }}
                        onClick={() =>
                          connect ? disconnectWebsocket() : connectWebsocket()
                        }
                      >
                        {getButtonText(connectButtonText)}
                      </Button>                      
                    )}
                  </Box>
                  <Box style={{ width: '100%', maxWidth: '100%', boxSizing: 'border-box' }}>
                    <Box className={classes.tabs}>
                      <Box className={classes.tabPanelContent}>
                        <Typography
                          data-testid="token-heading"
                          variant="body1"
                          className={classes.label}
                          display="block"
                        >
                          Token
                        </Typography>
                        <Box className={classes.parameterContainerWrapper} style={{ height: 'auto', overflow: 'visible' }}>
                          <Box
                            key={"api-token"}
                            className={classes.parameterContainer}
                            style={{ height: 'auto', overflow: 'visible' }}
                          >
                            <TextInput
                              testId={`input-headers`}
                              className={classes.callbackURLTextInput}
                              type="text"
                              placeholder="Header"
                              label="Header"
                              value={webSubHeader}
                              onChange={(e) => {
                                setWebSubHeader(e.target.value);
                              }}
                              error={webSubHeader === ""}
                              helperText="Enter Header"
                            />
                            <TextInput
                              testId={`input-header-value`}
                              className={classes.callbackURLTextInput}
                              type="text"
                              placeholder="Value"
                              label="Value"
                              value={webSubHeaderValue}
                              onChange={(e) => {
                                setWebSubHeaderValue(e.target.value);
                              }}
                              error={webSubHeaderValue === ""}
                              helperText="Enter Value"
                            />
                          </Box>
                        </Box>
                      </Box>
                    </Box>
                  </Box>
                  <Box style={{ width: '100%', maxWidth: '100%', boxSizing: 'border-box' }}>
                    <TabCard
                      className={classes.tabs}
                      tabItems={tabItemsForWebSub}
                      handleChange={handleTabChange}
                      value={selectedTab}
                      testId="env-tab-card"
                      fullHeight
                    >
                      {/* Tab 0: Subscribe Payload */}
                      <TabPanel
                        value={selectedTab}
                        index={0}
                      >
                        <Box className={classes.tabPanelContent}>
                          <Box className={classes.parameterContainerWrapper}>
                            <Box className={classes.parameterContainer}>
                              <TextInput
                                testId={`input-callbackurl`}
                                className={classes.callbackURLTextInput}
                                type="text"
                                placeholder="Callback URL"
                                label="Callback URL"
                                value={callbackURL}
                                onChange={(e) => {
                                  const newCallbackURL = e.target.value;
                                  setCallbackURL(newCallbackURL);
                                }}
                                helperText="Enter Callback URL"
                              />
                              <TextInput
                                testId={`input-leaseseconds`}
                                className={classes.callbackURLTextInput}
                                type="text"
                                placeholder="Lease Seconds"
                                label="Lease Seconds"
                                value={leaseSeconds}
                                onChange={(e) => {
                                  const newLeaseSeconds = e.target.value;
                                  setLeaseSeconds(newLeaseSeconds);
                                }}
                                helperText="Enter Lease Seconds"
                              />
                              
                            </Box>
                          </Box>
                        <Grid>
                          <Typography
                            data-testid="curl-command-txt"
                            variant="body1"
                            className={classes.label}
                            display="block"
                          >
                            Command
                          </Typography>
                          <Box className={classes.payloadContainer}>
                          <CopyToClipboard
                            value={generateCurlCommand()}
                            size="small"
                            testId="curl-command"
                            isMultiline={true}
                          />
                          </Box>
                        </Grid>
                        </Box>
                      </TabPanel>
                      {/* Tab 1: Unsubscribe Payload */}
                      <TabPanel
                        value={selectedTab}
                        index={1}
                      >
                        <Box className={classes.tabPanelContent}>
                          <Box className={classes.parameterContainerWrapper}>
                            <Box className={classes.parameterContainer}>
                              <TextInput
                                testId={`input-callbackurl`}
                                className={classes.callbackURLTextInput}
                                type="text"
                                placeholder="Callback URL"
                                label="Callback URL"
                                value={callbackURL}
                                onChange={(e) => {
                                  const newCallbackURL = e.target.value;
                                  setCallbackURL(newCallbackURL);
                                }}
                                helperText="Enter Callback URL"
                              />
                            </Box>
                          </Box>
                        <Grid>
                          <Typography
                            data-testid="curl-command-txt"
                            variant="body1"
                            className={classes.label}
                            display="block"
                          >
                            Command
                          </Typography>
                          <Box className={classes.payloadContainer}>
                          <CopyToClipboard
                            value={generateCurlCommand()}
                            size="small"
                            testId="curl-command"
                            isMultiline={true}
                          />
                          </Box>
                        </Grid>
                        </Box>
                      </TabPanel>
                    </TabCard>
                  </Box>
                </Suspense>
              )}
            </Box>
          </AccordionDetails>
        </Accordion>
      </Box>
    );
  }
  return (
    <Box className={classes.topicContainer}>
      <Accordion square testId="topic-accordion" bordered>
        <AccordionSummary
          aria-controls="panel1a-content"
          testId="topic-accordion-summary"
        >
          <Box className={classes.topicTypeContainer}>
            {asyncType === APITypeEnum.WS && (
              <Box className={classes.typeChipContainer}>
                <Chip
                  label="PUB"
                  testId="default"
                  variant="contained"
                  color="info"
                  size="medium"
                  disabled={!publish}
                />
              </Box>
            )}
            <Box className={classes.typeChipContainer}>
              <Chip
                label="SUB"
                testId="default"
                variant="contained"
                color="success"
                size="medium"
                disabled={!subscribe}
              />
            </Box>
            <Box className={classes.typeChipContainer}>
              <Typography variant="body2">{topic}</Typography>
            </Box>
          </Box>
        </AccordionSummary>
        <AccordionDetails testId="topic-accordion-details">
          <Box width="100%" style={{ maxWidth: '100%', boxSizing: 'border-box', overflow: 'hidden' }}>
            {topic && (
              <Suspense
                fallback={
                  <Box
                    display="flex"
                    alignItems="center"
                    justifyContent="center"
                  >
                    <CircularProgress size={25} />
                  </Box>
                }
              >
                {asyncType === APITypeEnum.WS && (
                  <Box className={classes.endpointContainer}>
                    <FormControl 
                      size="small" 
                      variant="outlined" 
                      style={{ 
                        width: 150, 
                        flexShrink: 0,
                        height: 40,
                      }}
                    >
                      <Select
                        value={selectedEndpointType}
                        onChange={handleEndpointTypeChange}
                        disabled={connect}
                        data-testid="endpoint-type-select"
                        inputProps={{ 'aria-label': 'Endpoint type' }}
                        style={{
                          height: 40,
                          borderRadius: 4,
                          backgroundColor: '#ffffff',
                          boxShadow: '0 1px 3px rgba(0, 0, 0, 0.08)',
                        }}
                      >
                        <MenuItem value="production">Production</MenuItem>
                        <MenuItem value="sandbox" disabled={!sandboxEndpoint || sandboxEndpoint === ''}>Sandbox</MenuItem>
                      </Select>
                    </FormControl>
                    <Box className={classes.textInput}>
                      <CopyToClipboard
                        value={endpoint}
                        size="small"
                        testId="endpoint-url"
                      />
                    </Box>
                    <Button
                      color={connect ? "secondary" : "primary"}
                      variant="contained"
                      pill={false}
                      size="small"
                      testId="disconnect"
                      style={{ 
                        flexShrink: 0, 
                        height: 40, 
                        minWidth: 120,
                        fontWeight: 500,
                        fontSize: 14,
                        textTransform: 'none',
                        borderRadius: 4,
                      }}
                      onClick={() =>
                        connect ? disconnectWebsocket() : connectWebsocket()
                      }
                    >
                      {getButtonText(connectButtonText)}
                    </Button>
                  </Box>
                )}
                <Box style={{ width: '100%', maxWidth: '100%', boxSizing: 'border-box' }}>
                  <TabCard
                    className={classes.tabs}
                    tabItems={
                      Object.keys(pathParams).length === 0
                        ? tabItemsWithoutParams
                        : tabItems
                    }
                    handleChange={handleTabChange}
                    value={selectedTab}
                    testId="env-tab-card"
                    fullHeight
                  >
                    {Object.keys(pathParams).length !== 0 && (
                      <TabPanel value={selectedTab} index={0}>
                        <Box className={classes.tabPanelContent}>
                          <Box className={classes.parameterContainerWrapper}>
                            <Box className={classes.parameterContainer}>
                              {Object.keys(pathParams).map((param) => (
                                <Box key={param}>
                                  <TextInput
                                    testId={`input-${param}`}
                                    className={classes.paramTextInput}
                                    type="text"
                                    placeholder={param}
                                    label={param}
                                    value={pathParams[param]}
                                    onChange={(e) => {
                                      const newParams = {
                                        ...pathParams,
                                        [param]: e.target.value,
                                      };
                                      setPathParams(newParams);
                                    }}
                                    error={pathParams[param] === ""}
                                    helperText="Enter parameter value"
                                  />
                                </Box>
                              ))}
                            </Box>
                            <Box className={classes.executeButton}>
                              <Button
                                testId="execute"
                                disabled={connect}
                                variant="outlined"
                                onClick={connectWebsocket}
                                startIcon={<PlayArrow />}
                                size="small"
                              >
                                <Typography variant="body1" align="center">
                                  Execute
                                </Typography>
                              </Button>
                            </Box>
                          </Box>
                        </Box>
                      </TabPanel>
                    )}
                    <TabPanel
                      value={selectedTab}
                      index={Object.keys(pathParams).length === 0 ? 0 : 1}
                    >
                      <Box className={classes.tabPanelContent}>
                        <Box className={classes.payloadContainer}>
                          <PayloadEditor
                            fileContent={input}
                            language="json"
                            height="170px"
                            width="100%"
                            setFileContent={setInput}
                          />
                        </Box>
                        <Box mt={5}>
                          <Tooltip title={tooltipText} placement="top-start">
                            <Box className={classes.sendIcon}>
                              <Button
                                testId="payload-send"
                                disabled={input.length === 0 || !connect}
                                variant="outlined"
                                startIcon={<Send />}
                                onClick={sendMessage}
                                size="small"
                              >
                                <Typography variant="body1" align="center">
                                  Send
                                </Typography>
                              </Button>
                            </Box>
                          </Tooltip>
                        </Box>
                      </Box>
                    </TabPanel>
                    <TabPanel
                      value={selectedTab}
                      index={Object.keys(pathParams).length === 0 ? 1 : 2}
                    >
                      <Box className={classes.tabPanelContent}>
                        <Box className={classes.parameterContainerWrapper}>
                          <Box
                            key={"api-token"}
                            className={classes.parameterContainer}
                          >
                            <TextInput
                              testId={`input-headers`}
                              className={classes.apiTokenTextInput}
                              type="text"
                              placeholder="API Token"
                              label="API Token"
                              value={apiToken}
                              onChange={(e) => {
                                const newToken = e.target.value;
                                setApiToken(newToken);
                              }}
                              error={apiToken === ""}
                              helperText="Enter API Token"
                            />
                          </Box>
                        </Box>
                      </Box>
                    </TabPanel>
                  </TabCard>
                  <OutputConsole
                    messages={messages}
                    clearLogs={clearLogs}
                    isDisabled={messages.length < 1}
                  />
                  <Box>
                    <Box className={classes.outputContainer}>
                      {messages.map((msg, index) =>
                        renderItem({
                          timestamp: messages[index]["timestamp"],
                          message: messages[index]["message"],
                          randomKey: index,
                          isNewLog: false,
                        })
                      )}
                    </Box>
                  </Box>
                </Box>
              </Suspense>
            )}
          </Box>
        </AccordionDetails>
      </Accordion>
    </Box>
  );
};

export default TopicViewer;
