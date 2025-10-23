import '@testing-library/jest-dom';
import { render, screen, fireEvent } from '@testing-library/react';
import { RadioGroup } from './RadioGroup';

describe('RadioGroup', () => {
    it('should render children correctly', () => {
        render(<RadioGroup>Test Content</RadioGroup>);
        expect(screen.getByText('Test Content')).toBeInTheDocument();
    });

    it('should apply custom className', () => {
        const { container } = render(
            <RadioGroup className="custom-class">Content</RadioGroup>
        );
        expect(container.firstChild).toHaveClass('custom-class');
    });

    it('should handle click events', () => {
        const handleClick = jest.fn();
        render(<RadioGroup onClick={handleClick}>Clickable</RadioGroup>);
        
        fireEvent.click(screen.getByText('Clickable'));
        expect(handleClick).toHaveBeenCalledTimes(1);
    });

    it('should respect disabled state', () => {
        const handleClick = jest.fn();
        render(
            <RadioGroup disabled onClick={handleClick}>
                Disabled
            </RadioGroup>
        );
        
        fireEvent.click(screen.getByText('Disabled'));
        expect(handleClick).not.toHaveBeenCalled();
    });
});
