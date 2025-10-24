import '@testing-library/jest-dom';
import { render, screen, fireEvent } from '@testing-library/react';
import { TooltipBase } from './TooltipBase';

describe('TooltipBase', () => {
    it('should render children correctly', () => {
        render(<TooltipBase>Test Content</TooltipBase>);
        expect(screen.getByText('Test Content')).toBeInTheDocument();
    });

    it('should apply custom className', () => {
        const { container } = render(
            <TooltipBase className="custom-class">Content</TooltipBase>
        );
        expect(container.firstChild).toHaveClass('custom-class');
    });

    it('should handle click events', () => {
        const handleClick = jest.fn();
        render(<TooltipBase onClick={handleClick}>Clickable</TooltipBase>);
        
        fireEvent.click(screen.getByText('Clickable'));
        expect(handleClick).toHaveBeenCalledTimes(1);
    });

    it('should respect disabled state', () => {
        const handleClick = jest.fn();
        render(
            <TooltipBase disabled onClick={handleClick}>
                Disabled
            </TooltipBase>
        );
        
        fireEvent.click(screen.getByText('Disabled'));
        expect(handleClick).not.toHaveBeenCalled();
    });
});
