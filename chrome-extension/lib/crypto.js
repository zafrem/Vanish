/**
 * Cryptography module using Web Crypto API
 * Implements AES-256-GCM encryption/decryption for client-side security
 */

/**
 * Generate a new AES-256-GCM encryption key
 * @returns {Promise<CryptoKey>} The generated key
 */
export async function generateKey() {
  const key = await crypto.subtle.generateKey(
    { name: 'AES-GCM', length: 256 },
    true, // extractable
    ['encrypt', 'decrypt']
  );
  return key;
}

/**
 * Export a CryptoKey to base64 string for URL transmission
 * @param {CryptoKey} key - The key to export
 * @returns {Promise<string>} Base64-encoded key
 */
export async function exportKey(key) {
  const exported = await crypto.subtle.exportKey('raw', key);
  return arrayBufferToBase64(exported);
}

/**
 * Import a base64 key string back to CryptoKey
 * @param {string} base64Key - Base64-encoded key
 * @returns {Promise<CryptoKey>} The imported key
 */
export async function importKey(base64Key) {
  const buffer = base64ToArrayBuffer(base64Key);
  return await crypto.subtle.importKey(
    'raw',
    buffer,
    { name: 'AES-GCM', length: 256 },
    true,
    ['encrypt', 'decrypt']
  );
}

/**
 * Encrypt plaintext using AES-256-GCM
 * @param {string} plaintext - The text to encrypt
 * @param {CryptoKey} key - The encryption key
 * @returns {Promise<{ciphertext: string, iv: string}>} Encrypted data and IV
 */
export async function encrypt(plaintext, key) {
  const encoder = new TextEncoder();
  const data = encoder.encode(plaintext);

  // Generate random 96-bit IV (recommended for GCM)
  const iv = crypto.getRandomValues(new Uint8Array(12));

  const ciphertext = await crypto.subtle.encrypt(
    { name: 'AES-GCM', iv },
    key,
    data
  );

  return {
    ciphertext: arrayBufferToBase64(ciphertext),
    iv: arrayBufferToBase64(iv)
  };
}

/**
 * Decrypt ciphertext using AES-256-GCM
 * @param {string} ciphertextBase64 - Base64-encoded ciphertext
 * @param {string} ivBase64 - Base64-encoded IV
 * @param {CryptoKey} key - The decryption key
 * @returns {Promise<string>} Decrypted plaintext
 */
export async function decrypt(ciphertextBase64, ivBase64, key) {
  const ciphertext = base64ToArrayBuffer(ciphertextBase64);
  const iv = base64ToArrayBuffer(ivBase64);

  try {
    const decrypted = await crypto.subtle.decrypt(
      { name: 'AES-GCM', iv },
      key,
      ciphertext
    );

    const decoder = new TextDecoder();
    return decoder.decode(decrypted);
  } catch (error) {
    throw new Error('Decryption failed. The key may be incorrect or the message corrupted.');
  }
}

/**
 * Convert ArrayBuffer to base64 string
 * @param {ArrayBuffer} buffer
 * @returns {string} Base64-encoded string
 */
function arrayBufferToBase64(buffer) {
  const bytes = new Uint8Array(buffer);
  let binary = '';
  for (let i = 0; i < bytes.length; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}

/**
 * Convert base64 string to ArrayBuffer
 * @param {string} base64
 * @returns {ArrayBuffer}
 */
function base64ToArrayBuffer(base64) {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes.buffer;
}
