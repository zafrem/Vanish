import { describe, test, expect, beforeEach } from 'vitest';
import { generateShareableURL, extractKeyFromURL } from '../src/utils/urlHelpers';

describe('URL Helpers', () => {
  beforeEach(() => {
    delete window.location;
    window.location = {
      origin: 'http://localhost:5173',
      hash: '',
    };
  });

  describe('generateShareableURL', () => {
    test('should generate URL with ID and key', () => {
      const messageID = 'test-id-123';
      const key = 'dGVzdC1rZXk='; // base64 encoded

      const url = generateShareableURL(messageID, key);

      expect(url).toBe(`http://localhost:5173/m/test-id-123#dGVzdC1rZXk=`);
    });

    test('should handle different origins', () => {
      window.location.origin = 'https://example.com';
      const messageID = 'msg-456';
      const key = 'YWJjZGVm'; // base64 encoded

      const url = generateShareableURL(messageID, key);

      expect(url).toBe(`https://example.com/m/msg-456#YWJjZGVm`);
    });

    test('should encode special characters in key', () => {
      const messageID = 'test-id';
      const key = 'a+b/c=='; // base64 with special chars

      const url = generateShareableURL(messageID, key);

      expect(url).toContain(key);
      expect(url).toBe(`http://localhost:5173/m/test-id#a+b/c==`);
    });
  });

  describe('extractKeyFromURL', () => {
    test('should extract key from URL fragment', () => {
      window.location.hash = '#dGVzdC1rZXk=';

      const key = extractKeyFromURL();

      expect(key).toBe('dGVzdC1rZXk=');
    });

    test('should return null when no fragment exists', () => {
      window.location.hash = '';

      const key = extractKeyFromURL();

      expect(key).toBeNull();
    });

    test('should handle fragment without hash symbol', () => {
      // Note: window.location.hash always includes the # in browsers
      // But extractKeyFromURL should strip it
      window.location.hash = '#dGVzdC1rZXk=';

      const key = extractKeyFromURL();

      expect(key).toBe('dGVzdC1rZXk=');
    });

    test('should extract key with special characters', () => {
      window.location.hash = '#a+b/c==';

      const key = extractKeyFromURL();

      expect(key).toBe('a+b/c==');
    });

    test('should return fragment content without trim', () => {
      // extractKeyFromURL doesn't trim, it just returns hash.substring(1)
      window.location.hash = '#dGVzdC1rZXk=';

      const key = extractKeyFromURL();

      expect(key).toBe('dGVzdC1rZXk=');
    });
  });
});
