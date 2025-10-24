import '@testing-library/jest-dom';
import { render, screen, fireEvent } from '@testing-library/react';
import { IconButton } from './IconButton';

describe('IconButton', () => {
  it('renders children correctly', () => {
    render(<IconButton>Test Content</IconButton>);
    expect(screen.getByText('Test Content')).toBeInTheDocument();
  });

  it('applies custom className', () => {
    const { container } = render(
      <IconButton className="custom-class">Content</IconButton>
    );
    expect(container.firstChild).toHaveClass('custom-class');
  });

  it('handles click events', () => {
    const handleClick = jest.fn();
    render(<IconButton onClick={handleClick}>Clickable</IconButton>);

    fireEvent.click(screen.getByText('Clickable'));
    expect(handleClick).toHaveBeenCalledTimes(1);
  });

  it('respects disabled state', () => {
    const handleClick = jest.fn();
    render(
      <IconButton disabled onClick={handleClick}>
        Disabled
      </IconButton>
    );

    fireEvent.click(screen.getByText('Disabled'));
    expect(handleClick).not.toHaveBeenCalled();
  });
});
