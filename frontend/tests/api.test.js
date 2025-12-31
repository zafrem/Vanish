import { describe, test, expect, vi, beforeEach } from 'vitest';
import { createMessage, getMessage, checkMessageExists, getUsers } from '../src/lib/api';

describe('API Functions', () => {
  beforeEach(() => {
    global.fetch = vi.fn();
    localStorage.clear();
  });

  describe('createMessage', () => {
    test('should create a message successfully', async () => {
      const mockResponse = {
        id: 'test-id-123',
        expiresAt: '2025-12-26T10:00:00Z',
      };

      global.fetch = vi.fn(() =>
        Promise.resolve({
          ok: true,
          json: () => Promise.resolve(mockResponse),
        })
      );

      localStorage.setItem('token', 'test-token');

      const result = await createMessage('ciphertext', 'iv', 2, 86400);

      expect(result).toEqual(mockResponse);
      expect(global.fetch).toHaveBeenCalledWith(
        '/api/messages',
        expect.objectContaining({
          method: 'POST',
          headers: expect.objectContaining({
            'Content-Type': 'application/json',
            'Authorization': 'Bearer test-token',
          }),
        })
      );
    });

    test('should throw error on failed request', async () => {
      global.fetch = vi.fn(() =>
        Promise.resolve({
          ok: false,
          status: 400,
          json: () => Promise.resolve({ error: 'Bad request' }),
        })
      );

      localStorage.setItem('token', 'test-token');

      await expect(createMessage('ciphertext', 'iv', 2, 86400)).rejects.toThrow('Bad request');
    });
  });

  describe('getMessage', () => {
    test('should get a message successfully', async () => {
      const mockResponse = {
        ciphertext: 'test-ciphertext',
        iv: 'test-iv',
      };

      global.fetch = vi.fn(() =>
        Promise.resolve({
          ok: true,
          json: () => Promise.resolve(mockResponse),
        })
      );

      localStorage.setItem('token', 'test-token');

      const result = await getMessage('test-id');

      expect(result).toEqual(mockResponse);
      expect(global.fetch).toHaveBeenCalledWith(
        '/api/messages/test-id',
        expect.objectContaining({
          headers: expect.objectContaining({
            'Authorization': 'Bearer test-token',
          }),
        })
      );
    });

    test('should throw error when message not found', async () => {
      global.fetch = vi.fn(() =>
        Promise.resolve({
          ok: false,
          status: 404,
          json: () => Promise.resolve({ error: 'Message not found' }),
        })
      );

      localStorage.setItem('token', 'test-token');

      await expect(getMessage('nonexistent-id')).rejects.toThrow('Message not found');
    });
  });

  describe('checkMessageExists', () => {
    test('should return true when message exists', async () => {
      global.fetch = vi.fn(() =>
        Promise.resolve({
          ok: true,
        })
      );

      localStorage.setItem('token', 'test-token');

      const result = await checkMessageExists('test-id');

      expect(result).toBe(true);
      expect(global.fetch).toHaveBeenCalledWith(
        '/api/messages/test-id',
        expect.objectContaining({
          method: 'HEAD',
        })
      );
    });

    test('should return false when message does not exist', async () => {
      global.fetch = vi.fn(() =>
        Promise.resolve({
          ok: false,
        })
      );

      localStorage.setItem('token', 'test-token');

      const result = await checkMessageExists('nonexistent-id');

      expect(result).toBe(false);
    });
  });

  describe('getUsers', () => {
    test('should get users successfully', async () => {
      const mockUsers = [
        { id: 1, email: 'user1@example.com', name: 'User 1' },
        { id: 2, email: 'user2@example.com', name: 'User 2' },
      ];

      global.fetch = vi.fn(() =>
        Promise.resolve({
          ok: true,
          json: () => Promise.resolve(mockUsers),
        })
      );

      localStorage.setItem('token', 'test-token');

      const result = await getUsers();

      expect(result).toEqual(mockUsers);
      expect(global.fetch).toHaveBeenCalledWith(
        '/api/users',
        expect.objectContaining({
          headers: expect.objectContaining({
            'Authorization': 'Bearer test-token',
          }),
        })
      );
    });

    test('should throw error on failed request', async () => {
      global.fetch = vi.fn(() =>
        Promise.resolve({
          ok: false,
          status: 500,
          json: () => Promise.resolve({ error: 'Server error' }),
        })
      );

      localStorage.setItem('token', 'test-token');

      await expect(getUsers()).rejects.toThrow('Failed to fetch users');
    });
  });
});
