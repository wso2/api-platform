/* eslint-disable prettier/prettier */
/* eslint-disable max-len */

import * as React from 'react';


export default function (props: React.SVGProps<SVGSVGElement>){
  return (
    <svg {...props}>
      <g><React.Fragment><svg viewBox="0 0 35 35"><defs><linearGradient x1=".5" x2=".5" y2="1" gradientUnits="objectBoundingBox"><stop offset="0" stop-color="#fff" /><stop offset="1" stop-color="#f7f8fb" /></linearGradient><filter width="35" height="35" x="0" y="0" filterUnits="userSpaceOnUse"><feOffset dy="1" /><feGaussianBlur result="blur" stdDeviation=".5" /><feFlood flood-color="#cbcfda" /><feComposite in2="blur" operator="in" /><feComposite in="SourceGraphic" /></filter></defs><g><g filter="url(#a)" transform="translate(1.5 .5) translate(-1.5 -.5)"><circle cx="16" cy="16" r="16" fill="url(#b)" transform="translate(1.5 .5)" /></g><path fill="#8d91a3" d="M7.465 7.365a1 1 0 0 1 0-1.415l1.12-1.121H1a1 1 0 0 1 0-2h7.586L7.465 1.707A1 1 0 1 1 8.878.293l2.657 2.657a1 1 0 0 1 .278.879 1 1 0 0 1-.278.878L8.878 7.365a1 1 0 0 1-1.414 0Z" transform="translate(1.5 .5) translate(9 12.172)" /></g></svg></React.Fragment></g>
    </svg>
  )
}
    

