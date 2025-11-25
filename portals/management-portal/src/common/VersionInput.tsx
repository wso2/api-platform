import * as React from "react";
import { Box, TextField, Typography } from "@mui/material";
import { formatVersionToMajorMinor } from "../helpers/openApiHelpers";

type Props = {
  value?: string;
  onChange?: (version: string) => void;
  disabled?: boolean;
  label?: string;
  inputHeight?: number | string;
};

const VersionInput: React.FC<Props> = ({ value, onChange, disabled, label, inputHeight }) => {
  // Normalize incoming value into major/minor display parts
  const normalized = React.useMemo(() => formatVersionToMajorMinor(value || ""), [value]);
  const initialParts = normalized.replace(/^v/, "").split('.');

  // Internal states allow empty strings so the user can clear fields while editing
  const [majorState, setMajorState] = React.useState<string>(initialParts[0] ?? '1');
  const [minorState, setMinorState] = React.useState<string>(initialParts[1] ?? '0');

  React.useEffect(() => {
    const p = normalized.replace(/^v/, '').split('.');
    const m = p[0] ?? '1';
    const n = p[1] ?? '0';
    setMajorState(m);
    setMinorState(n);
  }, [normalized]);

  const emitIfComplete = (m: string, n: string) => {
    if (m === "" || n === "") {
      onChange?.("");
      return;
    }
    onChange?.(`v${m}.${n}`);
  };

  return (
    <Box>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
        <TextField
          size="small"
          label={label ?? ""}
          required={true}
          InputLabelProps={{ shrink: true }}
          value={majorState}
          onChange={(e) => {
            const digits = e.target.value.replace(/\D/g, "");
            const next = digits;
            setMajorState(next);
            emitIfComplete(next, minorState);
          }}
          inputProps={{ inputMode: "numeric", pattern: "[0-9]*" }}
          disabled={disabled}
          sx={{ width: 80, height: inputHeight ?? undefined }}
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
          sx={{ width: 80, height: inputHeight ?? undefined }}
        />
      </Box>
    </Box>
  );
};

export default VersionInput;
