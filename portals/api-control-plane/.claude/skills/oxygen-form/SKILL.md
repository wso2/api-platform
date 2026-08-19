---
name: oxygen-form
description: Generate forms with Form.* components and validation. Use when creating input forms, multi-step wizards, or form validation.
---

# Generate Oxygen UI Form

## Instructions

1. Read `.claude/oxygen-ui/components.md` for Form.* API
2. Use `Form.Section` for grouping fields
3. Use `Form.Stack` for field layout
4. Add validation with `error` and `helperText` props

## Critical Rules

- Import form components from `@wso2/oxygen-ui`
- Use controlled components with state
- Show validation errors with `error` and `helperText` props
- Use theme tokens for spacing

## Basic Form

```tsx
import { useState } from 'react';
import { Box, TextField, Button, Alert } from '@wso2/oxygen-ui';

function BasicForm() {
  const [formData, setFormData] = useState({ name: '', email: '' });
  const [errors, setErrors] = useState<Record<string, string>>({});

  const validate = () => {
    const newErrors: Record<string, string> = {};

    if (!formData.name) {
      newErrors.name = 'Name is required';
    }

    if (!formData.email) {
      newErrors.email = 'Email is required';
    } else if (!/\S+@\S+\.\S+/.test(formData.email)) {
      newErrors.email = 'Invalid email format';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (validate()) {
      console.log('Form submitted:', formData);
    }
  };

  return (
    <Box component="form" onSubmit={handleSubmit} sx={{ maxWidth: 400 }}>
      <TextField
        fullWidth
        label="Name"
        value={formData.name}
        onChange={(e) => setFormData({ ...formData, name: e.target.value })}
        error={!!errors.name}
        helperText={errors.name}
        margin="normal"
        required
      />

      <TextField
        fullWidth
        label="Email"
        type="email"
        value={formData.email}
        onChange={(e) => setFormData({ ...formData, email: e.target.value })}
        error={!!errors.email}
        helperText={errors.email}
        margin="normal"
        required
      />

      <Button type="submit" variant="contained" sx={{ mt: 2 }}>
        Submit
      </Button>
    </Box>
  );
}
```

## Form with Sections

```tsx
import { Form, TextField, Button, Box } from '@wso2/oxygen-ui';

function SectionedForm() {
  return (
    <Box sx={{ maxWidth: 600 }}>
      <Form.Section>
        <Form.Header>Personal Information</Form.Header>
        <Form.Subheader>Enter your basic details</Form.Subheader>

        <Form.Stack spacing={2}>
          <Form.TextInput label="First Name" required />
          <Form.TextInput label="Last Name" required />
          <Form.TextInput label="Email" type="email" required />
        </Form.Stack>
      </Form.Section>

      <Form.Section>
        <Form.Header>Company Details</Form.Header>
        <Form.Subheader>Optional information about your organization</Form.Subheader>

        <Form.Stack spacing={2}>
          <Form.TextInput label="Company Name" />
          <Form.SelectInput label="Role">
            <Form.MenuItem value="developer">Developer</Form.MenuItem>
            <Form.MenuItem value="designer">Designer</Form.MenuItem>
            <Form.MenuItem value="manager">Manager</Form.MenuItem>
          </Form.SelectInput>
        </Form.Stack>
      </Form.Section>

      <Box sx={{ mt: 3, display: 'flex', gap: 2 }}>
        <Button variant="outlined">Cancel</Button>
        <Button variant="contained">Save</Button>
      </Box>
    </Box>
  );
}
```

## Login Form

```tsx
import { useState } from 'react';
import {
  Box,
  Paper,
  Typography,
  TextField,
  Button,
  Link,
  Divider,
  Alert,
} from '@wso2/oxygen-ui';

function LoginForm() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    try {
      // Login logic here
      await login(email, password);
    } catch (err) {
      setError('Invalid credentials');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Box
      sx={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        bgcolor: 'background.default',
      }}
    >
      <Paper sx={{ p: 4, maxWidth: 400, width: '100%' }}>
        <Typography variant="h5" gutterBottom align="center">
          Sign In
        </Typography>

        {error && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {error}
          </Alert>
        )}

        <Box component="form" onSubmit={handleSubmit}>
          <TextField
            fullWidth
            label="Email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            margin="normal"
            required
          />

          <TextField
            fullWidth
            label="Password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            margin="normal"
            required
          />

          <Button
            fullWidth
            type="submit"
            variant="contained"
            disabled={loading}
            sx={{ mt: 3, mb: 2 }}
          >
            {loading ? 'Signing in...' : 'Sign In'}
          </Button>

          <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
            <Link href="/forgot-password" variant="body2">
              Forgot password?
            </Link>
            <Link href="/register" variant="body2">
              Create account
            </Link>
          </Box>
        </Box>
      </Paper>
    </Box>
  );
}
```

## Multi-Step Wizard

```tsx
import { useState } from 'react';
import {
  Box,
  Paper,
  Stepper,
  Step,
  StepLabel,
  Button,
  Typography,
  TextField,
} from '@wso2/oxygen-ui';

const steps = ['Account', 'Profile', 'Review'];

function RegistrationWizard() {
  const [activeStep, setActiveStep] = useState(0);
  const [formData, setFormData] = useState({
    email: '',
    password: '',
    name: '',
    company: '',
  });

  const handleNext = () => setActiveStep((prev) => prev + 1);
  const handleBack = () => setActiveStep((prev) => prev - 1);

  const handleSubmit = () => {
    console.log('Form submitted:', formData);
  };

  const renderStepContent = (step: number) => {
    switch (step) {
      case 0:
        return (
          <>
            <TextField
              fullWidth
              label="Email"
              type="email"
              value={formData.email}
              onChange={(e) => setFormData({ ...formData, email: e.target.value })}
              margin="normal"
            />
            <TextField
              fullWidth
              label="Password"
              type="password"
              value={formData.password}
              onChange={(e) => setFormData({ ...formData, password: e.target.value })}
              margin="normal"
            />
          </>
        );
      case 1:
        return (
          <>
            <TextField
              fullWidth
              label="Full Name"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              margin="normal"
            />
            <TextField
              fullWidth
              label="Company"
              value={formData.company}
              onChange={(e) => setFormData({ ...formData, company: e.target.value })}
              margin="normal"
            />
          </>
        );
      case 2:
        return (
          <Box>
            <Typography variant="body1" gutterBottom>
              <strong>Email:</strong> {formData.email}
            </Typography>
            <Typography variant="body1" gutterBottom>
              <strong>Name:</strong> {formData.name}
            </Typography>
            <Typography variant="body1" gutterBottom>
              <strong>Company:</strong> {formData.company}
            </Typography>
          </Box>
        );
      default:
        return null;
    }
  };

  return (
    <Paper sx={{ p: 4, maxWidth: 600, mx: 'auto' }}>
      <Stepper activeStep={activeStep} sx={{ mb: 4 }}>
        {steps.map((label) => (
          <Step key={label}>
            <StepLabel>{label}</StepLabel>
          </Step>
        ))}
      </Stepper>

      <Box sx={{ minHeight: 200 }}>{renderStepContent(activeStep)}</Box>

      <Box sx={{ display: 'flex', justifyContent: 'space-between', mt: 4 }}>
        <Button disabled={activeStep === 0} onClick={handleBack}>
          Back
        </Button>
        {activeStep === steps.length - 1 ? (
          <Button variant="contained" onClick={handleSubmit}>
            Submit
          </Button>
        ) : (
          <Button variant="contained" onClick={handleNext}>
            Next
          </Button>
        )}
      </Box>
    </Paper>
  );
}
```

## Form with Select and Autocomplete

```tsx
import { useState } from 'react';
import { Box, TextField, Button, MenuItem, Autocomplete } from '@wso2/oxygen-ui';

const countries = [
  { label: 'United States', code: 'US' },
  { label: 'United Kingdom', code: 'UK' },
  { label: 'Canada', code: 'CA' },
  // ...more
];

function AddressForm() {
  const [country, setCountry] = useState<typeof countries[0] | null>(null);
  const [state, setState] = useState('');

  return (
    <Box sx={{ maxWidth: 400 }}>
      <Autocomplete
        options={countries}
        value={country}
        onChange={(_, newValue) => setCountry(newValue)}
        renderInput={(params) => (
          <TextField {...params} label="Country" margin="normal" />
        )}
      />

      <TextField
        fullWidth
        select
        label="State"
        value={state}
        onChange={(e) => setState(e.target.value)}
        margin="normal"
      >
        <MenuItem value="ca">California</MenuItem>
        <MenuItem value="ny">New York</MenuItem>
        <MenuItem value="tx">Texas</MenuItem>
      </TextField>

      <TextField fullWidth label="City" margin="normal" />
      <TextField fullWidth label="Address" margin="normal" multiline rows={2} />
      <TextField fullWidth label="Zip Code" margin="normal" />

      <Button variant="contained" sx={{ mt: 2 }}>
        Save Address
      </Button>
    </Box>
  );
}
```
