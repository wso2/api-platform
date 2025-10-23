import '@testing-library/jest-dom';
import { render, screen, fireEvent } from '@testing-library/react';
import { NavItemExpandable } from './NavItemExpandable';
import { MenuHomeFilledIcon, MenuHomeIcon } from '@design-system/Icons';

describe('NavItemExpandable', () => {
  const defaultProps = {
    title: 'Test Item',
    id: 'test-item',
    icon: <MenuHomeIcon fontSize='inherit' />,
    selectedIcon: <MenuHomeFilledIcon fontSize='inherit' />,
  };

  it('should render title correctly', () => {
    render(<NavItemExpandable {...defaultProps} isExpanded />);
    expect(screen.getByText('Test Item')).toBeInTheDocument();
  });

  it('should apply custom className', () => {
    const { container } = render(
      <NavItemExpandable {...defaultProps} className="custom-class" />
    );
    expect(container.firstChild).toHaveClass('custom-class');
  });

  it('should handle click events', () => {
    const handleClick = jest.fn();
    render(
      <NavItemExpandable {...defaultProps} onClick={handleClick} />
    );

    fireEvent.click(screen.getByText('Test Item'));
    expect(handleClick).toHaveBeenCalledTimes(1);
    expect(handleClick).toHaveBeenCalledWith('test-item');
  });

  it('should respect disabled state', () => {
    const handleClick = jest.fn();
    render(
      <NavItemExpandable {...defaultProps} disabled onClick={handleClick} />
    );

    fireEvent.click(screen.getByText('Test Item'));
    expect(handleClick).not.toHaveBeenCalled();
  });

  it('should render submenu items when provided', () => {
    const subMenuItems = [
      {
        title: 'Sub Item 1',
        id: 'sub-item-1',
        icon: <MenuHomeIcon fontSize='inherit' />,
        selectedIcon: <MenuHomeFilledIcon fontSize='inherit' />,
      },
    ];

    render(
      <NavItemExpandable {...defaultProps} subMenuItems={subMenuItems} isExpanded />
    );

    // Click to expand submenu
    fireEvent.click(screen.getByText('Test Item'));
    expect(screen.getByText('Sub Item 1')).toBeInTheDocument();
  });
});
