/* eslint-disable prettier/prettier */
/* eslint-disable max-len */

import * as React from 'react';


export default function (props: React.SVGProps<SVGSVGElement>){
  return (
    <svg {...props}>
      <g><React.Fragment><svg viewBox="0 0 21 19"><defs><linearGradient x1=".5" x2=".5" y2="1" gradientUnits="objectBoundingBox"><stop offset="0" stop-color="#ea4c4d" /><stop offset="1" stop-color="#d64142" /></linearGradient><filter width="21" height="19" x="0" y="0" filterUnits="userSpaceOnUse"><feOffset dy="1" /><feGaussianBlur result="blur" stdDeviation=".5" /><feFlood flood-opacity=".18" /><feComposite in2="blur" operator="in" /><feComposite in="SourceGraphic" /></filter></defs><g><g filter="url(#a)" transform="translate(1.5 .5) translate(-1.5 -.5)"><path fill="url(#b)" d="M17.674 12.469 11.022 1.156a2.343 2.343 0 0 0-4.041 0L.33 12.469a2.34 2.34 0 0 0-.03 2.344A2.31 2.31 0 0 0 2.32 16h13.333a2.36 2.36 0 0 0 2.02-3.531Z" transform="translate(1.5 .5)" /></g><path fill="#fff" d="M0 9a1 1 0 1 1 1 1 1 1 0 0 1-1-1m0-4V1a1 1 0 0 1 2 0v4a1 1 0 0 1-2 0" transform="translate(1.5 .5) translate(8 4)" /></g></svg></React.Fragment></g>
    </svg>
  )
}
    

