/**
 * Vanish Chrome Extension - Background Service Worker
 * Handles token management and API coordination
 */

/**
 * Token Manager Class
 * Extracts and manages JWT tokens from Vanish tabs
 */
class TokenManager {
  constructor() {
    this.token = null;
    this.tokenTimestamp = null;
  }

  /**
   * Extract JWT token from an authenticated Vanish tab
   */
  async extractTokenFromVanishTab() {
    try {
      // Find tabs matching Vanish URLs
      const tabs = await chrome.tabs.query({
        url: ['http://localhost:3000/*', 'https://vanish.app/*']
      });

      if (tabs.length === 0) {
        throw new Error('No Vanish tab found. Please open Vanish and log in first.');
      }

      // Use the first matching tab
      const tab = tabs[0];

      // Execute script in Vanish tab to read localStorage
      const results = await chrome.scripting.executeScript({
        target: { tabId: tab.id },
        func: () => localStorage.getItem('token')
      });

      const token = results[0].result;

      if (!token) {
        throw new Error('Not authenticated in Vanish. Please log in first.');
      }

      // Cache token in chrome.storage.local
      await chrome.storage.local.set({
        vanishToken: token,
        tokenTimestamp: Date.now()
      });

      this.token = token;
      this.tokenTimestamp = Date.now();

      return token;
    } catch (error) {
      console.error('[Vanish] Token extraction failed:', error);
      throw error;
    }
  }

  /**
   * Get cached or fresh token
   * Tokens are cached for 30 seconds to avoid excessive extraction
   */
  async getToken() {
    try {
      // Check in-memory cache first
      if (this.token && this.tokenTimestamp) {
        const age = Date.now() - this.tokenTimestamp;
        if (age < 30000) { // 30 second cache
          return this.token;
        }
      }

      // Try to load from chrome.storage.local
      const data = await chrome.storage.local.get(['vanishToken', 'tokenTimestamp']);

      if (data.vanishToken && data.tokenTimestamp) {
        const age = Date.now() - data.tokenTimestamp;
        if (age < 30000) { // 30 second cache
          this.token = data.vanishToken;
          this.tokenTimestamp = data.tokenTimestamp;
          return this.token;
        }
      }

      // Extract fresh token
      return await this.extractTokenFromVanishTab();
    } catch (error) {
      console.error('[Vanish] Failed to get token:', error);
      throw error;
    }
  }

  /**
   * Validate token by calling /auth/me endpoint
   */
  async validateToken(token) {
    try {
      const apiBase = await this.getApiBase();
      const response = await fetch(`${apiBase}/auth/me`, {
        headers: {
          'Authorization': `Bearer ${token}`
        }
      });

      return response.ok;
    } catch (error) {
      console.error('[Vanish] Token validation failed:', error);
      return false;
    }
  }

  /**
   * Clear cached token
   */
  async clearToken() {
    this.token = null;
    this.tokenTimestamp = null;
    await chrome.storage.local.remove(['vanishToken', 'tokenTimestamp']);
  }

  /**
   * Detect API base URL (localhost vs production)
   */
  async getApiBase() {
    try {
      // Check if there's a Vanish tab open to determine environment
      const tabs = await chrome.tabs.query({
        url: ['http://localhost:3000/*', 'https://vanish.app/*']
      });

      if (tabs.length > 0 && tabs[0].url.includes('localhost')) {
        return 'http://localhost:8080/api';
      }

      return 'https://api.vanish.app/api';
    } catch (error) {
      // Default to production
      return 'https://api.vanish.app/api';
    }
  }

  /**
   * Get Vanish base URL for generating shareable links
   */
  async getVanishBase() {
    try {
      const tabs = await chrome.tabs.query({
        url: ['http://localhost:3000/*', 'https://vanish.app/*']
      });

      if (tabs.length > 0 && tabs[0].url.includes('localhost')) {
        return 'http://localhost:3000';
      }

      return 'https://vanish.app';
    } catch (error) {
      return 'https://vanish.app';
    }
  }
}

// Initialize token manager
const tokenManager = new TokenManager();

/**
 * Message handler for communication with popup/content scripts
 */
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  // Handle different message types
  if (message.type === 'GET_TOKEN') {
    tokenManager.getToken()
      .then(token => sendResponse({ token }))
      .catch(error => sendResponse({ error: error.message }));
    return true; // Keep message channel open for async response
  }

  if (message.type === 'REFRESH_TOKEN') {
    tokenManager.extractTokenFromVanishTab()
      .then(token => sendResponse({ token }))
      .catch(error => sendResponse({ error: error.message }));
    return true;
  }

  if (message.type === 'CLEAR_TOKEN') {
    tokenManager.clearToken()
      .then(() => sendResponse({ success: true }))
      .catch(error => sendResponse({ error: error.message }));
    return true;
  }

  if (message.type === 'GET_API_BASE') {
    tokenManager.getApiBase()
      .then(apiBase => sendResponse({ apiBase }))
      .catch(error => sendResponse({ error: error.message }));
    return true;
  }

  if (message.type === 'GET_VANISH_BASE') {
    tokenManager.getVanishBase()
      .then(vanishBase => sendResponse({ vanishBase }))
      .catch(error => sendResponse({ error: error.message }));
    return true;
  }

  if (message.type === 'VALIDATE_TOKEN') {
    tokenManager.validateToken(message.token)
      .then(isValid => sendResponse({ isValid }))
      .catch(error => sendResponse({ error: error.message }));
    return true;
  }

  if (message.type === 'SCAN_COMPLETE') {
    // Store matches from content script (optional, for caching)
    // Could be used for popup to quickly access without querying content script
    chrome.storage.local.set({
      lastScanMatches: message.matches,
      lastScanTimestamp: Date.now()
    });
    return false;
  }

  return false;
});

/**
 * Extension installation handler
 */
chrome.runtime.onInstalled.addListener(async (details) => {
  if (details.reason === 'install') {
    // First install - initialize default settings
    const DEFAULT_KEYWORDS = [
      { id: 'kw-1', pattern: 'password\\s*[:=]\\s*\\S+', type: 'regex', enabled: true, color: '#FF6B6B', caseSensitive: false },
      { id: 'kw-2', pattern: 'api[_-]?key\\s*[:=]\\s*\\S+', type: 'regex', enabled: true, color: '#4ECDC4', caseSensitive: false },
      { id: 'kw-3', pattern: 'secret[_-]?key\\s*[:=]\\s*\\S+', type: 'regex', enabled: true, color: '#95E1D3', caseSensitive: false },
      { id: 'kw-4', pattern: 'token\\s*[:=]\\s*\\S+', type: 'regex', enabled: true, color: '#F38181', caseSensitive: false },
      { id: 'kw-5', pattern: 'AKIA[0-9A-Z]{16}', type: 'regex', enabled: true, color: '#FECA57', caseSensitive: true },
      { id: 'kw-6', pattern: '-----BEGIN (RSA |)PRIVATE KEY-----', type: 'regex', enabled: true, color: '#FCBAD3', caseSensitive: false }
    ];

    const DEFAULT_SETTINGS = {
      autoScan: true,
      highlightEnabled: true,
      scanDelay: 500,
      maxMatchesPerPage: 100
    };

    await chrome.storage.sync.set({
      keywords: DEFAULT_KEYWORDS,
      settings: DEFAULT_SETTINGS
    });

    console.log('[Vanish] Extension installed, default settings initialized');

    // Open options page
    chrome.runtime.openOptionsPage();
  } else if (details.reason === 'update') {
    console.log('[Vanish] Extension updated');
  }
});

/**
 * Handle extension context invalidation
 */
chrome.runtime.onSuspend.addListener(() => {
  console.log('[Vanish] Service worker suspending');
});

console.log('[Vanish] Background service worker initialized');
