/**
 * Browser-API stubs that jsdom does not implement but Oxygen UI / MUI rely on.
 * Imported for its side effects from `src/test/setup.ts`.
 */
import { vi } from 'vitest';

// MUI `useMediaQuery` + responsive theme read matchMedia.
if (!window.matchMedia) {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(), // deprecated, kept for older consumers
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }),
  });
}

// MUI menus/tooltips/popovers + Oxygen layout observe element size.
class ResizeObserverStub {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
}

// Lazy lists / sidebar visibility.
class IntersectionObserverStub {
  readonly root = null;
  readonly rootMargin = '';
  readonly thresholds: ReadonlyArray<number> = [];
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
  takeRecords = vi.fn(() => []);
}

vi.stubGlobal('ResizeObserver', ResizeObserverStub);
vi.stubGlobal('IntersectionObserver', IntersectionObserverStub);

// @asgardeo/auth-react references Worker at import time (web-worker storage),
// which jsdom does not provide. A no-op stub lets the module load in tests;
// component tests inject auth via AuthStateContext so the worker never runs.
class WorkerStub {
  onmessage: ((e: unknown) => void) | null = null;
  onerror: ((e: unknown) => void) | null = null;
  postMessage = vi.fn();
  terminate = vi.fn();
  addEventListener = vi.fn();
  removeEventListener = vi.fn();
  dispatchEvent = vi.fn(() => true);
}
vi.stubGlobal('Worker', WorkerStub);

// Node 25 ships an experimental global `localStorage` (file-backed) that
// shadows jsdom's and throws without --localstorage-file. Replace both Web
// Storage globals with a simple in-memory implementation for deterministic tests.
class MemoryStorage {
  private store = new Map<string, string>();
  get length() {
    return this.store.size;
  }
  clear() {
    this.store.clear();
  }
  getItem(key: string) {
    return this.store.has(key) ? this.store.get(key)! : null;
  }
  key(index: number) {
    return Array.from(this.store.keys())[index] ?? null;
  }
  removeItem(key: string) {
    this.store.delete(key);
  }
  setItem(key: string, value: string) {
    this.store.set(key, String(value));
  }
}
vi.stubGlobal('localStorage', new MemoryStorage());
vi.stubGlobal('sessionStorage', new MemoryStorage());

// Dialogs / wizards / scroll-into-view paths.
if (!window.scrollTo) {
  window.scrollTo = vi.fn() as typeof window.scrollTo;
}
if (!Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = vi.fn();
}
