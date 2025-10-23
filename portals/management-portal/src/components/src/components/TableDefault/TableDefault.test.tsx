import '@testing-library/jest-dom';
import { render, screen, fireEvent } from '@testing-library/react';
import { TableDefault } from './TableDefault';

describe('TableDefault', () => {
    it('should render children correctly', () => {
        render(<TableDefault>Test Content</TableDefault>);
        expect(screen.getByText('Test Content')).toBeInTheDocument();
    });

    it('should apply custom className', () => {
        const { container } = render(
            <TableDefault className="custom-class">Content</TableDefault>
        );
        expect(container.firstChild).toHaveClass('custom-class');
    });

    it('should handle click events', () => {
        const handleClick = jest.fn();
        render(<TableDefault onClick={handleClick}>Clickable</TableDefault>);
        
        fireEvent.click(screen.getByText('Clickable'));
        expect(handleClick).toHaveBeenCalledTimes(1);
    });

    it('should respect disabled state', () => {
        const handleClick = jest.fn();
        render(
            <TableDefault disabled onClick={handleClick}>
                Disabled
            </TableDefault>
        );
        
        fireEvent.click(screen.getByText('Disabled'));
        expect(handleClick).not.toHaveBeenCalled();
    });
});
