/* eslint-disable prettier/prettier */
/* eslint-disable max-len */

import * as React from 'react';


export default function (props: React.SVGProps<SVGSVGElement>){
  return (
    <svg {...props}>
      <g><React.Fragment><svg viewBox="0 0 20 20"><defs><linearGradient x1="50%" x2="50%" y1="0%" y2="100%"><stop offset="0%" stop-color="#53C08A" /><stop offset="100%" stop-color="#36B475" /></linearGradient><circle cx="9" cy="9" r="9" /><filter width="122.2%" height="122.2%" x="-11.1%" y="-5.6%" filterUnits="objectBoundingBox"><feOffset dy="1" in="SourceAlpha" result="shadowOffsetOuter1" /><feGaussianBlur in="shadowOffsetOuter1" result="shadowBlurOuter1" stdDeviation=".5" /><feColorMatrix in="shadowBlurOuter1" values="0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0.1 0" /></filter></defs><g fill="none" fillRule="evenodd" stroke="none" stroke-width="1"><g fillRule="nonzero" transform="translate(-1356 -386) translate(1357 386)"><use xlinkHref="#b" fill="black" filter="url(#a)" /><use xlinkHref="#b" fill="url(#c)" /></g><path fill="#FFFFFF" d="M5.5 10.7929c-.5523 0-1-.4477-1-1V6.207c0-.5523.4477-1 1-1s1 .4477 1 1v2.585l6 .0008c.5523 0 1 .4477 1 1s-.4477 1-1 1z" transform="translate(-1356 -386) translate(1357 386) rotate(-45 9 8)" /></g></svg></React.Fragment></g>
    </svg>
  )
}
    

