import { describe, test, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import RetrieveMessage from '../../src/components/RetrieveMessage';
import * as api from '../../src/lib/api';
import * as cryptoLib from '../../src/lib/crypto';
import * as downloadLib from '../../src/lib/download';
import * as clipboardLib from '../../src/lib/clipboard';
import { packEnvelope } from '../../src/lib/fileEnvelope';

vi.mock('../../src/lib/api');

describe('RetrieveMessage Component', () => {
  beforeEach(() => {
    // Mock URL fragment with encryption key
    Object.defineProperty(window, 'location', {
      writable: true,
      value: {
        pathname: '/m/test-id',
        hash: '#dGVzdC1rZXk=', // base64 test key
        origin: 'http://localhost',
        href: 'http://localhost/m/test-id#dGVzdC1rZXk=',
      },
    });
  });

  const renderComponent = () => {
    window.history.pushState({}, 'Test page', '/m/test-id#dGVzdC1rZXk=');
    return render(
      <BrowserRouter>
        <Routes>
          <Route path="/m/:id" element={<RetrieveMessage />} />
        </Routes>
      </BrowserRouter>
    );
  };

  test('should check if message exists on load', async () => {
    api.checkMessageExists.mockResolvedValue(true);

    renderComponent();

    await waitFor(() => {
      expect(api.checkMessageExists).toHaveBeenCalledWith('test-id');
    });
  });

  test('should show error if message not found', async () => {
    api.checkMessageExists.mockResolvedValue(false);

    renderComponent();

    await waitFor(() => {
      expect(screen.getByText(/Message Not Found/i)).toBeInTheDocument();
    });
  });

  test('should never render secret to DOM', async () => {
    api.checkMessageExists.mockResolvedValue(true);
    api.getMessage.mockResolvedValue({
      ciphertext: 'encrypted-data',
      iv: 'test-iv',
    });

    const { container } = renderComponent();

    await waitFor(() => {
      expect(screen.getByText(/Secret Awaits/i)).toBeInTheDocument();
    });

    const copyButton = screen.getByRole('button', { name: /Reveal & Destroy/i });
    fireEvent.click(copyButton);

    await waitFor(() => {
      expect(screen.getByText(/Message Burned!/i)).toBeInTheDocument();
    });

    // Verify 'decrypted' (the mock decrypted value) never appears in DOM
    expect(container.textContent).not.toContain('decrypted');
  });

  test('should show success after burning', async () => {
    api.checkMessageExists.mockResolvedValue(true);
    api.getMessage.mockResolvedValue({
      ciphertext: 'encrypted-data',
      iv: 'test-iv',
    });

    renderComponent();

    await waitFor(() => {
      const copyButton = screen.getByRole('button', { name: /Reveal & Destroy/i });
      fireEvent.click(copyButton);
    });

    await waitFor(() => {
      expect(screen.getByText(/Message Burned!/i)).toBeInTheDocument();
      expect(screen.getByText(/Secret copied to your clipboard/i)).toBeInTheDocument();
    });
  });

  describe('after reveal', () => {
    const reveal = async () => {
      api.checkMessageExists.mockResolvedValue(true);
      api.getMessage.mockResolvedValue({ ciphertext: 'ct', iv: 'iv' });
      vi.spyOn(cryptoLib, 'importKey').mockResolvedValue({});
      renderComponent();
      const button = await screen.findByRole('button', { name: /Destroy Message/i });
      fireEvent.click(button);
    };

    test('downloads a file with a sanitized name and does not touch the clipboard', async () => {
      const bytes = packEnvelope('../../evil.txt', 'text/plain', new TextEncoder().encode('FILE-CONTENT'));
      vi.spyOn(cryptoLib, 'decryptBytes').mockResolvedValue(bytes);
      const save = vi.spyOn(downloadLib, 'saveBlob').mockReturnValue(() => {});
      const copy = vi.spyOn(clipboardLib, 'copyToClipboard');

      await reveal();

      await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
      expect(save.mock.calls[0][1]).toBe('evil.txt');
      // The sender-controlled MIME type must never reach the Blob.
      expect(save.mock.calls[0]).toHaveLength(2);
      expect(copy).not.toHaveBeenCalled();
      expect(await screen.findByText(/evil\.txt/)).toBeInTheDocument();
      expect(screen.queryByText(/FILE-CONTENT/)).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: /save again/i })).toBeInTheDocument();
    });

    test('copies text to the clipboard and never renders it', async () => {
      vi.spyOn(cryptoLib, 'decryptBytes').mockResolvedValue(new TextEncoder().encode('TOP-SECRET'));
      const save = vi.spyOn(downloadLib, 'saveBlob');
      const copy = vi.spyOn(clipboardLib, 'copyToClipboard').mockResolvedValue({ success: true });

      await reveal();

      await waitFor(() => expect(copy).toHaveBeenCalledWith('TOP-SECRET'));
      expect(save).not.toHaveBeenCalled();
      expect(screen.queryByText(/TOP-SECRET/)).not.toBeInTheDocument();
      expect(await screen.findByText(/copied to your clipboard/i)).toBeInTheDocument();
    });

    test('a corrupt file shows an error instead of a download', async () => {
      const bad = new TextEncoder().encode('VNSHFILE1\n{"name":"a","type":"b","size":9}\nabc');
      vi.spyOn(cryptoLib, 'decryptBytes').mockResolvedValue(bad);
      const save = vi.spyOn(downloadLib, 'saveBlob');

      await reveal();

      expect(await screen.findByText(/Corrupt file/i)).toBeInTheDocument();
      expect(save).not.toHaveBeenCalled();
    });
  });
});
