/* eslint-disable prettier/prettier */
/* eslint-disable max-len */

import * as React from 'react';


export default function (props: React.SVGProps<SVGSVGElement>){
  return (
    <svg {...props}>
      <g><React.Fragment><svg viewBox="0 0 26 26"><defs><linearGradient x1="50%" x2="50%" y1="0%" y2="100%"><stop offset="0%" stop-color="#53C08A" /><stop offset="100%" stop-color="#36B475" /></linearGradient><circle cx="12" cy="12" r="12" /><filter width="116.7%" height="116.7%" x="-8.3%" y="-4.2%" filterUnits="objectBoundingBox"><feOffset dy="1" in="SourceAlpha" result="shadowOffsetOuter1" /><feGaussianBlur in="shadowOffsetOuter1" result="shadowBlurOuter1" stdDeviation=".5" /><feColorMatrix in="shadowBlurOuter1" values="0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0.1 0" /></filter></defs><g fill="none" fillRule="evenodd" stroke="none" stroke-width="1"><g fillRule="nonzero" transform="translate(-103 -95) translate(104 95)"><use xlinkHref="#b" fill="black" filter="url(#a)" /><use xlinkHref="#b" fill="url(#c)" /></g><path fill="#FFFFFF" d="M7 14.3905c-.5523 0-1-.4477-1-1V7.9428c0-.5523.4477-1 1-1h.6667c.5523 0 1 .4477 1 1l-.0007 3.781H17c.5523 0 1 .4478 1 1v.6667c0 .5523-.4477 1-1 1z" transform="translate(-103 -95) translate(104 95) rotate(-45 12 10.6667)" /></g></svg></React.Fragment></g>
    </svg>
  )
}
    

