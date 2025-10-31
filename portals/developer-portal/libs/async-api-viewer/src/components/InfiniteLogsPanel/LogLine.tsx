/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com). All Rights Reserved.
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

import React from 'react';
import clsx from 'clsx';
import { formatLogTimeStamp } from '../../hooks/datetime';
import {
  LogLevel,
  LOG_ERROR,
  LOG_INFO,
  LOG_WARN,
  LOG_LEVEL_INFO,
  LOG_FAILED,
  LOG_SUCCEEDED,
} from '../../types';
import useStyles from './style';
import { JsonViewer } from './../../designSystem/JsonViewer';
import { ChevronRight, CopyrightOutlined } from '@material-ui/icons';
import { CollectedLogLineWithKey } from '../WebsocketViewer/TopicViewer';


interface LogLineProps {
  getColor: (name: string) => string;
  logEntry: CollectedLogLineWithKey;
  timeZone: string;
  onCopy: (log: CollectedLogLineWithKey) => void;
  isExpanded?: boolean;
  isHeiglighted?: boolean;
  isExpandable?: boolean;
}

const messageText = "Message";

const preserveWhitespace = (text: string | undefined): string =>
  text?.replace(/\s/g, '\u00A0') ?? '';

function TableRow(props: { title: string; value: any }) {
  const { title, value } = props;
  const classes = useStyles();
  const getVal = () => {
    switch (typeof value) {
      case 'string':
        return preserveWhitespace(value);
      case 'number':
      case 'boolean':
      case 'bigint':
        return value.toString();
      case 'object':
        return value ? (
          <JsonViewer
            displayDataTypes={false}
            displayObjectSize={false}
            shouldCollapse={false}
            src={value}
            maxHeight="100%"
            name={false}
            enableClipboard={false}
          />
        ) : null;

      default:
        return '';
    }
  };
  const valueElement = getVal();
  if (
    !valueElement ||
    title === 'randomKey' ||
    title === 'isNewLog' ||
    title === 'traceAvailable' ||
    title === 'displayTimeStamp' ||
    title === 'apiName' ||
    !title
  ) {
    return null;
  }
  return (
    <div className={classes.expandedLogRow}>
      <span className={classes.expandedLogKey}>{title}</span>
      <span>{valueElement}</span>
    </div>
  );
}
export default function LogLine(props: LogLineProps) {
  const {
    logEntry,
    timeZone,
    onCopy,
    getColor,
    isHeiglighted,
    isExpanded,
    isExpandable,
  } = props;
  const classes = useStyles();

  const copyText = "Copy"
  const copySelectionText = "Copy Selection"

  const getGatewayCodeColorCode = (code: string) => {
    const intCode = parseInt(code, 10);
    if (intCode >= 200 && intCode < 300) {
      return classes.infoLogs;
    }
    if (intCode >= 300 && intCode < 400) {
      return classes.warnLogs;
    }
    if (intCode >= 400 && intCode < 600) {
      return classes.errorLogs;
    }
    return classes.logOther;
  };

  const getLogColorCode = (levelString: LogLevel | string) => {
    const level = levelString.toUpperCase();
    if (level === LOG_INFO) {
      return classes.infoLogs;
    }
    if (level === LOG_WARN) {
      return classes.warningLogs;
    }
    if (level === LOG_ERROR) {
      return classes.errorLogs;
    }
    if (level === LOG_FAILED) {
      return classes.errorLogs;
    }
    if (level === LOG_SUCCEEDED) {
      return classes.infoLogs;
    }
    return getGatewayCodeColorCode(level);
    return classes.logOther;
  };
  const handleOnCopy = () => {
    onCopy(logEntry);
  };

  const handleEventPropagation = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
  };

  // Note: Please avoid using MUI element in this component
  return (
    <div>
      <div
        key={logEntry.randomKey}
        className={clsx(classes.defaultLogLine, {
          [classes.highlightedLogLine]: logEntry.isNewLog && !isHeiglighted,
          [classes.selectedLogLine]: isHeiglighted,
          [classes.whiteBackground]: !(isHeiglighted || logEntry.isNewLog),
        })}
        data-testid="log-panel-entry"
      >
        {isExpandable ? (
          <div className={classes.arrow}>
            {isExpanded ? (
              <ChevronRight fontSize="inherit" rotate={90} />
            ) : (
              <ChevronRight fontSize="inherit" />
            )}
          </div>
        ) : (
          <div className={classes.arrow} />
        )}

        <div className={classes.logLineWrapper}>
          <span className={classes.infoSection}>
            <span
              className={clsx(
                getLogColorCode(LOG_LEVEL_INFO),
                classes.toolTipParent
              )}
            >
              {formatLogTimeStamp(new Date(logEntry.timestamp), timeZone)}
              &nbsp;
            </span>
            {logEntry?.message && (
              <span
                className={clsx(
                  logEntry?.message?.includes('Error')
                    ? classes.errorLogs
                    : getColor(logEntry?.message),
                  classes.toolTipParent
                )}
              >
                {logEntry?.message}
                &nbsp;
                <span className={classes.toolTipChild}>{messageText}</span>
              </span>
            )}
          </span>
          <span className={classes.logMsg}>
            <span
              className={clsx(
                classes.disableUserSelect,
                classes.logEntryActions
              )}
            >
              <span className={classes.actionContainer}>
                <button
                  onClick={(e) => {
                    handleEventPropagation(e);
                    handleOnCopy();
                  }}
                  onMouseDown={handleEventPropagation}
                  onMouseUp={handleEventPropagation}
                  onMouseEnter={handleEventPropagation}
                  type="button"
                  className={classes.actionButton}
                >
                  <CopyrightOutlined className={classes.actionIcon} />
                  <span className={classes.actionLabel}>
                    {isHeiglighted ? copySelectionText : copyText}
                  </span>
                </button>
              </span>
            </span>
          </span>
        </div>
      </div>
      {isExpanded && (
        <div className={classes.tableContainer}>
          {Object.entries(logEntry).map(([key, value]) => (
            <TableRow
              key={key}
              title={key}
              value={
                key === 'logLine'
                  ? preserveWhitespace(String(value ?? ''))
                  : value
              }
            />
          ))}
        </div>
      )}
    </div>
  );
}
