import '@testing-library/jest-dom';
import { render, screen, fireEvent } from '@testing-library/react';
import { TopLevelSelector, Level, type LevelItem } from './TopLevelSelector';

describe('TopLevelSelector', () => {
    const sampleItems: LevelItem[] = [
        { label: 'Item 1', id: '1' },
        { label: 'Item 2', id: '2' },
        { label: 'Item 3', id: '3' },
    ];
    const selectedItem = sampleItems[0];
    const recentItems = sampleItems;
    const level = Level.ORGANIZATION;
    const onSelect = jest.fn();

    it('should render the selected item label', () => {
        render(
            <TopLevelSelector
                items={sampleItems}
                recentItems={recentItems}
                selectedItem={selectedItem}
                level={level}
                onSelect={onSelect}
            />
        );
        expect(screen.getByText('Item 1')).toBeInTheDocument();
    });

    it('should apply custom className', () => {
        const { container } = render(
            <TopLevelSelector
                items={sampleItems}
                recentItems={recentItems}
                selectedItem={selectedItem}
                level={level}
                onSelect={onSelect}
                className="custom-class"
            />
        );
        expect(container.firstChild).toHaveClass('custom-class');
    });

    it('should handle click events', () => {
        const handleClick = jest.fn();
        render(
            <TopLevelSelector
                items={sampleItems}
                recentItems={recentItems}
                selectedItem={selectedItem}
                level={level}
                onSelect={onSelect}
                onClick={handleClick}
            />
        );
        fireEvent.click(screen.getByText('Organization'));
        expect(handleClick).toHaveBeenCalledTimes(1);
    });

    it('should respect disabled state', () => {
        const handleClick = jest.fn();
        render(
            <TopLevelSelector
                items={sampleItems}
                recentItems={recentItems}
                selectedItem={selectedItem}
                level={level}
                onSelect={onSelect}
                onClick={handleClick}
                disabled
            />
        );
        fireEvent.click(screen.getByText('Organization'));
        expect(handleClick).not.toHaveBeenCalled();
    });
});
