import '@testing-library/jest-dom';
import { render, screen, fireEvent } from '@testing-library/react';
import { NoDataMessage } from './NoDataMessage';

describe('NoDataMessage', () => {
    it('should render children correctly', () => {
        render(<NoDataMessage>Test Content</NoDataMessage>);
        expect(screen.getByText('Test Content')).toBeInTheDocument();
    });

    it('should apply custom className', () => {
        const { container } = render(
            <NoDataMessage className="custom-class">Content</NoDataMessage>
        );
        expect(container.firstChild).toHaveClass('custom-class');
    });

    it('should handle click events', () => {
        const handleClick = jest.fn();
        render(<NoDataMessage onClick={handleClick}>Clickable</NoDataMessage>);
        
        fireEvent.click(screen.getByText('Clickable'));
        expect(handleClick).toHaveBeenCalledTimes(1);
    });

    it('should respect disabled state', () => {
        const handleClick = jest.fn();
        render(
            <NoDataMessage disabled onClick={handleClick}>
                Disabled
            </NoDataMessage>
        );
        
        fireEvent.click(screen.getByText('Disabled'));
        expect(handleClick).not.toHaveBeenCalled();
    });
});
