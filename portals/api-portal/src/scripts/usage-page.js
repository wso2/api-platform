/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/*
 * The Usage page. It reads GET /usage for the chosen subscription, period and grouping (per hour,
 * day or week), and shows:
 *   - the subscription's billable usage: a total, and a bar chart with a tooltip on each bar;
 *   - for the current billing period, the usage charges and the estimated total.
 *
 * The subscriber knows their API and plan, not the plan's prices or meters, so the usage of all
 * prices that count the same unit (requests, or tokens) is added up into one chart, titled with the
 * API and plan. The current billing period is drawn in full, with the part still to come shaded.
 *
 * The charts use Chart.js, which the page loads; without it, simple bars are drawn instead. Times
 * are in UTC. Everything from the response is written with textContent.
 */
(function usagePage() {
    const PERIOD_LABELS = {
        CURRENT: 'current billing period',
        PREVIOUS: 'previous billing period',
        LAST_30_DAYS: 'last 30 days',
        LAST_7_DAYS: 'last 7 days',
        LAST_24_HOURS: 'last 24 hours',
    };
    const BIN_LABELS = { HOUR: 'Hour', DAY: 'Day', WEEK: 'Week' };
    const BIN_NAMES = { HOUR: 'hour', DAY: 'day', WEEK: 'week' };
    const subscriptionSelect = document.getElementById('usageSubscription');
    const periodSelect = document.getElementById('usagePeriod');
    if (!subscriptionSelect || !periodSelect) return;
    let requestedId = new URLSearchParams(window.location.search).get('subscriptionId');
    // The bin the subscriber chose; null shows the period's default.
    let interval = null;
    let charts = [];
    // Only the latest read is shown: a slower, older answer is dropped.
    let latest = 0;

    /**
     * The first read shows a spinner in place of the page; a later one dims what is shown under a
     * spinner until the new usage replaces it.
     */
    function loading(on) {
        const content = document.getElementById('usageContent');
        const shown = content && !content.hidden;
        show('usageLoading', on && !shown);
        if (content) content.classList.toggle('is-loading', on && shown);
        if (content) content.setAttribute('aria-busy', String(on));
    }

    function el(tag, className, text) {
        const node = document.createElement(tag);
        if (className) node.className = className;
        if (text !== undefined && text !== null) node.textContent = text;
        return node;
    }

    function show(id, visible) {
        const node = document.getElementById(id);
        if (node) node.hidden = !visible;
    }

    function formatNumber(value) {
        return Number(value || 0).toLocaleString('en-US');
    }

    function formatHour(iso) {
        return new Date(iso).toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: false, timeZone: 'UTC' });
    }

    /** A bucket's label: "Oct 8" for a day, "Oct 8, 14:00" for an hour. */
    function bucketLabel(iso, bin) {
        return bin === 'HOUR' ? formatDay(iso) + ', ' + formatHour(iso) : formatDay(iso);
    }

    function weekday(iso) {
        return new Date(iso).toLocaleDateString('en-US', { weekday: 'short', timeZone: 'UTC' });
    }

    /** A bucket's label on the axis: "14:00", or the date at midnight; "Oct 8" for a day or week. */
    function tickLabel(iso, bin) {
        if (bin !== 'HOUR') return formatDay(iso);
        return new Date(iso).getUTCHours() === 0 ? formatDay(iso) : formatHour(iso);
    }

    /** A bucket's whole range, for its tooltip. */
    function bucketRange(iso, bin) {
        const start = new Date(iso).getTime();
        if (bin === 'HOUR') {
            return weekday(iso) + ', ' + formatDay(iso) + ' · ' + formatHour(iso) + '–'
                + formatHour(new Date(start + 3600000).toISOString()) + ' UTC';
        }
        if (bin === 'WEEK') return 'Week of ' + formatDay(iso) + ' – ' + formatDay(new Date(start + 6 * 86400000).toISOString(), true);
        return weekday(iso) + ', ' + formatDay(iso, true);
    }

    function compactNumber(value) {
        return new Intl.NumberFormat('en-US', { notation: 'compact', maximumFractionDigits: 1 }).format(value);
    }

    /**
     * A theme colour as the browser resolves it ("rgb(26, 76, 109)"), so the chart's canvas can use
     * it however the theme writes it (a hex value, color-mix(), another variable).
     */
    function themeColor(name, fallback) {
        const probe = document.createElement('span');
        probe.style.color = 'var(' + name + ', ' + fallback + ')';
        document.body.append(probe);
        const color = getComputedStyle(probe).color;
        probe.remove();
        return color || fallback;
    }

    /** An rgb() or hex colour with transparency; any other colour is returned as it is. */
    function withAlpha(color, alpha) {
        const rgb = /^rgba?\((\d+),\s*(\d+),\s*(\d+)/.exec(color);
        if (rgb) return 'rgba(' + rgb[1] + ', ' + rgb[2] + ', ' + rgb[3] + ', ' + alpha + ')';
        const hex = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i.exec(color);
        if (!hex) return color;
        const h = hex[1].length === 3 ? hex[1].split('').map((c) => c + c).join('') : hex[1];
        const n = parseInt(h, 16);
        return 'rgba(' + ((n >> 16) & 255) + ', ' + ((n >> 8) & 255) + ', ' + (n & 255) + ', ' + alpha + ')';
    }

    function formatDay(iso, withYear) {
        return new Date(iso + (iso.length === 10 ? 'T00:00:00Z' : '')).toLocaleDateString('en-US',
            { month: 'short', day: 'numeric', year: withYear ? 'numeric' : undefined, timeZone: 'UTC' });
    }

    function card(label, value, note) {
        const node = el('div', 'usage-card');
        node.append(el('span', 'usage-card-label', label), el('span', 'usage-card-value', value));
        if (note) node.append(el('span', 'usage-card-note', note));
        return node;
    }

    function usageLines(data) {
        return data.estimate ? data.estimate.lines.filter((l) => l.usage) : [];
    }

    /** "1,234 requests": a count in the unit a price counts. */
    function unitCount(n, unit) {
        return formatNumber(n) + ' ' + unit + (n === 1 ? '' : 's');
    }

    /**
     * The usage prices added up per unit ("request", "token"), bucket by bucket, with the bin they are
     * counted in and the bins the period offers. A bucket still to come stays empty (null).
     */
    function seriesOf(data) {
        if (!data.usage) return [];
        const byUnit = new Map();
        for (const p of data.usage.prices || []) {
            if (!p.buckets) continue;
            const sum = byUnit.get(p.unit);
            if (!sum) {
                byUnit.set(p.unit, { unit: p.unit, total: p.total, interval: data.usage.interval, intervals: data.usage.intervals,
                    buckets: p.buckets.map((b) => ({ start: b.start, value: b.value })) });
                continue;
            }
            sum.total = sum.total === null || p.total === null ? null : sum.total + p.total;
            sum.buckets.forEach((b, i) => {
                const value = p.buckets[i] ? p.buckets[i].value : null;
                if (value !== null) b.value = (b.value || 0) + value;
            });
        }
        return [...byUnit.values()];
    }

    /** "Billable requests": what a unit's series counts. */
    function billable(unit) {
        return 'Billable ' + unit + 's';
    }

    function renderCards(data) {
        const cards = document.getElementById('usageCards');
        const hourly = data.usage && data.usage.interval === 'HOUR';
        const range = data.from && data.to
            ? (hourly ? bucketLabel(data.from, 'HOUR') + ' – ' + bucketLabel(data.to, 'HOUR') + ' UTC'
                : formatDay(data.from) + ' – ' + formatDay(data.to, true))
            : '';
        const nodes = seriesOf(data).map((s) => card(billable(s.unit) + ', ' + PERIOD_LABELS[data.period],
            s.total === null ? '–' : unitCount(s.total, s.unit), range));
        if (data.estimate) {
            // The lines of the plan's usage prices; any other line counts only in the estimated total.
            const lines = usageLines(data);
            if (lines.length > 0) {
                const usageTotal = lines.reduce((sum, l) => sum + Number(l.amount), 0);
                const usageText = new Intl.NumberFormat('en-US', { style: 'currency', currency: data.estimate.currency })
                    .format(usageTotal);
                nodes.push(card('Usage charges', usageText));
            }
            nodes.push(card('Estimated total', data.estimate.totalText));
        }
        cards.replaceChildren(...nodes);
    }

    // Shades the part of the billing period still to come, from its first bucket to the end.
    const restOfPeriod = {
        id: 'usageRestOfPeriod',
        beforeDatasetsDraw(c, args, opts) {
            if (opts.first === null || opts.first === undefined) return;
            const x = c.scales.x;
            const area = c.chartArea;
            const left = opts.first === 0 ? area.left : (x.getPixelForValue(opts.first - 1) + x.getPixelForValue(opts.first)) / 2;
            const ctx = c.ctx;
            ctx.save();
            // A faint tint of the theme's muted colour, whatever form the colour is given in.
            ctx.globalAlpha = 0.06;
            ctx.fillStyle = opts.color;
            ctx.fillRect(left, area.top, area.right - left, area.bottom - area.top);
            ctx.globalAlpha = 1;
            ctx.restore();
        },
    };

    function chartConfig(usage, unitText) {
        const bin = usage.interval;
        const buckets = usage.buckets;
        const primary = themeColor('--primary', '#1d4ed8');
        const border = themeColor('--border', '#e5e7eb');
        const muted = themeColor('--text-muted', '#6b7280');
        const firstAhead = buckets.findIndex((b) => b.value === null);
        return {
            type: 'bar',
            data: {
                labels: buckets.map((b) => b.start),
                datasets: [{
                    data: buckets.map((b) => b.value),
                    backgroundColor: withAlpha(primary, 0.78),
                    hoverBackgroundColor: primary,
                    borderRadius: 3,
                    maxBarThickness: 40,
                    categoryPercentage: 0.86,
                    barPercentage: 0.92,
                }],
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                animation: { duration: 300 },
                // Hovering anywhere over a bar's column shows it.
                interaction: { mode: 'index', intersect: false },
                plugins: {
                    legend: { display: false },
                    usageRestOfPeriod: { first: firstAhead === -1 ? null : firstAhead, color: muted },
                    tooltip: {
                        displayColors: false,
                        padding: 10,
                        filter: (item) => item.raw !== null,
                        callbacks: {
                            title: (items) => bucketRange(buckets[items[0].dataIndex].start, bin),
                            label: (item) => unitText(item.raw),
                        },
                    },
                },
                scales: {
                    x: {
                        grid: { display: false },
                        border: { color: border },
                        ticks: {
                            color: muted, maxRotation: 0, autoSkip: true, autoSkipPadding: 18,
                            callback: (value, index) => tickLabel(buckets[index].start, bin),
                        },
                    },
                    y: {
                        beginAtZero: true,
                        grace: '8%',
                        border: { display: false },
                        grid: { color: border },
                        ticks: { color: muted, precision: 0, maxTicksLimit: 6, callback: (v) => compactNumber(v) },
                    },
                },
            },
            plugins: [restOfPeriod],
        };
    }

    /** Hour / Day / Week: the bins the period can be shown in; choosing one reloads the usage. */
    function binControl(usage) {
        const group = el('div', 'usage-bins');
        group.setAttribute('role', 'group');
        group.setAttribute('aria-label', 'Show usage per');
        ['HOUR', 'DAY', 'WEEK'].forEach((bin) => {
            const button = el('button', 'usage-bin', BIN_LABELS[bin]);
            button.type = 'button';
            button.setAttribute('aria-pressed', String(usage.interval === bin));
            button.disabled = !(usage.intervals || []).includes(bin);
            if (button.disabled) button.title = 'Not available for this period';
            button.addEventListener('click', () => {
                if (usage.interval === bin) return;
                interval = bin;
                load();
            });
            group.append(button);
        });
        return group;
    }

    /** Plain bars, when Chart.js could not be loaded. */
    function plainBars(usage, unitText) {
        const buckets = usage.buckets;
        const max = Math.max(1, ...buckets.filter((b) => b.value !== null).map((b) => b.value));
        const bars = el('div', 'usage-chart');
        buckets.forEach((b) => {
            const bar = el('div', b.value === null ? 'usage-bar usage-bar--ahead' : 'usage-bar');
            if (b.value !== null) {
                bar.style.height = ((b.value / max) * 100).toFixed(1) + '%';
                bar.title = bucketRange(b.start, usage.interval) + ': ' + unitText(b.value);
            }
            bars.append(bar);
        });
        const axis = el('div', 'usage-axis');
        axis.setAttribute('aria-hidden', 'true');
        axis.append(el('span', null, bucketLabel(buckets[0].start, usage.interval)),
            el('span', null, bucketLabel(buckets[buckets.length - 1].start, usage.interval)));
        const holder = el('div');
        holder.append(bars, axis);
        return holder;
    }

    /** One unit's chart, titled by the API and plan; the first carries the Hour / Day / Week choice, which applies to all. */
    function renderChart(usage, withBins, title) {
        const bin = usage.interval;
        const buckets = usage.buckets;
        const counted = buckets.filter((b) => b.value !== null);
        const unitText = (n) => unitCount(n, usage.unit);
        const peak = counted.reduce((a, b) => (b.value > a.value ? b : a), { value: 0 });

        const section = el('section', 'usage-section usage-chart-section');
        const head = el('div', 'usage-chart-head');
        const heading = el('div', 'usage-chart-heading');
        const counts = billable(usage.unit) + ' per ' + BIN_NAMES[bin];
        heading.append(el('h2', 'usage-section-title', title),
            el('p', 'usage-chart-sub', peak.value > 0
                ? counts + ' · Times in UTC · Peak ' + unitText(peak.value) + ', ' + bucketRange(peak.start, bin)
                : counts + ' · Times in UTC'));
        head.append(heading);
        if (withBins) head.append(binControl(usage));
        section.append(head);
        if (peak.value === 0) {
            section.append(el('p', 'usage-muted', 'No usage in this period.'));
            return section;
        }
        if (!window.Chart) {
            section.append(plainBars(usage, unitText));
            return section;
        }
        const holder = el('div', 'usage-chart-canvas');
        const canvas = el('canvas');
        canvas.setAttribute('role', 'img');
        canvas.setAttribute('aria-label', title + ', ' + counts + ', '
            + bucketRange(buckets[0].start, bin) + ' to ' + bucketRange(buckets[buckets.length - 1].start, bin)
            + '. Total ' + unitText(usage.total) + '; peak ' + unitText(peak.value) + ', ' + bucketRange(peak.start, bin) + '.');
        holder.append(canvas);
        section.append(holder);
        // The chart measures its holder, so it is drawn once the section is on the page.
        requestAnimationFrame(() => { charts.push(new window.Chart(canvas, chartConfig(usage, unitText))); });
        return section;
    }

    function render(data) {
        loading(false);
        if (data.subscriptions.length === 0) {
            show('usageEmpty', true);
            return;
        }
        if (subscriptionSelect.options.length === 0) {
            data.subscriptions.forEach((s) => {
                const option = el('option', null, s.apiName + ' · ' + s.planName);
                option.value = s.id;
                subscriptionSelect.append(option);
            });
            show('usageFilters', true);
        }
        subscriptionSelect.value = data.subscriptionId;
        show('usageContent', true);
        show('usagePaymentPending', data.status === 'PAYMENT_PENDING');
        show('usageUnavailable', !data.usageAvailable);
        renderCards(data);
        charts.forEach((c) => c.destroy());
        charts = [];
        const shown = seriesOf(data).filter((s) => s.buckets && s.buckets.length);
        const sub = data.subscriptions.find((x) => x.id === data.subscriptionId);
        const title = sub ? sub.apiName + ' · ' + sub.planName : '';
        document.getElementById('usageCharts').replaceChildren(...shown.map((s, i) => renderChart(s, i === 0, title)));
    }

    async function load() {
        const request = ++latest;
        loading(true);
        const params = new URLSearchParams({ period: periodSelect.value });
        // The first load may name a subscription in the page address (?subscriptionId=).
        const subscriptionId = subscriptionSelect.value || requestedId;
        requestedId = null;
        if (subscriptionId) params.set('subscriptionId', subscriptionId);
        if (interval) params.set('interval', interval);
        try {
            const resp = await fetch(apiPortalApi.root('/usage?' + params.toString()), {
                headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': window.apiPortalApi.csrfToken() },
            });
            // A subscription named in the address that is not the caller's: show their first one.
            if (resp.status === 404 && subscriptionId && !subscriptionSelect.value) return load();
            // A bin the period does not offer: show the period's default.
            if (resp.status === 400 && interval) {
                interval = null;
                return load();
            }
            if (!resp.ok) throw new Error('HTTP ' + resp.status);
            const data = await resp.json();
            if (request === latest) render(data);
        } catch (_) {
            if (request !== latest) return;
            loading(false);
            show('usageContent', true);
            show('usageUnavailable', true);
        }
    }

    // Another subscription or period starts from that period's default bin.
    function reload() {
        interval = null;
        load();
    }

    load();
    subscriptionSelect.addEventListener('change', reload);
    periodSelect.addEventListener('change', reload);
})();
