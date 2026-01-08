# Vanish Chrome Extension

A Chrome extension that detects user-defined keywords on web pages and creates secure Vanish links for sharing sensitive information.

## Features

- 🔍 **Keyword Detection**: Automatically scans web pages for user-defined patterns (passwords, API keys, secrets)
- 🎨 **Visual Highlighting**: Highlights detected secrets with customizable colors
- 🔐 **Zero-Knowledge Encryption**: Client-side AES-256-GCM encryption before transmission
- 🔗 **One-Click Link Creation**: Generate shareable Vanish links directly from detected secrets
- ⚙️ **Customizable Keywords**: Configure your own patterns (regex or literal matching)
- 🔥 **Burn-on-Read**: Links are destroyed after first access

## Installation

### Development Mode (Load Unpacked)

1. **Add Extension Icons** (required before loading):
   ```bash
   cd chrome-extension/assets
   # Create placeholder icons (requires ImageMagick):
   convert -size 16x16 xc:#EF4444 icon16.png
   convert -size 48x48 xc:#EF4444 icon48.png
   convert -size 128x128 xc:#EF4444 icon128.png

   # Or use any image editor to create PNG icons in these sizes
   ```

2. **Open Chrome Extensions**:
   - Navigate to `chrome://extensions/`
   - Enable "Developer mode" (toggle in top-right)

3. **Load Extension**:
   - Click "Load unpacked"
   - Select the `chrome-extension/` directory
   - Extension should now appear in your toolbar

## Setup

### Prerequisites

- Chrome or Edge browser (Manifest V3 compatible)
- Vanish backend running (localhost:8080 or production)
- Vanish frontend running (localhost:3000 or production)

### First-Time Setup

1. **Log in to Vanish**:
   - Open Vanish web app (`http://localhost:3000` or production URL)
   - Register or log in with your account
   - Keep this tab open (extension extracts JWT token from here)

2. **Configure Extension**:
   - Click extension icon in toolbar
   - Click "Settings" to open options page
   - Review default keywords or add your own
   - Click "Save Settings"

## Usage

### Detecting Secrets

1. Navigate to any webpage
2. Extension automatically scans for keywords
3. Detected secrets are highlighted on the page
4. Click extension icon to see list of matches

### Creating Vanish Links

1. Click extension icon
2. View detected secrets
3. Click "Create Link" next to any secret
4. Select recipient from user list
5. Copy the generated URL
6. Share URL securely with recipient

### Managing Keywords

1. Click extension icon → "Settings"
2. **Add Keyword**:
   - Click "+ Add Keyword"
   - Enter pattern (e.g., `password` or `api[_-]?key`)
   - Choose type: "Literal" or "Regex"
   - Pick highlight color
   - Enable/disable as needed
3. **Edit Keyword**: Modify pattern, type, color, or case sensitivity
4. **Delete Keyword**: Click trash icon
5. **Reset**: Click "Reset to Defaults" to restore default keywords
6. **Save**: Click "Save Settings" to apply changes

## Default Keywords

The extension comes with these pre-configured patterns:

| Pattern | Type | Description |
|---------|------|-------------|
| `password\s*[:=]\s*\S+` | Regex | Detects "password: value" |
| `api[_-]?key\s*[:=]\s*\S+` | Regex | Detects "api_key: value" |
| `secret[_-]?key\s*[:=]\s*\S+` | Regex | Detects "secret_key: value" |
| `token\s*[:=]\s*\S+` | Regex | Detects "token: value" |
| `AKIA[0-9A-Z]{16}` | Regex | AWS Access Keys |
| `-----BEGIN (RSA \|)PRIVATE KEY-----` | Regex | Private keys |

## Settings

### Detection Settings

- **Auto-scan pages**: Automatically scan pages when they load
- **Highlight detected secrets**: Show colored highlights on page
- **Scan delay**: Debounce time (ms) before rescanning after DOM changes
- **Max matches per page**: Limit number of detected secrets (performance)

### Authentication

The extension extracts JWT tokens from authenticated Vanish tabs:

- **Token Cache**: Tokens cached for 30 seconds
- **Session Timeout**: Vanish sessions expire after 60 seconds
- **Refresh Token**: Click to extract fresh token from Vanish tab
- **Clear Token**: Remove cached token

## Architecture

```
chrome-extension/
├── manifest.json              # Extension configuration
├── background/
│   └── service-worker.js     # Token management, API calls
├── content/
│   ├── content-script.js     # Page scanning, keyword detection
│   └── highlight.css         # Highlight styling
├── popup/
│   ├── popup.html            # Extension popup UI
│   ├── popup.js              # UI logic, message creation
│   └── popup.css             # Popup styling
├── options/
│   ├── options.html          # Settings page
│   ├── options.js            # Keyword management
│   └── options.css           # Settings styling
├── lib/
│   ├── crypto.js             # AES-256-GCM encryption (from frontend)
│   └── api.js                # API client
└── assets/
    └── icon*.png             # Extension icons
```

## Security

### Client-Side Encryption

- Secrets encrypted in browser using Web Crypto API
- AES-256-GCM with random IV per message
- Encryption key never sent to server
- Key embedded in URL fragment (#) which browsers don't transmit

### Token Management

- JWT tokens extracted from Vanish localStorage
- Cached locally for 30 seconds
- Never logged or exposed
- Cleared on extension uninstall

### Permissions

- `storage`: Store keywords and settings
- `activeTab`: Access current page when popup opened
- `scripting`: Inject content scripts for scanning
- `tabs`: Query tabs for token extraction
- `host_permissions`: Access Vanish app and API

## Troubleshooting

### "No Vanish tab found"

**Solution**: Open Vanish web app and log in, then try again.

### "Not authenticated"

**Solution**:
1. Ensure you're logged in to Vanish
2. Click "Refresh Token" in extension settings
3. If problem persists, log out and back in to Vanish

### No secrets detected

**Solution**:
1. Click "Rescan Page" in popup
2. Check if keywords are enabled in settings
3. Add custom keywords if defaults don't match content

### Highlights not appearing

**Solution**:
1. Open extension settings
2. Ensure "Highlight detected secrets" is checked
3. Save settings and reload page

### Extension not working after update

**Solution**:
1. Go to `chrome://extensions/`
2. Find Vanish extension
3. Click "Reload" button

## Development

### File Structure

- **manifest.json**: Manifest V3 configuration
- **service-worker.js**: Background logic (token extraction, API coordination)
- **content-script.js**: Page scanning with DOM traversal and regex matching
- **popup.js**: UI state management and Vanish link creation
- **options.js**: Settings CRUD operations
- **crypto.js**: Reused from frontend (no modifications needed)
- **api.js**: Adapted API client for extension context

### Testing

1. **Load Extension**: Chrome → Extensions → Load Unpacked → Select directory
2. **Test Scanning**: Navigate to page with text, verify detection
3. **Test Authentication**: Log in to Vanish, verify token extraction
4. **Test Link Creation**: Create link, verify URL format and decryption
5. **Test Settings**: Modify keywords, verify persistence and rescanning

### Debugging

- **Background Script**: `chrome://extensions/` → Click "service worker" → Console
- **Content Script**: Right-click page → Inspect → Console → Look for `[Vanish]` logs
- **Popup**: Right-click extension icon → Inspect popup

## API Reference

### Messages (Content Script ↔ Background)

```javascript
// Get current token
chrome.runtime.sendMessage({ type: 'GET_TOKEN' })
// Returns: { token: "...", error: "..." }

// Refresh token
chrome.runtime.sendMessage({ type: 'REFRESH_TOKEN' })

// Get API base URL
chrome.runtime.sendMessage({ type: 'GET_API_BASE' })
// Returns: { apiBase: "http://localhost:8080/api" }
```

### Messages (Popup ↔ Content Script)

```javascript
// Get detected matches
chrome.tabs.sendMessage(tabId, { type: 'GET_MATCHES' })
// Returns: { matches: [...] }

// Rescan page
chrome.tabs.sendMessage(tabId, { type: 'RESCAN_PAGE' })

// Reload settings
chrome.tabs.sendMessage(tabId, { type: 'RELOAD_SETTINGS' })
```

## Known Issues

- Extension requires Vanish tab to be open for authentication
- Highlighting may fail on dynamically generated content
- Regex patterns with high complexity may cause performance issues

## Future Enhancements

- Right-click context menu: "Create Vanish Link for Selection"
- Keyboard shortcut support
- ML-based secret detection
- Desktop notifications for detected secrets
- Export/import keyword configurations

## License

Same as Vanish project.

## Support

For issues or questions:
- GitHub Issues: https://github.com/milkiss/vanish/issues
- Documentation: See main Vanish README
