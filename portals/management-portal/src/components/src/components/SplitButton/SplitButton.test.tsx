import '@testing-library/jest-dom';
import { render, screen, fireEvent } from '@testing-library/react';
import { SplitButton } from './SplitButton';

describe('SplitButton', () => {
    it('should render children correctly', () => {
        render(<SplitButton>Test Content</SplitButton>);
        expect(screen.getByText('Test Content')).toBeInTheDocument();
    });

    it('should apply custom className', () => {
        const { container } = render(
            <SplitButton className="custom-class">Content</SplitButton>
        );
        expect(container.firstChild).toHaveClass('custom-class');
    });

    it('should handle click events', () => {
        const handleClick = jest.fn();
        render(<SplitButton onClick={handleClick}>Clickable</SplitButton>);
        
        fireEvent.click(screen.getByText('Clickable'));
        expect(handleClick).toHaveBeenCalledTimes(1);
    });

    it('should respect disabled state', () => {
        const handleClick = jest.fn();
        render(
            <SplitButton disabled onClick={handleClick}>
                Disabled
            </SplitButton>
        );
        
        fireEvent.click(screen.getByText('Disabled'));
        expect(handleClick).not.toHaveBeenCalled();
    });
});
