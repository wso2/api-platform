import '@testing-library/jest-dom';
import { render, screen, fireEvent } from '@testing-library/react';
import { SimpleSelect } from './SimpleSelect';

describe('SimpleSelect', () => {
    it('should render children correctly', () => {
        render(<SimpleSelect>Test Content</SimpleSelect>);
        expect(screen.getByText('Test Content')).toBeInTheDocument();
    });

    it('should apply custom className', () => {
        const { container } = render(
            <SimpleSelect className="custom-class">Content</SimpleSelect>
        );
        expect(container.firstChild).toHaveClass('custom-class');
    });

    it('should handle click events', () => {
        const handleClick = jest.fn();
        render(<SimpleSelect onClick={handleClick}>Clickable</SimpleSelect>);
        
        fireEvent.click(screen.getByText('Clickable'));
        expect(handleClick).toHaveBeenCalledTimes(1);
    });

    it('should respect disabled state', () => {
        const handleClick = jest.fn();
        render(
            <SimpleSelect disabled onClick={handleClick}>
                Disabled
            </SimpleSelect>
        );
        
        fireEvent.click(screen.getByText('Disabled'));
        expect(handleClick).not.toHaveBeenCalled();
    });
});
