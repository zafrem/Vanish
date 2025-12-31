/**
 * URL helper functions for generating and parsing shareable links
 * Critical: Encryption key MUST stay in URL fragment (#) to prevent server transmission
 */

/**
 * Generate a shareable URL with the encryption key in the fragment
 * @param {string} messageId - The message ID from the server
 * @param {string} encryptionKey - Base64-encoded encryption key
 * @returns {string} Complete shareable URL
 */
export function generateShareableURL(messageId, encryptionKey) {
  const baseURL = window.location.origin;
  const path = `/m/${messageId}`;

  // CRITICAL: Key goes in fragment (#) so it's never sent to server
  return `${baseURL}${path}#${encryptionKey}`;
}

/**
 * Extract the encryption key from the URL fragment
 * @returns {string|null} The encryption key or null if not present
 */
export function extractKeyFromURL() {
  // window.location.hash includes the '#'
  const hash = window.location.hash;

  if (!hash || hash.length <= 1) {
    return null;
  }

  // Remove the '#' prefix
  return hash.substring(1);
}

/**
 * Extract the message ID from the URL path
 * @returns {string|null} The message ID or null if not found
 */
export function extractMessageIdFromPath() {
  const pathname = window.location.pathname;

  // Match pattern /m/:id
  const match = pathname.match(/^\/m\/([^/]+)$/);

  return match ? match[1] : null;
}

/**
 * Validate that a URL has both message ID and encryption key
 * @param {string} url - URL to validate
 * @returns {boolean} True if valid vanish URL
 */
export function isValidVanishURL(url) {
  try {
    const urlObj = new URL(url);
    const pathMatch = urlObj.pathname.match(/^\/m\/([^/]+)$/);
    const hasFragment = urlObj.hash.length > 1;

    return !!(pathMatch && hasFragment);
  } catch {
    return false;
  }
}
