import { describe, test, expect, vi, beforeEach, afterEach } from 'vitest';
import { saveBlob } from '../src/lib/download';

describe('saveBlob', () => {
  let click;
  beforeEach(() => {
    vi.useFakeTimers();
    URL.createObjectURL = vi.fn(() => 'blob:mock');
    URL.revokeObjectURL = vi.fn();
    click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function () {
      this.__clicked = { href: this.href, download: this.download };
    });
  });
  afterEach(() => {
    vi.useRealTimers();
    click.mockRestore();
  });

  test('triggers a download with the given name and revokes the URL', () => {
    saveBlob(new Uint8Array([1, 2, 3]), 'a.bin', 'application/octet-stream');
    expect(URL.createObjectURL).toHaveBeenCalledTimes(1);
    expect(click).toHaveBeenCalledTimes(1);
    expect(click.mock.instances[0].__clicked.download).toBe('a.bin');
    vi.advanceTimersByTime(1500);
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:mock');
    expect(document.querySelectorAll('a[download]').length).toBe(0);
  });

  test('returned function saves again', () => {
    const again = saveBlob(new Uint8Array([1]), 'a.bin');
    again();
    expect(click).toHaveBeenCalledTimes(2);
    expect(URL.createObjectURL).toHaveBeenCalledTimes(2);
  });
});
