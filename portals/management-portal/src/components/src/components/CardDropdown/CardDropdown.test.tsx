import '@testing-library/jest-dom';
import { render, screen, fireEvent } from '@testing-library/react';
import { CardDropdown } from './CardDropdown';

describe('CardDropdown', () => {
    it('should render children correctly', () => {
        render(<CardDropdown>Test Content</CardDropdown>);
        expect(screen.getByText('Test Content')).toBeInTheDocument();
    });

    it('should apply custom className', () => {
        const { container } = render(
            <CardDropdown className="custom-class">Content</CardDropdown>
        );
        expect(container.firstChild).toHaveClass('custom-class');
    });

    it('should handle click events', () => {
        const handleClick = jest.fn();
        render(<CardDropdown onClick={handleClick}>Clickable</CardDropdown>);
        
        fireEvent.click(screen.getByText('Clickable'));
        expect(handleClick).toHaveBeenCalledTimes(1);
    });

    it('should respect disabled state', () => {
        const handleClick = jest.fn();
        render(
            <CardDropdown disabled onClick={handleClick}>
                Disabled
            </CardDropdown>
        );
        
        fireEvent.click(screen.getByText('Disabled'));
        expect(handleClick).not.toHaveBeenCalled();
    });
});
