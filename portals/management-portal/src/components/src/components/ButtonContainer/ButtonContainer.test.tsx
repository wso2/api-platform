import '@testing-library/jest-dom';
import { render, screen, fireEvent } from '@testing-library/react';
import { ButtonContainer } from './ButtonContainer';

describe('ButtonContainer', () => {
    it('should render children correctly', () => {
        render(<ButtonContainer>Test Content</ButtonContainer>);
        expect(screen.getByText('Test Content')).toBeInTheDocument();
    });

    it('should apply custom className', () => {
        const { container } = render(
            <ButtonContainer className="custom-class">Content</ButtonContainer>
        );
        expect(container.firstChild).toHaveClass('custom-class');
    });

    it('should handle click events', () => {
        const handleClick = jest.fn();
        render(<ButtonContainer onClick={handleClick}>Clickable</ButtonContainer>);
        
        fireEvent.click(screen.getByText('Clickable'));
        expect(handleClick).toHaveBeenCalledTimes(1);
    });

    it('should respect disabled state', () => {
        const handleClick = jest.fn();
        render(
            <ButtonContainer disabled onClick={handleClick}>
                Disabled
            </ButtonContainer>
        );
        
        fireEvent.click(screen.getByText('Disabled'));
        expect(handleClick).not.toHaveBeenCalled();
    });
});
