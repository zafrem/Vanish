import { describe, test, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import CreateMessage from '../../src/components/CreateMessage';
import * as api from '../../src/lib/api';
import { AuthProvider } from '../../src/context/AuthContext';

// Mock the API
vi.mock('../../src/lib/api');

// Mock AuthContext
const mockUser = { id: 'user-1', email: 'test@example.com' };

describe('CreateMessage Component', () => {
  const renderComponent = () => {
    return render(
      <BrowserRouter>
        <AuthProvider>
          <CreateMessage />
        </AuthProvider>
      </BrowserRouter>
    );
  };

  beforeEach(() => {
    // Mock getUsers API call
    vi.spyOn(api, 'getUsers').mockResolvedValue([
      { id: 'user-1', email: 'test@example.com' },
      { id: 'user-2', email: 'other@example.com' }
    ]);

    // Mock fetch for auth
    global.fetch = vi.fn((url) => {
      if (url === '/api/auth/me') {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve(mockUser)
        });
      }
      return Promise.reject(new Error('Unknown URL'));
    });

    // Set token in localStorage
    localStorage.setItem('token', 'fake-token');
  });

  afterEach(() => {
    localStorage.clear();
    vi.clearAllMocks();
  });

  test('should render the form', async () => {
    renderComponent();

    await waitFor(() => {
      expect(screen.getByText(/Create Secret Message/i)).toBeInTheDocument();
    });

    expect(screen.getByPlaceholderText(/Enter password, API key, or sensitive data/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Create Secret Link/i })).toBeInTheDocument();
  });

  test('should not submit empty message', async () => {
    renderComponent();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Create Secret Link/i })).toBeInTheDocument();
    });

    const submitButton = screen.getByRole('button', { name: /Create Secret Link/i });
    expect(submitButton).toBeDisabled();
  });

  test('should generate shareable URL after creation', async () => {
    api.createMessage.mockResolvedValue({
      id: 'test-id-123',
      expiresAt: '2025-12-26T10:00:00Z',
    });

    renderComponent();

    // Wait for component to load users
    await waitFor(() => {
      expect(screen.getByPlaceholderText(/Enter password, API key, or sensitive data/i)).toBeInTheDocument();
    });

    const textarea = screen.getByPlaceholderText(/Enter password, API key, or sensitive data/i);
    const recipientSelect = screen.getByRole('combobox');
    const submitButton = screen.getByRole('button', { name: /Create Secret Link/i });

    fireEvent.change(textarea, { target: { value: 'my secret' } });
    fireEvent.change(recipientSelect, { target: { value: '2' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText(/Secret Created!/i)).toBeInTheDocument();
    }, { timeout: 3000 });

    // Should display a URL
    const urlElement = screen.getByText(/\/m\/test-id-123#/);
    expect(urlElement).toBeInTheDocument();
  });

  test('should not expose plaintext in API call', async () => {
    const mockCreate = vi.fn().mockResolvedValue({
      id: 'test-id',
      expiresAt: '2025-12-26T10:00:00Z',
    });
    vi.spyOn(api, 'createMessage').mockImplementation(mockCreate);

    renderComponent();

    // Wait for component to load
    await waitFor(() => {
      expect(screen.getByPlaceholderText(/Enter password, API key, or sensitive data/i)).toBeInTheDocument();
    });

    const textarea = screen.getByPlaceholderText(/Enter password, API key, or sensitive data/i);
    const recipientSelect = screen.getByRole('combobox');
    fireEvent.change(textarea, { target: { value: 'my secret password' } });
    fireEvent.change(recipientSelect, { target: { value: '2' } });

    const submitButton = screen.getByRole('button', { name: /Create Secret Link/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalled();
    }, { timeout: 3000 });

    // Verify the API was NOT called with plaintext
    const callArgs = mockCreate.mock.calls[0];
    expect(callArgs[0]).not.toBe('my secret password');
    expect(callArgs[0]).toBeDefined(); // Should be ciphertext
  });
});
