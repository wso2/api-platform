import '@testing-library/jest-dom';
import { render, screen, fireEvent } from '@testing-library/react';
import { Toggler } from './Toggler';

describe('Toggler', () => {
    it('should render children correctly', () => {
        render(<Toggler>Test Content</Toggler>);
        expect(screen.getByText('Test Content')).toBeInTheDocument();
    });

    it('should apply custom className', () => {
        const { container } = render(
            <Toggler className="custom-class">Content</Toggler>
        );
        expect(container.firstChild).toHaveClass('custom-class');
    });

    it('should handle click events', () => {
        const handleClick = jest.fn();
        render(<Toggler onClick={handleClick}>Clickable</Toggler>);
        
        fireEvent.click(screen.getByText('Clickable'));
        expect(handleClick).toHaveBeenCalledTimes(1);
    });

    it('should respect disabled state', () => {
        const handleClick = jest.fn();
        render(
            <Toggler disabled onClick={handleClick}>
                Disabled
            </Toggler>
        );
        
        fireEvent.click(screen.getByText('Disabled'));
        expect(handleClick).not.toHaveBeenCalled();
    });
});
