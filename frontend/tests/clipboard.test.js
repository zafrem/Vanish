import { describe, test, expect, vi } from 'vitest';
import { copyToClipboard } from '../src/lib/clipboard';

describe('Clipboard Module', () => {
  test('should copy to clipboard using Clipboard API', async () => {
    const mockWriteText = vi.fn().mockResolvedValue();
    Object.assign(navigator, {
      clipboard: {
        writeText: mockWriteText,
      },
    });

    const result = await copyToClipboard('test secret');

    expect(result.success).toBe(true);
    expect(mockWriteText).toHaveBeenCalledWith('test secret');
  });

  test('should handle clipboard API failure', async () => {
    const mockWriteText = vi.fn().mockRejectedValue(new Error('Permission denied'));
    Object.assign(navigator, {
      clipboard: {
        writeText: mockWriteText,
      },
    });

    // Mock execCommand fallback
    document.execCommand = vi.fn().mockReturnValue(true);

    const result = await copyToClipboard('test secret');

    // Should fall back to execCommand
    expect(document.execCommand).toHaveBeenCalledWith('copy');
  });
});
