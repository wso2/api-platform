/* eslint-disable prettier/prettier */
/* eslint-disable max-len */

import * as React from 'react';


export default function (props: React.SVGProps<SVGSVGElement>){
  return (
    <svg {...props}>
      <g><React.Fragment><svg viewBox="0 0 58 58"><defs><linearGradient x1="50%" x2="50%" y1="0%" y2="100%"><stop offset="0%" stop-color="#FFFFFF" /><stop offset="100%" stop-color="#F7F8FB" /></linearGradient><circle cx="27" cy="27" r="27" /><filter width="109.3%" height="109.3%" x="-4.6%" y="-2.8%" filterUnits="objectBoundingBox"><feMorphology in="SourceAlpha" operator="dilate" radius=".5" result="shadowSpreadOuter1" /><feOffset dy="1" in="shadowSpreadOuter1" result="shadowOffsetOuter1" /><feGaussianBlur in="shadowOffsetOuter1" result="shadowBlurOuter1" stdDeviation=".5" /><feComposite in="shadowBlurOuter1" in2="SourceAlpha" operator="out" result="shadowBlurOuter1" /><feColorMatrix in="shadowBlurOuter1" values="0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0.180725524 0" /></filter></defs><g fill="none" fillRule="evenodd" stroke="none" stroke-width="1"><g fillRule="nonzero" transform="translate(-691 -207) translate(693 208)"><use xlinkHref="#b" fill="black" filter="url(#a)" /><use xlinkHref="#b" fill="url(#c)" stroke="#F0F1FB" /></g><path fill="#5567D5" d="M28.5 22.5c.8284 0 1.5.6716 1.5 1.5q-.0001.1163-.0172.2277c.011.0888.0172.1798.0172.2723v13c0 1.1046-.8954 2-2 2s-2-.8954-2-2v-12h-1.5c-.8284 0-1.5-.6716-1.5-1.5s.6716-1.5 1.5-1.5zm-1-8.5c1.3807 0 2.5 1.1193 2.5 2.5S28.8807 19 27.5 19 25 17.8807 25 16.5s1.1193-2.5 2.5-2.5" transform="translate(-691 -207) translate(693 208)" /></g></svg></React.Fragment></g>
    </svg>
  )
}
    

