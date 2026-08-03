import { Box, CodeBlock, IconButton, Tooltip } from '@wso2/oxygen-ui';
import { Check, Copy } from '@wso2/oxygen-ui-icons-react';
import { useState } from 'react';

/** A bash CodeBlock with a copy-to-clipboard button overlaid in the corner. */
export function CopyableCommand({ code }: { code: string }) {
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // clipboard unavailable
    }
  };

  return (
    <Box sx={{ position: 'relative' }}>
      <Tooltip title={copied ? 'Copied' : 'Copy'}>
        <IconButton
          aria-label="Copy command"
          onClick={copy}
          size="small"
          sx={{ position: 'absolute', right: 8, top: 8, zIndex: 1 }}
        >
          {copied ? <Check size={16} /> : <Copy size={16} />}
        </IconButton>
      </Tooltip>
      <CodeBlock code={code} language="bash" />
    </Box>
  );
}
