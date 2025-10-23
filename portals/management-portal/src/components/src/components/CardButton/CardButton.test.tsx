import '@testing-library/jest-dom';
import { render, screen, fireEvent } from '@testing-library/react';
import { CardButton } from './CardButton';

describe('CardButton', () => {
    it('should render children correctly', () => {
        render(<CardButton>Test Content</CardButton>);
        expect(screen.getByText('Test Content')).toBeInTheDocument();
    });

    it('should apply custom className', () => {
        const { container } = render(
            <CardButton className="custom-class">Content</CardButton>
        );
        expect(container.firstChild).toHaveClass('custom-class');
    });

    it('should handle click events', () => {
        const handleClick = jest.fn();
        render(<CardButton onClick={handleClick}>Clickable</CardButton>);
        
        fireEvent.click(screen.getByText('Clickable'));
        expect(handleClick).toHaveBeenCalledTimes(1);
    });

    it('should respect disabled state', () => {
        const handleClick = jest.fn();
        render(
            <CardButton disabled onClick={handleClick}>
                Disabled
            </CardButton>
        );
        
        fireEvent.click(screen.getByText('Disabled'));
        expect(handleClick).not.toHaveBeenCalled();
    });
});
