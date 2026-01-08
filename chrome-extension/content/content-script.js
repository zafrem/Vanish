/**
 * Vanish Chrome Extension - Content Script
 * Scans pages for user-defined keywords and highlights matches
 */

// Default keywords for secret detection
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
 * Secret Scanner Class
 * Handles page scanning, keyword detection, and highlighting
 */
class SecretScanner {
  constructor() {
    this.matches = [];
    this.keywords = [];
    this.settings = {};
    this.highlights = [];
    this.observer = null;
    this.scanTimeout = null;
  }

  /**
   * Initialize the scanner
   */
  async initialize() {
    try {
      // Load keywords and settings from storage
      const data = await chrome.storage.sync.get(['keywords', 'settings']);
      this.keywords = data.keywords || DEFAULT_KEYWORDS;
      this.settings = data.settings || DEFAULT_SETTINGS;

      // Start scanning if auto-scan is enabled
      if (this.settings.autoScan) {
        this.scanPage();
        this.setupMutationObserver();
      }
    } catch (error) {
      console.error('[Vanish] Failed to initialize scanner:', error);
    }
  }

  /**
   * Scan the entire page for keywords
   */
  scanPage() {
    this.matches = [];
    this.clearHighlights();

    try {
      const textNodes = this.getTextNodes(document.body);

      for (const node of textNodes) {
        this.scanNode(node);
      }

      // Limit matches
      const limitedMatches = this.matches.slice(0, this.settings.maxMatchesPerPage);

      // Send results to background script
      chrome.runtime.sendMessage({
        type: 'SCAN_COMPLETE',
        matches: limitedMatches
      }).catch(() => {
        // Silently fail if extension context is invalidated
      });
    } catch (error) {
      console.error('[Vanish] Error during page scan:', error);
    }
  }

  /**
   * Scan a single text node for keyword matches
   */
  scanNode(node) {
    const text = node.textContent;

    for (const keyword of this.keywords.filter(k => k.enabled)) {
      try {
        const regex = this.buildRegex(keyword);
        let match;

        while ((match = regex.exec(text)) !== null) {
          const context = this.extractContext(text, match.index, match[0].length);

          this.matches.push({
            text: match[0],
            keyword: keyword.pattern,
            context: context,
            position: match.index,
            color: keyword.color
          });

          // Highlight if enabled
          if (this.settings.highlightEnabled) {
            this.highlightMatch(node, match.index, match[0].length, keyword.color);
          }

          // Prevent infinite loops with zero-width matches
          if (match[0].length === 0) {
            regex.lastIndex++;
          }
        }
      } catch (error) {
        console.error(`[Vanish] Error processing keyword "${keyword.pattern}":`, error);
      }
    }
  }

  /**
   * Build regex from keyword pattern
   */
  buildRegex(keyword) {
    if (keyword.type === 'regex') {
      const flags = keyword.caseSensitive ? 'g' : 'gi';
      return new RegExp(keyword.pattern, flags);
    } else {
      // Literal match - escape special characters
      const escaped = keyword.pattern.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
      const flags = keyword.caseSensitive ? 'g' : 'gi';
      return new RegExp(escaped, flags);
    }
  }

  /**
   * Highlight a match in the DOM
   */
  highlightMatch(node, start, length, color) {
    try {
      // Find the exact position in the text node
      const text = node.textContent;
      const matchText = text.substring(start, start + length);

      // Create a temporary div to work with
      const parent = node.parentNode;
      if (!parent) return;

      // Split the text node
      const before = text.substring(0, start);
      const after = text.substring(start + length);

      // Create highlight span
      const highlight = document.createElement('span');
      highlight.className = 'vanish-highlight';
      highlight.style.backgroundColor = color;
      highlight.textContent = matchText;
      highlight.setAttribute('data-vanish-match', 'true');

      // Replace the text node with the highlighted version
      const fragment = document.createDocumentFragment();
      if (before) fragment.appendChild(document.createTextNode(before));
      fragment.appendChild(highlight);
      if (after) fragment.appendChild(document.createTextNode(after));

      parent.replaceChild(fragment, node);

      this.highlights.push(highlight);
    } catch (error) {
      // Silently fail if highlighting fails
    }
  }

  /**
   * Extract context around a match for display
   */
  extractContext(text, position, length, contextLength = 50) {
    const start = Math.max(0, position - contextLength);
    const end = Math.min(text.length, position + length + contextLength);

    let prefix = text.substring(start, position);
    const match = text.substring(position, position + length);
    let suffix = text.substring(position + length, end);

    if (start > 0) prefix = '...' + prefix;
    if (end < text.length) suffix = suffix + '...';

    return { prefix, match, suffix };
  }

  /**
   * Get all text nodes from an element
   */
  getTextNodes(element) {
    const textNodes = [];
    const walker = document.createTreeWalker(
      element,
      NodeFilter.SHOW_TEXT,
      {
        acceptNode: (node) => {
          const parent = node.parentElement;
          if (!parent) return NodeFilter.FILTER_REJECT;

          // Skip script, style, and other non-visible elements
          const tagName = parent.tagName.toLowerCase();
          if (['script', 'style', 'noscript', 'iframe', 'object'].includes(tagName)) {
            return NodeFilter.FILTER_REJECT;
          }

          // Skip elements that already have vanish highlights
          if (parent.classList.contains('vanish-highlight')) {
            return NodeFilter.FILTER_REJECT;
          }

          // Skip hidden elements
          const style = window.getComputedStyle(parent);
          if (style.display === 'none' || style.visibility === 'hidden') {
            return NodeFilter.FILTER_REJECT;
          }

          // Skip empty text nodes
          if (!node.textContent.trim()) {
            return NodeFilter.FILTER_REJECT;
          }

          return NodeFilter.FILTER_ACCEPT;
        }
      }
    );

    let node;
    while ((node = walker.nextNode())) {
      textNodes.push(node);
    }

    return textNodes;
  }

  /**
   * Setup mutation observer to detect DOM changes
   */
  setupMutationObserver() {
    this.observer = new MutationObserver((mutations) => {
      // Debounce scanning on DOM changes
      clearTimeout(this.scanTimeout);
      this.scanTimeout = setTimeout(() => {
        this.scanPage();
      }, this.settings.scanDelay);
    });

    this.observer.observe(document.body, {
      childList: true,
      subtree: true,
      characterData: true
    });
  }

  /**
   * Clear all highlights
   */
  clearHighlights() {
    for (const highlight of this.highlights) {
      try {
        const parent = highlight.parentNode;
        if (parent) {
          parent.replaceChild(document.createTextNode(highlight.textContent), highlight);
        }
      } catch (error) {
        // Silently fail
      }
    }
    this.highlights = [];
  }

  /**
   * Reload settings and rescan
   */
  async reload() {
    try {
      const data = await chrome.storage.sync.get(['keywords', 'settings']);
      this.keywords = data.keywords || DEFAULT_KEYWORDS;
      this.settings = data.settings || DEFAULT_SETTINGS;
      this.scanPage();
    } catch (error) {
      console.error('[Vanish] Failed to reload settings:', error);
    }
  }

  /**
   * Cleanup and destroy scanner
   */
  destroy() {
    if (this.observer) {
      this.observer.disconnect();
    }
    clearTimeout(this.scanTimeout);
    this.clearHighlights();
  }
}

// Initialize scanner
const scanner = new SecretScanner();
scanner.initialize();

// Listen for messages from popup/background
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  try {
    if (message.type === 'RESCAN_PAGE') {
      scanner.scanPage();
      sendResponse({ success: true });
    } else if (message.type === 'GET_MATCHES') {
      sendResponse({ matches: scanner.matches });
    } else if (message.type === 'CLEAR_HIGHLIGHTS') {
      scanner.clearHighlights();
      sendResponse({ success: true });
    } else if (message.type === 'RELOAD_SETTINGS') {
      scanner.reload();
      sendResponse({ success: true });
    }
  } catch (error) {
    sendResponse({ error: error.message });
  }

  return true; // Keep message channel open for async response
});

// Cleanup on unload
window.addEventListener('beforeunload', () => {
  scanner.destroy();
});
