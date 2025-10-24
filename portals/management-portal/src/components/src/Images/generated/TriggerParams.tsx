/* eslint-disable prettier/prettier */
/* eslint-disable max-len */

import * as React from 'react';


export default function (props: React.SVGProps<SVGSVGElement>){
  return (
    <svg {...props}>
      <g><React.Fragment><svg viewBox="0 0 104 31"><defs><filter width="104" height="31" x="0" y="0" filterUnits="userSpaceOnUse"><feOffset dy="1" /><feGaussianBlur result="blur" stdDeviation="1" /><feFlood flood-color="#a9acb6" flood-opacity=".302" /><feComposite in2="blur" operator="in" /><feComposite in="SourceGraphic" /></filter></defs><g filter="url(#a)" transform="translate(3.5 2.5) translate(-3.5 -2.5)"><rect width="97" height="24" fill="#f0f1fb" stroke="#fff" stroke-miterlimit="10" stroke-width="1" rx="12" transform="translate(3.5 2.5)" /></g><text fill="#40404b" font-family="GilmerMedium, Gilmer Medium" font-size="10" transform="translate(3.5 2.5) translate(48.5 12)"><tspan x="-35.83" y="0">calller, req, self</tspan></text></svg></React.Fragment></g>
    </svg>
  )
}
    

