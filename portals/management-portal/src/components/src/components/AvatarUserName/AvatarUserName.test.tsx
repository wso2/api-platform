import '@testing-library/jest-dom';
import { render, screen, fireEvent } from '@testing-library/react';
import { AvatarUserName } from './AvatarUserName';

describe('AvatarUserName', () => {
    it('should render children correctly', () => {
        render(<AvatarUserName>Test Content</AvatarUserName>);
        expect(screen.getByText('Test Content')).toBeInTheDocument();
    });

    it('should apply custom className', () => {
        const { container } = render(
            <AvatarUserName className="custom-class">Content</AvatarUserName>
        );
        expect(container.firstChild).toHaveClass('custom-class');
    });

    it('should handle click events', () => {
        const handleClick = jest.fn();
        render(<AvatarUserName onClick={handleClick}>Clickable</AvatarUserName>);
        
        fireEvent.click(screen.getByText('Clickable'));
        expect(handleClick).toHaveBeenCalledTimes(1);
    });

    it('should respect disabled state', () => {
        const handleClick = jest.fn();
        render(
            <AvatarUserName disabled onClick={handleClick}>
                Disabled
            </AvatarUserName>
        );
        
        fireEvent.click(screen.getByText('Disabled'));
        expect(handleClick).not.toHaveBeenCalled();
    });
});
