import '@testing-library/jest-dom';
import { render, screen, fireEvent } from '@testing-library/react';
import { Typography } from './Typography';

describe('Typography', () => {
    it('should render children correctly', () => {
        render(<Typography>Test Content</Typography>);
        expect(screen.getByText('Test Content')).toBeInTheDocument();
    });

    it('should apply custom className', () => {
        const { container } = render(
            <Typography className="custom-class">Content</Typography>
        );
        expect(container.firstChild).toHaveClass('custom-class');
    });

    it('should handle click events', () => {
        const handleClick = jest.fn();
        render(<Typography onClick={handleClick}>Clickable</Typography>);
        
        fireEvent.click(screen.getByText('Clickable'));
        expect(handleClick).toHaveBeenCalledTimes(1);
    });

    it('should respect disabled state', () => {
        const handleClick = jest.fn();
        render(
            <Typography disabled onClick={handleClick}>
                Disabled
            </Typography>
        );
        
        fireEvent.click(screen.getByText('Disabled'));
        expect(handleClick).not.toHaveBeenCalled();
    });
});
