import '@testing-library/jest-dom';
import { render, screen, fireEvent } from '@testing-library/react';
import { TextInput } from './TextInput';

describe('TextInput', () => {
  it('renders label and value correctly', () => {
    render(
      <TextInput
        testId="test-input"
        label="Test Label"
        value="Test Value"
        onChange={() => {}}
      />
    );
    expect(screen.getByText('Test Label')).toBeInTheDocument();
    expect(screen.getByDisplayValue('Test Value')).toBeInTheDocument();
  });

  it('applies custom className', () => {
    const { container } = render(
      <TextInput
        testId="test-input"
        label="Test"
        value=""
        className="custom-class"
        onChange={() => {}}
      />
    );
    expect(container.firstChild).toHaveClass('custom-class');
  });

  it('handles value changes', () => {
    const handleChange = jest.fn();
    render(
      <TextInput
        testId="test-input"
        label="Test"
        value=""
        onChange={handleChange}
      />
    );

    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: 'New Value' },
    });
    expect(handleChange).toHaveBeenCalledWith('New Value');
  });

  it('respects disabled state', () => {
    render(
      <TextInput
        testId="test-input"
        label="Test"
        value=""
        onChange={() => {}}
        disabled
      />
    );
    expect(screen.getByRole('textbox')).toBeDisabled();
  });
});
