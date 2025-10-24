import '@testing-library/jest-dom';
import { render, screen, fireEvent } from '@testing-library/react';
import { Link } from './Link';

describe('Link', () => {
    it('should render children correctly', () => {
        render(<Link>Test Content</Link>);
        expect(screen.getByText('Test Content')).toBeInTheDocument();
    });

    it('should apply custom className', () => {
        const { container } = render(
            <Link className="custom-class">Content</Link>
        );
        expect(container.firstChild).toHaveClass('custom-class');
    });

    it('should handle click events', () => {
        const handleClick = jest.fn();
        render(<Link onClick={handleClick}>Clickable</Link>);
        
        fireEvent.click(screen.getByText('Clickable'));
        expect(handleClick).toHaveBeenCalledTimes(1);
    });

    it('should respect disabled state', () => {
        const handleClick = jest.fn();
        render(
            <Link disabled onClick={handleClick}>
                Disabled
            </Link>
        );
        
        fireEvent.click(screen.getByText('Disabled'));
        expect(handleClick).not.toHaveBeenCalled();
    });
});
