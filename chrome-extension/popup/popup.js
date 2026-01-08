/**
 * Vanish Chrome Extension - Popup UI
 * Displays detected secrets and handles Vanish link creation
 */

import { generateKey, exportKey, encrypt } from '../lib/crypto.js';

/**
 * Vanish Popup Class
 * Manages popup UI state and interactions
 */
class VanishPopup {
  constructor() {
    this.matches = [];
    this.users = [];
    this.selectedMatch = null;
    this.selectedRecipient = null;
    this.currentView = 'matches'; // 'matches', 'recipients', 'success', 'error'
  }

  /**
   * Initialize the popup
   */
  async initialize() {
    try {
      await this.loadMatches();
      this.render();
    } catch (error) {
      this.showError(error.message);
    }
  }

  /**
   * Load matches from content script
   */
  async loadMatches() {
    try {
      const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });

      if (!tab.id) {
        throw new Error('No active tab found');
      }

      const response = await chrome.tabs.sendMessage(tab.id, { type: 'GET_MATCHES' });
      this.matches = response.matches || [];
    } catch (error) {
      console.error('[Vanish] Failed to load matches:', error);
      this.matches = [];
    }
  }

  /**
   * Load users from Vanish API
   */
  async loadUsers() {
    try {
      const { token } = await chrome.runtime.sendMessage({ type: 'GET_TOKEN' });

      if (!token) {
        throw new Error('Please log in to Vanish first');
      }

      const { apiBase } = await chrome.runtime.sendMessage({ type: 'GET_API_BASE' });

      const response = await fetch(`${apiBase}/users`, {
        headers: {
          'Authorization': `Bearer ${token}`
        }
      });

      if (!response.ok) {
        throw new Error('Failed to fetch users');
      }

      this.users = await response.json();
    } catch (error) {
      console.error('[Vanish] Failed to load users:', error);
      throw error;
    }
  }

  /**
   * Create Vanish message and generate shareable link
   */
  async createVanishMessage(secretText, recipientId) {
    try {
      // Get token
      const { token, error: tokenError } = await chrome.runtime.sendMessage({ type: 'GET_TOKEN' });

      if (tokenError) {
        throw new Error(tokenError);
      }

      if (!token) {
        throw new Error('Please log in to Vanish first');
      }

      // Get API base
      const { apiBase } = await chrome.runtime.sendMessage({ type: 'GET_API_BASE' });
      const { vanishBase } = await chrome.runtime.sendMessage({ type: 'GET_VANISH_BASE' });

      // Generate encryption key
      const key = await generateKey();
      const keyString = await exportKey(key);

      // Encrypt message
      const { ciphertext, iv } = await encrypt(secretText, key);

      // Create message via API
      const response = await fetch(`${apiBase}/messages`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          ciphertext,
          iv,
          recipient_id: recipientId,
          encryption_key: keyString,
          ttl: 86400 // 24 hours
        })
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({ error: 'Unknown error' }));
        throw new Error(errorData.error || 'Failed to create message');
      }

      const data = await response.json();

      // Generate shareable URL with key in fragment
      const url = `${vanishBase}/m/${data.id}#${keyString}`;

      return url;
    } catch (error) {
      console.error('[Vanish] Failed to create message:', error);
      throw error;
    }
  }

  /**
   * Render the appropriate view
   */
  render() {
    const app = document.getElementById('app');

    switch (this.currentView) {
      case 'matches':
        app.innerHTML = this.renderMatches();
        this.attachMatchesEventListeners();
        break;
      case 'recipients':
        app.innerHTML = this.renderRecipients();
        this.attachRecipientsEventListeners();
        break;
      case 'success':
        app.innerHTML = this.renderSuccess();
        this.attachSuccessEventListeners();
        break;
      case 'error':
        app.innerHTML = this.renderError();
        this.attachErrorEventListeners();
        break;
      default:
        app.innerHTML = '<p>Unknown view</p>';
    }
  }

  /**
   * Render matches view
   */
  renderMatches() {
    if (this.matches.length === 0) {
      return `
        <div class="empty-state">
          <div class="icon">🔍</div>
          <h2>No Secrets Detected</h2>
          <p>No sensitive information found on this page.</p>
          <button id="rescan-btn" class="btn-secondary">Rescan Page</button>
          <button id="settings-btn" class="btn-link">Configure Keywords</button>
        </div>
      `;
    }

    return `
      <div class="matches-container">
        <div class="header">
          <h2>Detected Secrets (${this.matches.length})</h2>
          <button id="rescan-btn" class="btn-icon" title="Rescan page">🔄</button>
        </div>

        <div class="matches-list">
          ${this.matches.map((match, idx) => this.renderMatch(match, idx)).join('')}
        </div>

        <div class="actions">
          <button id="settings-btn" class="btn-secondary">Settings</button>
        </div>
      </div>
    `;
  }

  /**
   * Render a single match
   */
  renderMatch(match, idx) {
    return `
      <div class="match-item">
        <div class="match-header">
          <span class="match-keyword" style="background-color: ${match.color}20; color: ${match.color}; border: 1px solid ${match.color};">
            ${this.escapeHtml(match.keyword)}
          </span>
          <button class="btn-create" data-index="${idx}">Create Link</button>
        </div>
        <div class="match-context">
          <span class="context-prefix">${this.escapeHtml(match.context.prefix)}</span>
          <span class="context-match">${this.escapeHtml(match.context.match)}</span>
          <span class="context-suffix">${this.escapeHtml(match.context.suffix)}</span>
        </div>
      </div>
    `;
  }

  /**
   * Render recipients selection view
   */
  renderRecipients() {
    if (!this.selectedMatch) {
      return '<p>No match selected</p>';
    }

    return `
      <div class="recipients-container">
        <h2>Select Recipient</h2>

        <div class="secret-preview">
          <label>Secret to share:</label>
          <div class="secret-text">${this.escapeHtml(this.selectedMatch.text)}</div>
        </div>

        <div class="recipient-list">
          <input type="text" id="recipient-search" placeholder="Search users..." />
          <div id="users-list" class="users-list">
            ${this.users.length === 0 ? '<p class="loading">Loading users...</p>' : ''}
            ${this.users.map(user => this.renderUser(user)).join('')}
          </div>
        </div>

        <div class="actions">
          <button id="cancel-btn" class="btn-secondary">Cancel</button>
        </div>
      </div>
    `;
  }

  /**
   * Render a single user
   */
  renderUser(user) {
    return `
      <div class="user-item" data-user-id="${user.id}">
        <div class="user-info">
          <div class="user-name">${this.escapeHtml(user.name)}</div>
          <div class="user-email">${this.escapeHtml(user.email)}</div>
        </div>
        <button class="btn-select" data-user-id="${user.id}">Select</button>
      </div>
    `;
  }

  /**
   * Render success view
   */
  renderSuccess() {
    return `
      <div class="success-container">
        <div class="icon success">✓</div>
        <h2>Secret Link Created!</h2>
        <p>Share this link securely with the recipient</p>

        <div class="url-box">
          <input type="text" id="shareable-url" value="${this.escapeHtml(this.shareableURL)}" readonly />
        </div>

        <div class="actions">
          <button id="copy-btn" class="btn-primary">Copy Link</button>
          <button id="new-btn" class="btn-secondary">Create Another</button>
        </div>

        <div class="info-box">
          <p>⚠️ This link can only be opened once</p>
          <p>🔥 The secret will be permanently destroyed after reading</p>
        </div>
      </div>
    `;
  }

  /**
   * Render error view
   */
  renderError() {
    return `
      <div class="error-container">
        <div class="icon error">⚠️</div>
        <h2>Error</h2>
        <p class="error-message">${this.escapeHtml(this.errorMessage)}</p>

        <div class="actions">
          <button id="back-btn" class="btn-primary">Go Back</button>
          <button id="refresh-token-btn" class="btn-secondary">Refresh Token</button>
        </div>
      </div>
    `;
  }

  /**
   * Attach event listeners for matches view
   */
  attachMatchesEventListeners() {
    document.getElementById('rescan-btn')?.addEventListener('click', () => this.handleRescan());
    document.getElementById('settings-btn')?.addEventListener('click', () => this.handleSettings());

    document.querySelectorAll('.btn-create').forEach(btn => {
      btn.addEventListener('click', (e) => {
        const idx = parseInt(e.target.dataset.index);
        this.handleCreateLink(idx);
      });
    });
  }

  /**
   * Attach event listeners for recipients view
   */
  attachRecipientsEventListeners() {
    document.getElementById('cancel-btn')?.addEventListener('click', () => this.handleCancel());
    document.getElementById('recipient-search')?.addEventListener('input', (e) => this.handleSearch(e.target.value));

    document.querySelectorAll('.btn-select').forEach(btn => {
      btn.addEventListener('click', (e) => {
        const userId = parseInt(e.target.dataset.userId);
        this.handleSelectRecipient(userId);
      });
    });
  }

  /**
   * Attach event listeners for success view
   */
  attachSuccessEventListeners() {
    document.getElementById('copy-btn')?.addEventListener('click', () => this.handleCopy());
    document.getElementById('new-btn')?.addEventListener('click', () => this.handleNew());
  }

  /**
   * Attach event listeners for error view
   */
  attachErrorEventListeners() {
    document.getElementById('back-btn')?.addEventListener('click', () => this.handleBack());
    document.getElementById('refresh-token-btn')?.addEventListener('click', () => this.handleRefreshToken());
  }

  /**
   * Handle rescan button click
   */
  async handleRescan() {
    try {
      const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
      await chrome.tabs.sendMessage(tab.id, { type: 'RESCAN_PAGE' });
      await this.loadMatches();
      this.render();
    } catch (error) {
      this.showError('Failed to rescan page');
    }
  }

  /**
   * Handle settings button click
   */
  handleSettings() {
    chrome.runtime.openOptionsPage();
  }

  /**
   * Handle create link button click
   */
  async handleCreateLink(matchIndex) {
    this.selectedMatch = this.matches[matchIndex];
    this.currentView = 'recipients';
    this.render();

    try {
      await this.loadUsers();
      this.render(); // Re-render with users loaded
    } catch (error) {
      this.showError(error.message);
    }
  }

  /**
   * Handle cancel button click
   */
  handleCancel() {
    this.currentView = 'matches';
    this.selectedMatch = null;
    this.render();
  }

  /**
   * Handle recipient search
   */
  handleSearch(query) {
    const usersList = document.getElementById('users-list');
    const lowerQuery = query.toLowerCase();

    document.querySelectorAll('.user-item').forEach(item => {
      const name = item.querySelector('.user-name').textContent.toLowerCase();
      const email = item.querySelector('.user-email').textContent.toLowerCase();

      if (name.includes(lowerQuery) || email.includes(lowerQuery)) {
        item.style.display = 'flex';
      } else {
        item.style.display = 'none';
      }
    });
  }

  /**
   * Handle recipient selection
   */
  async handleSelectRecipient(userId) {
    try {
      this.selectedRecipient = this.users.find(u => u.id === userId);

      // Show loading
      document.getElementById('app').innerHTML = `
        <div class="loading">
          <div class="spinner"></div>
          <p>Creating secure link...</p>
        </div>
      `;

      // Create Vanish message
      this.shareableURL = await this.createVanishMessage(
        this.selectedMatch.text,
        userId
      );

      this.currentView = 'success';
      this.render();
    } catch (error) {
      this.showError(error.message);
    }
  }

  /**
   * Handle copy button click
   */
  async handleCopy() {
    try {
      const urlInput = document.getElementById('shareable-url');
      urlInput.select();
      await navigator.clipboard.writeText(this.shareableURL);

      const copyBtn = document.getElementById('copy-btn');
      copyBtn.textContent = '✓ Copied!';
      copyBtn.classList.add('success');

      setTimeout(() => {
        copyBtn.textContent = 'Copy Link';
        copyBtn.classList.remove('success');
      }, 2000);
    } catch (error) {
      this.showError('Failed to copy URL');
    }
  }

  /**
   * Handle new button click
   */
  handleNew() {
    this.currentView = 'matches';
    this.selectedMatch = null;
    this.selectedRecipient = null;
    this.shareableURL = null;
    this.render();
  }

  /**
   * Handle back button click
   */
  handleBack() {
    this.currentView = 'matches';
    this.errorMessage = null;
    this.render();
  }

  /**
   * Handle refresh token button click
   */
  async handleRefreshToken() {
    try {
      document.getElementById('app').innerHTML = `
        <div class="loading">
          <div class="spinner"></div>
          <p>Refreshing token...</p>
        </div>
      `;

      const { token, error } = await chrome.runtime.sendMessage({ type: 'REFRESH_TOKEN' });

      if (error) {
        throw new Error(error);
      }

      // Go back to matches
      this.currentView = 'matches';
      this.render();
    } catch (error) {
      this.showError(error.message);
    }
  }

  /**
   * Show error state
   */
  showError(message) {
    this.errorMessage = message;
    this.currentView = 'error';
    this.render();
  }

  /**
   * Escape HTML to prevent XSS
   */
  escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }
}

// Initialize popup
const popup = new VanishPopup();
popup.initialize();
