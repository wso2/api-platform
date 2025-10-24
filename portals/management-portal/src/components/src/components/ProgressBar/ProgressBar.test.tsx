import '@testing-library/jest-dom';
import { render, screen, fireEvent } from '@testing-library/react';
import { ProgressBar } from './ProgressBar';

describe('ProgressBar', () => {
    it('should render children correctly', () => {
        render(<ProgressBar>Test Content</ProgressBar>);
        expect(screen.getByText('Test Content')).toBeInTheDocument();
    });

    it('should apply custom className', () => {
        const { container } = render(
            <ProgressBar className="custom-class">Content</ProgressBar>
        );
        expect(container.firstChild).toHaveClass('custom-class');
    });

    it('should handle click events', () => {
        const handleClick = jest.fn();
        render(<ProgressBar onClick={handleClick}>Clickable</ProgressBar>);
        
        fireEvent.click(screen.getByText('Clickable'));
        expect(handleClick).toHaveBeenCalledTimes(1);
    });

    it('should respect disabled state', () => {
        const handleClick = jest.fn();
        render(
            <ProgressBar disabled onClick={handleClick}>
                Disabled
            </ProgressBar>
        );
        
        fireEvent.click(screen.getByText('Disabled'));
        expect(handleClick).not.toHaveBeenCalled();
    });
});
