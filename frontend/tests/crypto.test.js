import { describe, test, expect } from 'vitest';
import { generateKey, exportKey, importKey, encrypt, decrypt } from '../src/lib/crypto';

describe('Crypto Module', () => {
  test('should generate a key', async () => {
    const key = await generateKey();
    expect(key).toBeDefined();
    expect(key.type).toBe('secret');
  });

  test('should export and import a key', async () => {
    const key = await generateKey();
    const exported = await exportKey(key);

    expect(typeof exported).toBe('string');
    expect(exported.length).toBeGreaterThan(0);

    const imported = await importKey(exported);
    expect(imported).toBeDefined();
  });

  test('should encrypt data', async () => {
    const key = await generateKey();
    const plaintext = 'test secret';

    const { ciphertext, iv } = await encrypt(plaintext, key);

    expect(ciphertext).toBeDefined();
    expect(iv).toBeDefined();
    expect(typeof ciphertext).toBe('string');
    expect(typeof iv).toBe('string');
  });

  test('should encrypt and decrypt correctly', async () => {
    const key = await generateKey();
    const plaintext = 'my secret message';

    const { ciphertext, iv } = await encrypt(plaintext, key);
    const decrypted = await decrypt(ciphertext, iv, key);

    expect(decrypted).toBe(plaintext);
  });

  test('should handle special characters', async () => {
    const key = await generateKey();
    const plaintext = 'Special: !@#$%^&*()_+{}|:"<>?[];\',./`~';

    const { ciphertext, iv } = await encrypt(plaintext, key);
    const decrypted = await decrypt(ciphertext, iv, key);

    expect(decrypted).toBe(plaintext);
  });
});
