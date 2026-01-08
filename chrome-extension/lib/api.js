/**
 * Vanish Chrome Extension - API Client
 * Adapted from frontend/src/lib/api.js for extension context
 */

/**
 * Create a new encrypted message
 * @param {string} token - JWT authentication token
 * @param {string} apiBase - API base URL
 * @param {string} ciphertext - Base64-encoded encrypted data
 * @param {string} iv - Base64-encoded initialization vector
 * @param {number} recipientId - ID of the intended recipient
 * @param {string} encryptionKey - Client-side encryption key for recipient access
 * @param {number} ttl - Time to live in seconds (optional)
 * @returns {Promise<{id: string, expiresAt: string}>}
 */
export async function createMessage(token, apiBase, ciphertext, iv, recipientId, encryptionKey, ttl = 86400) {
  const payload = {
    ciphertext,
    iv,
    recipient_id: recipientId,
    encryption_key: encryptionKey,
    ttl
  };

  const response = await fetch(`${apiBase}/messages`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`
    },
    body: JSON.stringify(payload)
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: 'Unknown error' }));
    throw new Error(error.error || 'Failed to create message');
  }

  return response.json();
}

/**
 * Get list of all users (for recipient selection)
 * @param {string} token - JWT authentication token
 * @param {string} apiBase - API base URL
 * @returns {Promise<Array<{id: number, name: string, email: string}>>}
 */
export async function getUsers(token, apiBase) {
  const response = await fetch(`${apiBase}/users`, {
    headers: {
      'Authorization': `Bearer ${token}`
    }
  });

  if (!response.ok) {
    throw new Error('Failed to fetch users');
  }

  return response.json();
}

/**
 * Retrieve and burn a message (atomic operation)
 * @param {string} token - JWT authentication token
 * @param {string} apiBase - API base URL
 * @param {string} messageId - The message ID
 * @returns {Promise<{ciphertext: string, iv: string}>}
 */
export async function getMessage(token, apiBase, messageId) {
  const response = await fetch(`${apiBase}/messages/${messageId}`, {
    method: 'GET',
    headers: {
      'Authorization': `Bearer ${token}`
    }
  });

  if (!response.ok) {
    if (response.status === 404) {
      throw new Error('Message not found or already burned');
    }
    if (response.status === 403) {
      throw new Error('You are not the intended recipient of this message');
    }
    const error = await response.json().catch(() => ({ error: 'Unknown error' }));
    throw new Error(error.error || 'Failed to retrieve message');
  }

  return response.json();
}

/**
 * Check backend health
 * @param {string} apiBase - API base URL
 * @returns {Promise<{status: string}>}
 */
export async function checkHealth(apiBase) {
  const healthUrl = apiBase.replace('/api', '/health');
  const response = await fetch(healthUrl);
  return response.json();
}
