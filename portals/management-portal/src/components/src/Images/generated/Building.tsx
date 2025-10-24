/* eslint-disable prettier/prettier */
/* eslint-disable max-len */

import * as React from 'react';


export default function (props: React.SVGProps<SVGSVGElement>){
  return (
    <svg {...props}>
      <g><React.Fragment><svg preserveAspectRatio="xMidYMid" viewBox="0 0 100 100"><circle cx="50" cy="50" r="46" fill="none" stroke="#5567d5" stroke-dasharray="72.2566 72.2566" stroke-linecap="round" stroke-width="10"><animateTransform attributeName="transform" dur="1s" keyTimes="0;1" repeatCount="indefinite" type="rotate" values="0 50 50;360 50 50" /></circle><circle cx="50" cy="50" r="35" fill="none" stroke="#ccd1f2" stroke-dasharray="54.9779 54.9779" stroke-dashoffset="54.9779" stroke-linecap="round" stroke-width="10"><animateTransform attributeName="transform" dur="1s" keyTimes="0;1" repeatCount="indefinite" type="rotate" values="0 50 50;-360 50 50" /></circle></svg></React.Fragment></g>
    </svg>
  )
}
    

