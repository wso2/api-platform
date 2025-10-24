import '@testing-library/jest-dom';
import { render, screen, fireEvent } from '@testing-library/react';
import { Box } from './Box';

describe('Box', () => {
  it('should render children correctly', () => {
    render(<Box>Test Content</Box>);
    expect(screen.getByText('Test Content')).toBeInTheDocument();
  });

  it('should apply custom className', () => {
    const { container } = render(<Box className="custom-class">Content</Box>);
    expect(container.firstChild).toHaveClass('custom-class');
  });

  it('should handle click events', () => {
    const handleClick = jest.fn();
    render(<Box onClick={handleClick}>Clickable</Box>);

    fireEvent.click(screen.getByText('Clickable'));
    expect(handleClick).toHaveBeenCalledTimes(1);
  });

  it('should respect disabled state', () => {
    const handleClick = jest.fn();
    render(
      <Box disabled onClick={handleClick}>
        Disabled
      </Box>
    );

    fireEvent.click(screen.getByText('Disabled'));
    expect(handleClick).not.toHaveBeenCalled();
  });
});
