import { describe, test, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import RetrieveMessage from '../../src/components/RetrieveMessage';
import * as api from '../../src/lib/api';

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
      expect(screen.getByText(/Secret Message Awaits/i)).toBeInTheDocument();
    });

    const copyButton = screen.getByRole('button', { name: /Copy to Clipboard & Destroy/i });
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
      const copyButton = screen.getByRole('button', { name: /Copy to Clipboard & Destroy/i });
      fireEvent.click(copyButton);
    });

    await waitFor(() => {
      expect(screen.getByText(/Message Burned!/i)).toBeInTheDocument();
      expect(screen.getByText(/Secret copied to your clipboard/i)).toBeInTheDocument();
    });
  });
});
