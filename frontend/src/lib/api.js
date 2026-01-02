/**
 * API client for backend communication
 */

const API_BASE = import.meta.env.VITE_API_URL || '/api';

// Export token getter for admin components
export function getToken() {
  return localStorage.getItem('token');
}

// Helper to get auth headers
function getAuthHeaders() {
  const token = localStorage.getItem('token');
  return {
    'Content-Type': 'application/json',
    ...(token && { 'Authorization': `Bearer ${token}` })
  };
}

/**
 * Create a new encrypted message
 * @param {string} ciphertext - Base64-encoded encrypted data
 * @param {string} iv - Base64-encoded initialization vector
 * @param {number} recipientId - ID of the intended recipient
 * @param {string} encryptionKey - Client-side encryption key for recipient access
 * @param {number} ttl - Time to live in seconds (optional)
 * @returns {Promise<{id: string, expiresAt: string}>}
 */
export async function createMessage(ciphertext, iv, recipientId, encryptionKey, ttl = null) {
  const payload = {
    ciphertext,
    iv,
    recipient_id: recipientId,
    encryption_key: encryptionKey,
  };

  if (ttl !== null) {
    payload.ttl = ttl;
  }

  const response = await fetch(`${API_BASE}/messages`, {
    method: 'POST',
    headers: getAuthHeaders(),
    body: JSON.stringify(payload),
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: 'Unknown error' }));
    throw new Error(error.error || 'Failed to create message');
  }

  return response.json();
}

/**
 * Retrieve and burn a message (atomic operation)
 * @param {string} messageId - The message ID
 * @returns {Promise<{ciphertext: string, iv: string}>}
 */
export async function getMessage(messageId) {
  const response = await fetch(`${API_BASE}/messages/${messageId}`, {
    method: 'GET',
    headers: getAuthHeaders(),
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
 * Check if a message exists without burning it
 * @param {string} messageId - The message ID
 * @returns {Promise<boolean>} True if message exists
 */
export async function checkMessageExists(messageId) {
  const response = await fetch(`${API_BASE}/messages/${messageId}`, {
    method: 'HEAD',
    headers: getAuthHeaders(),
  });

  return response.ok;
}

/**
 * Get list of all users (for recipient selection)
 * @returns {Promise<Array<{id: number, name: string, email: string}>>}
 */
export async function getUsers() {
  const response = await fetch(`${API_BASE}/users`, {
    headers: getAuthHeaders(),
  });

  if (!response.ok) {
    throw new Error('Failed to fetch users');
  }

  return response.json();
}

/**
 * Get current user's message history
 * @param {number} limit - Maximum number of messages to return
 * @returns {Promise<Array>}
 */
export async function getHistory(limit = 50) {
  const response = await fetch(`${API_BASE}/history?limit=${limit}`, {
    headers: getAuthHeaders(),
  });

  if (!response.ok) {
    throw new Error('Failed to fetch history');
  }

  return response.json();
}

/**
 * Check backend health
 * @returns {Promise<{status: string}>}
 */
export async function checkHealth() {
  const response = await fetch('/health');
  return response.json();
}

/**
 * Send a Slack notification to the recipient
 * @param {number} recipientId
 * @param {string} messageUrl
 * @returns {Promise<void>}
 */
export async function sendSlackNotification(recipientId, messageUrl) {
  const response = await fetch(`${API_BASE}/notifications/send-slack`, {
    method: 'POST',
    headers: getAuthHeaders(),
    body: JSON.stringify({
      recipient_id: recipientId,
      message_url: messageUrl
    }),
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: 'Unknown error' }));
    throw new Error(error.error || 'Failed to send Slack notification');
  }
}

/**
 * Send an Email notification to the recipient
 * @param {number} recipientId
 * @param {string} messageUrl
 * @returns {Promise<void>}
 */
export async function sendEmailNotification(recipientId, messageUrl) {
  const response = await fetch(`${API_BASE}/notifications/send-email`, {
    method: 'POST',
    headers: getAuthHeaders(),
    body: JSON.stringify({
      recipient_id: recipientId,
      message_url: messageUrl
    }),
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: 'Unknown error' }));
    throw new Error(error.error || 'Failed to send Email notification');
  }
}
