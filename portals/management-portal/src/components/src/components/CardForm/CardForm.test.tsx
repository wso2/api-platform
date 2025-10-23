import '@testing-library/jest-dom';
import { render, screen, fireEvent } from '@testing-library/react';
import { CardForm } from './CardForm';

describe('CardForm', () => {
    it('should render children correctly', () => {
        render(<CardForm>Test Content</CardForm>);
        expect(screen.getByText('Test Content')).toBeInTheDocument();
    });

    it('should apply custom className', () => {
        const { container } = render(
            <CardForm className="custom-class">Content</CardForm>
        );
        expect(container.firstChild).toHaveClass('custom-class');
    });

    it('should handle click events', () => {
        const handleClick = jest.fn();
        render(<CardForm onClick={handleClick}>Clickable</CardForm>);
        
        fireEvent.click(screen.getByText('Clickable'));
        expect(handleClick).toHaveBeenCalledTimes(1);
    });

    it('should respect disabled state', () => {
        const handleClick = jest.fn();
        render(
            <CardForm disabled onClick={handleClick}>
                Disabled
            </CardForm>
        );
        
        fireEvent.click(screen.getByText('Disabled'));
        expect(handleClick).not.toHaveBeenCalled();
    });
});
