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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { Box } from '@wso2/oxygen-ui';
import { useEffect, useId, useRef } from 'react';

import { usePrefersReducedMotion } from '@/hooks/usePrefersReducedMotion';

// Artwork colours, fixed on purpose: the theme has no indigo, and its primary
// is a redder orange than the mark this drawing belongs to.
const INDIGO = '#6C75E8';
const INDIGO_BRIGHT = '#8E98FA';
const ORANGE = '#F07C45';
const ORANGE_BRIGHT = '#F5935C';

const VIEW_WIDTH = 480;
const VIEW_HEIGHT = 330;
const CENTER_X = VIEW_WIDTH / 2;
const CENTER_Y = VIEW_HEIGHT / 2;
const STRANDS = 34;
const SPREAD = 150;
const FLAT = 0.1;

type Side = 'left' | 'right';
type Point = { x: number; y: number };

// A strand runs flat along the outer edge, then bends in to the centre.
const strandPoints = (index: number, side: Side): [Point, Point, Point, Point] => {
  const startY = CENTER_Y + (index / (STRANDS - 1) - 0.5) * 2 * SPREAD;
  const origin = side === 'left' ? 0 : VIEW_WIDTH;
  const sign = side === 'left' ? 1 : -1;
  const at = (fraction: number) => origin + sign * CENTER_X * fraction;
  return [
    { x: at(FLAT), y: startY },
    { x: at(0.58), y: startY },
    { x: at(0.72), y: CENTER_Y },
    { x: CENTER_X, y: CENTER_Y },
  ];
};

const strandPath = (index: number, side: Side): string => {
  const [start, c1, c2, end] = strandPoints(index, side);
  const origin = side === 'left' ? 0 : VIEW_WIDTH;
  return `M ${origin} ${start.y} L ${start.x} ${start.y} C ${c1.x} ${c1.y}, ${c2.x} ${c2.y}, ${end.x} ${end.y}`;
};

const bezier = (points: [Point, Point, Point, Point], t: number): Point => {
  const u = 1 - t;
  const [a, b, c, d] = points;
  return {
    x: u * u * u * a.x + 3 * u * u * t * b.x + 3 * u * t * t * c.x + t * t * t * d.x,
    y: u * u * u * a.y + 3 * u * u * t * b.y + 3 * u * t * t * c.y + t * t * t * d.y,
  };
};

// Deterministic, so the specks sit in the same place on every render.
const seeded = (seed: number) => () => {
  seed = (seed + 0x6d2b79f5) | 0;
  let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
  t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};

const specks = (side: Side) => {
  const random = seeded(side === 'left' ? 7 : 19);
  return Array.from({ length: 18 }, () => {
    const index = Math.floor(random() * STRANDS);
    // Weighted toward the centre, where the strands gather.
    const t = 0.15 + 0.8 * Math.sqrt(random());
    const point = bezier(strandPoints(index, side), t);
    return { ...point, opacity: 0.35 + random() * 0.5, radius: 0.8 + random() * 1.8 };
  });
};

const strands = Array.from({ length: STRANDS }, (_, index) => index);
const leftSpecks = specks('left');
const rightSpecks = specks('right');

/**
 * Decorative strands converging on a single point: requests fanning in and out
 * of one API. Both ends fade out, so the drawing melts into whatever surface
 * it sits on, light or dark.
 */
export function ConvergingStrandsIllustration() {
  const svgRef = useRef<SVGSVGElement>(null);
  const reducedMotion = usePrefersReducedMotion();

  // The drawing leans a little toward the pointer: it publishes where the
  // pointer is, relative to the drawing, as --px and --py (-1 to 1), and each
  // layer moves by its own share of that, so the layers read as having depth.
  useEffect(() => {
    const svg = svgRef.current;
    if (!svg || reducedMotion) return undefined;
    let frame = 0;
    const lean = (x: number, y: number) => {
      svg.style.setProperty('--px', x.toFixed(3));
      svg.style.setProperty('--py', y.toFixed(3));
    };
    const onMove = (event: PointerEvent) => {
      if (event.pointerType === 'touch') return;
      if (!Number.isFinite(event.clientX) || !Number.isFinite(event.clientY)) return;
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        const box = svg.getBoundingClientRect();
        if (box.width === 0 || box.height === 0) return;
        const clamp = (value: number) => Math.max(-1, Math.min(1, value));
        lean(
          clamp((event.clientX - (box.left + box.width / 2)) / box.width),
          clamp((event.clientY - (box.top + box.height / 2)) / box.height),
        );
      });
    };
    const onLeave = () => {
      cancelAnimationFrame(frame);
      lean(0, 0);
    };
    window.addEventListener('pointermove', onMove, { passive: true });
    document.documentElement.addEventListener('pointerleave', onLeave);
    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener('pointermove', onMove);
      document.documentElement.removeEventListener('pointerleave', onLeave);
    };
  }, [reducedMotion]);

  // Gradient ids are document-wide: two drawings on a page must not share them.
  const uid = useId().replace(/:/g, '');
  const leftId = `strands-left-${uid}`;
  const rightId = `strands-right-${uid}`;
  const glowId = `strands-glow-${uid}`;
  return (
    <Box
      aria-hidden="true"
      component="svg"
      preserveAspectRatio="xMidYMid meet"
      ref={svgRef}
      sx={{
        display: 'block',
        height: 'auto',
        maxWidth: '100%',
        width: '100%',
        '& .strand': { fill: 'none', strokeWidth: 0.9, vectorEffect: 'non-scaling-stroke' },
        '& .strand-left': { stroke: `url(#${leftId})` },
        '& .strand-right': { stroke: `url(#${rightId})` },
        '& .fill-left': { fill: INDIGO_BRIGHT },
        '& .fill-right': { fill: ORANGE_BRIGHT },
        '& .stop-left': { stopColor: INDIGO },
        '& .stop-left-bright': { stopColor: INDIGO_BRIGHT },
        '& .stop-right': { stopColor: ORANGE },
        '& .stop-right-bright': { stopColor: ORANGE_BRIGHT },
        '& .stop-glow': { stopColor: ORANGE },
        // Each layer takes its own share of the lean; further layers move less.
        '& .layer': { transition: 'transform 300ms ease-out' },
        '& .layer-glow': {
          transform: 'translate(calc(var(--px, 0) * 18px), calc(var(--py, 0) * 12px))',
        },
        '& .layer-left': {
          transform: 'translate(calc(var(--px, 0) * 3px), calc(var(--py, 0) * 3px))',
        },
        '& .layer-right': {
          transform: 'translate(calc(var(--px, 0) * 5px), calc(var(--py, 0) * 4px))',
        },
        '& .layer-specks': {
          transform: 'translate(calc(var(--px, 0) * 10px), calc(var(--py, 0) * 8px))',
        },
        '& .layer-core': {
          transform: 'translate(calc(var(--px, 0) * 7px), calc(var(--py, 0) * 6px))',
        },
      }}
      viewBox={`0 0 ${VIEW_WIDTH} ${VIEW_HEIGHT}`}
    >
      <defs>
        <linearGradient
          gradientUnits="userSpaceOnUse"
          id={leftId}
          x1="0"
          x2={CENTER_X}
          y1="0"
          y2="0"
        >
          <stop className="stop-left" offset="0" stopOpacity="0" />
          <stop className="stop-left" offset="0.35" stopOpacity="0.5" />
          <stop className="stop-left-bright" offset="1" stopOpacity="1" />
        </linearGradient>
        <linearGradient
          gradientUnits="userSpaceOnUse"
          id={rightId}
          x1={VIEW_WIDTH}
          x2={CENTER_X}
          y1="0"
          y2="0"
        >
          <stop className="stop-right" offset="0" stopOpacity="0" />
          <stop className="stop-right" offset="0.35" stopOpacity="0.55" />
          <stop className="stop-right-bright" offset="1" stopOpacity="1" />
        </linearGradient>
        <radialGradient id={glowId}>
          <stop className="stop-glow" offset="0" stopOpacity="0.22" />
          <stop className="stop-glow" offset="1" stopOpacity="0" />
        </radialGradient>
      </defs>
      <g className="layer layer-glow">
        <ellipse cx={CENTER_X} cy={CENTER_Y} fill={`url(#${glowId})`} rx={150} ry={110} />
      </g>
      <g className="layer layer-left">
        {strands.map((index) => (
          <path className="strand strand-left" d={strandPath(index, 'left')} key={`l${index}`} />
        ))}
      </g>
      <g className="layer layer-right">
        {strands.map((index) => (
          <path className="strand strand-right" d={strandPath(index, 'right')} key={`r${index}`} />
        ))}
      </g>
      <g className="layer layer-specks">
        {leftSpecks.map((speck, index) => (
          <circle
            className="fill-left"
            cx={speck.x}
            cy={speck.y}
            fillOpacity={speck.opacity}
            key={`ls${index}`}
            r={speck.radius}
          />
        ))}
        {rightSpecks.map((speck, index) => (
          <circle
            className="fill-right"
            cx={speck.x}
            cy={speck.y}
            fillOpacity={speck.opacity}
            key={`rs${index}`}
            r={speck.radius}
          />
        ))}
      </g>
      <g className="layer layer-core">
        <circle className="fill-left" cx={CENTER_X - 4} cy={CENTER_Y} r={2} />
        <circle className="fill-right" cx={CENTER_X + 4} cy={CENTER_Y} r={2.5} />
      </g>
    </Box>
  );
}
