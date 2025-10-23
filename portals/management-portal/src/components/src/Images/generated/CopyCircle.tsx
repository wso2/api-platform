/* eslint-disable prettier/prettier */
/* eslint-disable max-len */

import * as React from 'react';


export default function (props: React.SVGProps<SVGSVGElement>){
  return (
    <svg {...props}>
      <g><React.Fragment><svg viewBox="0 0 32 32"><defs><rect width="32" height="32" x="0" y="0" rx="16" /><filter width="100%" height="100%" x="0%" y="0%" filterUnits="objectBoundingBox"><feOffset in="SourceAlpha" result="shadowOffsetOuter1" /><feComposite in="shadowOffsetOuter1" in2="SourceAlpha" operator="out" result="shadowOffsetOuter1" /><feColorMatrix in="shadowOffsetOuter1" values="0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0.07 0" /></filter><rect width="32" height="32" x="0" y="0" rx="16" /><filter width="103.1%" height="103.1%" x="-1.6%" y="-1.6%" filterUnits="objectBoundingBox"><feMorphology in="SourceAlpha" radius="1" result="shadowSpreadInner1" /><feOffset in="shadowSpreadInner1" result="shadowOffsetInner1" /><feComposite in="shadowOffsetInner1" in2="SourceAlpha" k2="-1" k3="1" operator="arithmetic" result="shadowInnerInner1" /><feColorMatrix in="shadowInnerInner1" values="0 0 0 0 1 0 0 0 0 1 0 0 0 0 1 0 0 0 0.302229021 0" /></filter></defs><g fill="none" fillRule="evenodd" stroke="none" stroke-width="1" transform="translate(-836 -344) translate(836 344)"><g fill="#FFFFFF" fillRule="nonzero"><use xlinkHref="#b" filter="url(#a)" /><use xlinkHref="#b" fillOpacity="0" /></g><circle cx="16" cy="16" r="16" /><use xlinkHref="#c" fill="#F7F8FB" /><g fill="#FFFFFF" opacity=".2048"><use xlinkHref="#d" fillOpacity=".8" /><use xlinkHref="#d" filter="url(#e)" /></g><g transform="translate(10 10)"><path d="M7 3c1.1046 0 2 .8954 2 2v5c0 1.1046-.8954 2-2 2H2c-1.1046 0-2-.8954-2-2V5c0-1.1046.8954-2 2-2Zm0 2H2v5h5zm2-5c1.5977 0 2.9037 1.249 2.995 2.8237L12 3v5c0 .5523-.4477 1-1 1-.5128 0-.9355-.386-.9933-.8834L10 8V3c0-.5128-.386-.9355-.8834-.9933L9 2H5c-.5523 0-1-.4477-1-1 0-.5128.386-.9355.8834-.9933L5 0z" /><use xlinkHref="#f" fill="#CBCEDB" fillRule="nonzero" stroke="none" /></g></g></svg></React.Fragment></g>
    </svg>
  )
}
    

