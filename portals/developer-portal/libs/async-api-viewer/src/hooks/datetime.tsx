/*
 * Copyright (c) 2025, WSO2 Inc. (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 Inc. and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein is strictly forbidden, unless permitted by WSO2 in accordance with
 * the WSO2 Commercial License available at http://wso2.com/licenses.
 * For specific language governing the permissions and limitations under
 * this license, please see the license as well as any agreement you’ve
 * entered into with WSO2 governing the purchase of this software and any
 * associated services.
 */

import dayjs from 'dayjs';
import customParseFormat from 'dayjs/plugin/customParseFormat';
import timezone from 'dayjs/plugin/timezone';
import utc from 'dayjs/plugin/utc';

dayjs.extend(timezone);
dayjs.extend(utc);
dayjs.extend(customParseFormat);

export function formatWithTimeZone(
  date: Date,
  timeZone: string,
  formatString: string = 'HH:mm:ss'
) {
  return dayjs(date).tz(timeZone).format(formatString);
}

export const DEFAULT_ZONE =
  Intl.DateTimeFormat().resolvedOptions().timeZone?.toString() || 'UTC';

const parseDate = (date: string | Date) => {
  if (typeof date === 'string') {
    return date === '' ? new Date() : new Date(date);
  }
  return date;
};

export const formatLogTimeStamp = (
  date: Date,
  timeZone: string = DEFAULT_ZONE
) => {
  const parsedDate = parseDate(date);
  return `${formatWithTimeZone(
    parsedDate,
    timeZone,
    'YYYY-MM-DDTHH:mm:ss.SSSZ'
  )}`;
};

export default formatLogTimeStamp;
