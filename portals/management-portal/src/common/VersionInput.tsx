import * as React from "react";
import { Box, TextField, Typography } from "@mui/material";
import { formatVersionToMajorMinor } from "../helpers/openApiHelpers";

type Props = {
  value?: string;
  onChange?: (version: string) => void;
  disabled?: boolean;
};

const VersionInput: React.FC<Props> = ({ value, onChange, disabled }) => {
  // Normalize incoming value into major/minor display parts
  const normalized = React.useMemo(() => formatVersionToMajorMinor(value || ""), [value]);
  const initialParts = normalized.replace(/^v/, "").split('.');

  // Internal states allow empty strings so the user can clear fields while editing
  const [majorState, setMajorState] = React.useState<string>(initialParts[0] ?? '1');
  const [minorState, setMinorState] = React.useState<string>(initialParts[1] ?? '0');

  // Sync when parent value changes (but avoid clobbering while user types)
  React.useEffect(() => {
    const p = normalized.replace(/^v/, '').split('.');
    const m = p[0] ?? '1';
    const n = p[1] ?? '0';
    setMajorState(m);
    setMinorState(n);
  }, [normalized]);

  const emitIfComplete = (m: string, n: string) => {
    // If either field is empty, emit empty string to signal invalid/incomplete
    if (m === "" || n === "") {
      onChange?.("");
      return;
    }
    onChange?.(`v${m}.${n}`);
  };

  return (
    <Box>
      <Typography variant="body2" sx={{ mb: 0.5 }}>
        Version
      </Typography>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
        <TextField
          size="small"
          value={majorState}
          onChange={(e) => {
            // allow empty to permit clearing; strip nondigits
            const digits = e.target.value.replace(/\D/g, "");
            const next = digits;
            setMajorState(next);
            emitIfComplete(next, minorState);
          }}
          inputProps={{ inputMode: "numeric", pattern: "[0-9]*" }}
          disabled={disabled}
          sx={{ width: 80 }}
        />

        <Typography variant="body2">.</Typography>

        <TextField
          size="small"
          value={minorState}
          onChange={(e) => {
            const digits = e.target.value.replace(/\D/g, "");
            const next = digits;
            setMinorState(next);
            emitIfComplete(majorState, next);
          }}
          inputProps={{ inputMode: "numeric", pattern: "[0-9]*" }}
          disabled={disabled}
          sx={{ width: 80 }}
        />
      </Box>
    </Box>
  );
};

export default VersionInput;
