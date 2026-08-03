/**
 * Formats an ISO timestamp as a short relative string (e.g. "3 hours ago"),
 * the oxygen-console equivalent of the legacy `TimeDuration` component. Returns
 * an empty string for missing/invalid input so callers can render conditionally.
 */
const DIVISIONS: { amount: number; unit: Intl.RelativeTimeFormatUnit }[] = [
  { amount: 60, unit: 'second' },
  { amount: 60, unit: 'minute' },
  { amount: 24, unit: 'hour' },
  { amount: 7, unit: 'day' },
  { amount: 4.34524, unit: 'week' },
  { amount: 12, unit: 'month' },
  { amount: Number.POSITIVE_INFINITY, unit: 'year' },
];

const formatter = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' });

export const relativeTime = (value?: string | number | Date): string => {
  if (value === undefined || value === null || value === '') return '';
  const date = value instanceof Date ? value : new Date(value);
  const time = date.getTime();
  if (Number.isNaN(time)) return '';

  let duration = (time - Date.now()) / 1000;
  for (const division of DIVISIONS) {
    if (Math.abs(duration) < division.amount) {
      return formatter.format(Math.round(duration), division.unit);
    }
    duration /= division.amount;
  }
  return '';
};
