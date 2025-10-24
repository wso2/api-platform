/* eslint-disable prettier/prettier */
/* eslint-disable max-len */

import * as React from 'react';


export default function (props: React.SVGProps<SVGSVGElement>){
  return (
    <svg {...props}>
      <g><React.Fragment><svg viewBox="0 0 20 20"><defs><linearGradient x1="50%" x2="50%" y1="0%" y2="100%"><stop offset="0%" stop-color="#FFAD52" /><stop offset="100%" stop-color="#FF9D52" /></linearGradient><circle cx="9" cy="9" r="9" /><filter width="122.2%" height="122.2%" x="-11.1%" y="-5.6%" filterUnits="objectBoundingBox"><feOffset dy="1" in="SourceAlpha" result="shadowOffsetOuter1" /><feGaussianBlur in="shadowOffsetOuter1" result="shadowBlurOuter1" stdDeviation=".5" /><feColorMatrix in="shadowBlurOuter1" values="0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0.180725524 0" /></filter></defs><g fill="none" fillRule="evenodd" stroke="none" stroke-width="1"><g fillRule="nonzero" transform="translate(-104 -403) translate(105 403)"><use xlinkHref="#b" fill="black" filter="url(#a)" /><use xlinkHref="#b" fill="url(#c)" /></g><path fill="#FFFFFF" d="M9 12c.5523 0 1 .4477 1 1s-.4477 1-1 1-1-.4477-1-1 .4477-1 1-1m0-8c.5523 0 1 .4477 1 1v4c0 .5523-.4477 1-1 1s-1-.4477-1-1V5c0-.5523.4477-1 1-1" transform="translate(-104 -403) translate(105 403) matrix(1 0 0 -1 0 18)" /></g></svg></React.Fragment></g>
    </svg>
  )
}
    

