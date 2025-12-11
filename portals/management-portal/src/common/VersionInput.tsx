import * as React from "react";
import { Box, Typography } from "@mui/material";
import { TextInput } from "../components/src/components/TextInput";
import { formatVersionToMajorMinor } from "../helpers/openApiHelpers";

type Props = {
  value?: string;
  onChange?: (version: string) => void;
  disabled?: boolean;
  label?: string;
};

const VersionInput: React.FC<Props> = ({ value, onChange, disabled, label }) => {
  // Normalize incoming value into major/minor display parts
  const normalized = React.useMemo(() => formatVersionToMajorMinor(value || ""), [value]);
  const initialParts = normalized.replace(/^v/, "").split('.');

  // Internal states allow empty strings so the user can clear fields while editing
  const [majorState, setMajorState] = React.useState<string>(initialParts[0]);
  const [minorState, setMinorState] = React.useState<string>(initialParts[1]);
  const touchedRef = React.useRef<boolean>(false);

  React.useEffect(() => {
    const p = normalized.replace(/^v/, '').split('.');
    const m = p[0] ?? '';
    const n = p[1] ?? '';
    if (!touchedRef.current) {
      setMajorState(m);
      setMinorState(n);
    }
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
        <Box sx={{ width: 96 }}>
          <TextInput
            label={label ?? ""}
            size="medium"
            value={majorState}
            onChange={(v: string) => {
              touchedRef.current = true;
              const digits = v.replace(/\D/g, "");
              setMajorState(digits);
              emitIfComplete(digits, minorState);
            }}
            disabled={disabled}
            inputPropsJson={{ inputMode: "numeric", pattern: "[0-9]*" }}
            fullWidth
            testId="version-major"
          />
        </Box>

        <Typography variant="body2" sx={{ mt: 4 }}>.</Typography>

        <Box sx={{ width: 96, mt:3 }}>
          <TextInput
            size="medium"
            value={minorState}
            onChange={(v: string) => {
              touchedRef.current = true;
              const digits = v.replace(/\D/g, "");
              setMinorState(digits);
              emitIfComplete(majorState, digits);
            }}
            disabled={disabled}
            inputPropsJson={{ inputMode: "numeric", pattern: "[0-9]*" }}
            fullWidth
            testId="version-minor"
          />
        </Box>
      </Box>
    </Box>
  );
};

export default VersionInput;
