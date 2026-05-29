import React from 'react';

type McpMenuIconProps = React.SVGProps<SVGSVGElement> & {
  size?: number;
};

export default function McpMenuIcon({
  size = 25,
  ...props
}: McpMenuIconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 90 90"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      {...props}
    >
      <path
        d="M9 42.4265L42.9411 8.48544C47.6274 3.79913 55.2255 3.79913 59.9115 8.48544C64.598 13.1717 64.598 20.7697 59.9115 25.456L34.279 51.0886"
        stroke="currentColor"
        strokeWidth={6}
        strokeLinecap="round"
      />
      <path
        d="M34.6326 50.7353L59.9115 25.4561C64.598 20.7698 72.196 20.7698 76.8825 25.4561L77.059 25.6329C81.7455 30.3192 81.7455 37.9172 77.059 42.6034L46.3624 73.3003C44.8003 74.8623 44.8003 77.3948 46.3624 78.9568L52.6655 85.2603"
        stroke="currentColor"
        strokeWidth={6}
        strokeLinecap="round"
      />
      <path
        d="M51.4264 16.9707L26.3241 42.073C21.6378 46.7593 21.6378 54.3572 26.3241 59.0437C31.0104 63.7297 38.6083 63.7297 43.2946 59.0437L68.397 33.9413"
        stroke="currentColor"
        strokeWidth={6}
        strokeLinecap="round"
      />
    </svg>
  );
}
