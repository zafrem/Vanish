/**
 * Vanish Chrome Extension - Options Page
 * Manage keywords, settings, and authentication
 */

// Default keywords
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

/**
 * Settings Manager Class
 */
class SettingsManager {
  constructor() {
    this.keywords = [];
    this.settings = {};
  }

  /**
   * Initialize the settings page
   */
  async initialize() {
    await this.loadSettings();
    this.render();
    this.attachEventListeners();
    this.checkAuthStatus();
  }

  /**
   * Load settings from storage
   */
  async loadSettings() {
    try {
      const data = await chrome.storage.sync.get(['keywords', 'settings']);
      this.keywords = data.keywords || DEFAULT_KEYWORDS;
      this.settings = data.settings || DEFAULT_SETTINGS;

      // Populate settings UI
      document.getElementById('auto-scan').checked = this.settings.autoScan;
      document.getElementById('highlight-enabled').checked = this.settings.highlightEnabled;
      document.getElementById('scan-delay').value = this.settings.scanDelay;
      document.getElementById('max-matches').value = this.settings.maxMatchesPerPage;
    } catch (error) {
      console.error('[Vanish] Failed to load settings:', error);
      this.showStatus('Failed to load settings', 'error');
    }
  }

  /**
   * Save settings to storage
   */
  async saveSettings() {
    try {
      // Gather settings from UI
      this.settings.autoScan = document.getElementById('auto-scan').checked;
      this.settings.highlightEnabled = document.getElementById('highlight-enabled').checked;
      this.settings.scanDelay = parseInt(document.getElementById('scan-delay').value);
      this.settings.maxMatchesPerPage = parseInt(document.getElementById('max-matches').value);

      // Validate settings
      if (this.settings.scanDelay < 100 || this.settings.scanDelay > 5000) {
        throw new Error('Scan delay must be between 100 and 5000 ms');
      }

      if (this.settings.maxMatchesPerPage < 10 || this.settings.maxMatchesPerPage > 500) {
        throw new Error('Max matches must be between 10 and 500');
      }

      // Save to storage
      await chrome.storage.sync.set({
        keywords: this.keywords,
        settings: this.settings
      });

      this.showStatus('Settings saved successfully!', 'success');

      // Notify all tabs to reload settings
      const tabs = await chrome.tabs.query({});
      for (const tab of tabs) {
        try {
          await chrome.tabs.sendMessage(tab.id, { type: 'RELOAD_SETTINGS' });
        } catch (error) {
          // Silently fail if tab doesn't have content script
        }
      }
    } catch (error) {
      console.error('[Vanish] Failed to save settings:', error);
      this.showStatus(error.message, 'error');
    }
  }

  /**
   * Render keywords list
   */
  render() {
    const container = document.getElementById('keywords-list');
    container.innerHTML = this.keywords.map((kw, idx) => this.renderKeyword(kw, idx)).join('');
  }

  /**
   * Render a single keyword
   */
  renderKeyword(keyword, idx) {
    return `
      <div class="keyword-item" data-index="${idx}">
        <div class="keyword-header">
          <label class="checkbox-label">
            <input type="checkbox"
                   data-index="${idx}"
                   ${keyword.enabled ? 'checked' : ''}
                   class="keyword-enabled" />
            <span>Enabled</span>
          </label>
          <button data-index="${idx}" class="btn-delete" title="Delete keyword">🗑️</button>
        </div>

        <div class="keyword-field">
          <label>Pattern:</label>
          <input type="text"
                 data-index="${idx}"
                 value="${this.escapeHtml(keyword.pattern)}"
                 class="keyword-pattern"
                 placeholder="e.g., password or api[_-]?key" />
        </div>

        <div class="keyword-row">
          <div class="keyword-field">
            <label>Type:</label>
            <select data-index="${idx}" class="keyword-type">
              <option value="literal" ${keyword.type === 'literal' ? 'selected' : ''}>Literal</option>
              <option value="regex" ${keyword.type === 'regex' ? 'selected' : ''}>Regex</option>
            </select>
          </div>

          <div class="keyword-field">
            <label>Color:</label>
            <input type="color"
                   data-index="${idx}"
                   value="${keyword.color}"
                   class="keyword-color" />
          </div>

          <div class="keyword-field">
            <label class="checkbox-label">
              <input type="checkbox"
                     data-index="${idx}"
                     ${keyword.caseSensitive ? 'checked' : ''}
                     class="keyword-case-sensitive" />
              <span>Case Sensitive</span>
            </label>
          </div>
        </div>
      </div>
    `;
  }

  /**
   * Attach event listeners
   */
  attachEventListeners() {
    // Save button
    document.getElementById('save-btn').addEventListener('click', () => this.handleSave());

    // Add keyword button
    document.getElementById('add-keyword-btn').addEventListener('click', () => this.handleAddKeyword());

    // Reset defaults button
    document.getElementById('reset-defaults-btn').addEventListener('click', () => this.handleResetDefaults());

    // Refresh token button
    document.getElementById('refresh-token-btn').addEventListener('click', () => this.handleRefreshToken());

    // Clear token button
    document.getElementById('clear-token-btn').addEventListener('click', () => this.handleClearToken());

    // Delegate keyword events
    document.getElementById('keywords-list').addEventListener('click', (e) => {
      if (e.target.classList.contains('btn-delete')) {
        const idx = parseInt(e.target.dataset.index);
        this.handleDeleteKeyword(idx);
      }
    });

    document.getElementById('keywords-list').addEventListener('change', (e) => {
      const idx = parseInt(e.target.dataset.index);

      if (e.target.classList.contains('keyword-enabled')) {
        this.keywords[idx].enabled = e.target.checked;
      } else if (e.target.classList.contains('keyword-pattern')) {
        this.keywords[idx].pattern = e.target.value;
      } else if (e.target.classList.contains('keyword-type')) {
        this.keywords[idx].type = e.target.value;
      } else if (e.target.classList.contains('keyword-color')) {
        this.keywords[idx].color = e.target.value;
      } else if (e.target.classList.contains('keyword-case-sensitive')) {
        this.keywords[idx].caseSensitive = e.target.checked;
      }
    });
  }

  /**
   * Handle save button
   */
  handleSave() {
    this.saveSettings();
  }

  /**
   * Handle add keyword
   */
  handleAddKeyword() {
    const newKeyword = {
      id: `kw-${Date.now()}`,
      pattern: '',
      type: 'literal',
      enabled: true,
      color: '#3b82f6',
      caseSensitive: false
    };

    this.keywords.push(newKeyword);
    this.render();
    this.showStatus('Keyword added. Don\'t forget to save!', 'info');
  }

  /**
   * Handle delete keyword
   */
  handleDeleteKeyword(idx) {
    if (confirm('Are you sure you want to delete this keyword?')) {
      this.keywords.splice(idx, 1);
      this.render();
      this.showStatus('Keyword deleted. Don\'t forget to save!', 'info');
    }
  }

  /**
   * Handle reset defaults
   */
  handleResetDefaults() {
    if (confirm('Reset all keywords to defaults? This will overwrite your current keywords.')) {
      this.keywords = JSON.parse(JSON.stringify(DEFAULT_KEYWORDS));
      this.settings = JSON.parse(JSON.stringify(DEFAULT_SETTINGS));

      // Update UI
      document.getElementById('auto-scan').checked = this.settings.autoScan;
      document.getElementById('highlight-enabled').checked = this.settings.highlightEnabled;
      document.getElementById('scan-delay').value = this.settings.scanDelay;
      document.getElementById('max-matches').value = this.settings.maxMatchesPerPage;

      this.render();
      this.showStatus('Reset to defaults. Don\'t forget to save!', 'info');
    }
  }

  /**
   * Handle refresh token
   */
  async handleRefreshToken() {
    try {
      this.showAuthStatus('Refreshing token...', 'loading');

      const response = await chrome.runtime.sendMessage({ type: 'REFRESH_TOKEN' });

      if (response.error) {
        throw new Error(response.error);
      }

      this.showAuthStatus('Token refreshed successfully!', 'success');
      setTimeout(() => this.checkAuthStatus(), 2000);
    } catch (error) {
      this.showAuthStatus(`Error: ${error.message}`, 'error');
    }
  }

  /**
   * Handle clear token
   */
  async handleClearToken() {
    if (confirm('Clear cached token? You will need to log in to Vanish again.')) {
      try {
        await chrome.runtime.sendMessage({ type: 'CLEAR_TOKEN' });
        this.showAuthStatus('Token cleared', 'info');
      } catch (error) {
        this.showAuthStatus(`Error: ${error.message}`, 'error');
      }
    }
  }

  /**
   * Check authentication status
   */
  async checkAuthStatus() {
    try {
      const response = await chrome.runtime.sendMessage({ type: 'GET_TOKEN' });

      if (response.error) {
        this.showAuthStatus(`Not authenticated: ${response.error}`, 'warning');
      } else if (response.token) {
        this.showAuthStatus('Authenticated ✓', 'success');
      } else {
        this.showAuthStatus('Not authenticated', 'warning');
      }
    } catch (error) {
      this.showAuthStatus('Unable to check status', 'error');
    }
  }

  /**
   * Show auth status
   */
  showAuthStatus(message, type) {
    const statusEl = document.getElementById('auth-status');
    statusEl.innerHTML = `<div class="status-${type}">${message}</div>`;
  }

  /**
   * Show status message
   */
  showStatus(message, type = 'info') {
    const statusEl = document.getElementById('status-message');
    statusEl.textContent = message;
    statusEl.className = `status-message status-${type}`;

    setTimeout(() => {
      statusEl.textContent = '';
      statusEl.className = 'status-message';
    }, 3000);
  }

  /**
   * Escape HTML
   */
  escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }
}

// Initialize settings manager
const manager = new SettingsManager();
manager.initialize();
