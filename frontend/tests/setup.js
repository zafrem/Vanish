import { expect, afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';
import '@testing-library/jest-dom/vitest';

// Cleanup after each test
afterEach(() => {
  cleanup();
});

// Mock Web Crypto API for tests
Object.defineProperty(global, 'crypto', {
  value: {
    subtle: {
      generateKey: async () => ({ type: 'secret' }),
      exportKey: async () => new ArrayBuffer(32),
      importKey: async () => ({ type: 'secret' }),
      encrypt: async () => new ArrayBuffer(16),
      decrypt: async () => new TextEncoder().encode('decrypted'),
    },
    getRandomValues: (arr) => {
      for (let i = 0; i < arr.length; i++) {
        arr[i] = Math.floor(Math.random() * 256);
      }
      return arr;
    },
  },
  writable: true,
  configurable: true,
});

// Mock clipboard API
Object.assign(navigator, {
  clipboard: {
    writeText: async (text) => Promise.resolve(),
  },
});
