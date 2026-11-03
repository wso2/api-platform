import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TruncatedText } from './TruncatedText';

beforeEach(() => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      disconnect() {}
    },
  );
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe('TruncatedText', () => {
  it('keeps text at the character limit unchanged without a tooltip', () => {
    render(<TruncatedText text="Hello" maxCharacters={5} />);
    expect(screen.getByText('Hello').getAttribute('title')).toBe('');
    expect(screen.getByText('Hello').getAttribute('tabindex')).toBeNull();
  });

  it('truncates Unicode text and exposes the full value to the tooltip', () => {
    render(<TruncatedText text="Hi😀there" maxCharacters={3} />);
    const text = screen.getByText('Hi😀…');
    expect(text.getAttribute('title')).toBe('Hi😀there');
    expect(text.getAttribute('tabindex')).toBe('0');
  });

  it('does not cut a combining accent or a joined emoji in half', () => {
    // "e" + combining acute is one visible character, as is a family emoji.
    const accented = 'e\u0301e\u0301e\u0301e\u0301';
    render(<TruncatedText text={accented} maxCharacters={2} />);
    expect(screen.getByText('e\u0301e\u0301…')).toBeInTheDocument();

    const family = '\u{1F468}\u200D\u{1F469}\u200D\u{1F467}\u{1F468}\u200D\u{1F469}\u200D\u{1F467}';
    render(<TruncatedText text={family} maxCharacters={1} />);
    expect(screen.getByText('\u{1F468}\u200D\u{1F469}\u200D\u{1F467}…')).toBeInTheDocument();
  });

  it('enables the tooltip when the rendered text exceeds the line limit', () => {
    vi.spyOn(HTMLElement.prototype, 'scrollHeight', 'get').mockReturnValue(60);
    vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(40);
    render(<TruncatedText text="A long description" maxLines={2} />);
    expect(screen.getByText('A long description').getAttribute('title')).toBe('A long description');
  });
});
