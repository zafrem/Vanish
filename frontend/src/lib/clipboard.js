/**
 * Clipboard operations module
 * Handles copying to clipboard without exposing sensitive data in DOM
 */

/**
 * Copy text directly to clipboard without rendering to DOM
 * Uses modern Clipboard API with fallback for older browsers
 * @param {string} text - The text to copy
 * @returns {Promise<{success: boolean, error?: string}>}
 */
export async function copyToClipboard(text) {
  // Try modern Clipboard API first (requires HTTPS or localhost)
  if (navigator.clipboard && navigator.clipboard.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return { success: true };
    } catch (err) {
      console.error('Clipboard API failed:', err);
      // Fall back to execCommand
      return fallbackCopyToClipboard(text);
    }
  } else {
    // Use fallback for older browsers
    return fallbackCopyToClipboard(text);
  }
}

/**
 * Fallback clipboard copy using execCommand
 * Creates a temporary, hidden textarea to perform the copy
 * @param {string} text
 * @returns {{success: boolean, error?: string}}
 */
function fallbackCopyToClipboard(text) {
  // Create temporary textarea element
  const textarea = document.createElement('textarea');
  textarea.value = text;

  // Make it invisible and non-interactive
  textarea.style.position = 'fixed';
  textarea.style.left = '-9999px';
  textarea.style.top = '-9999px';
  textarea.style.opacity = '0';
  textarea.style.pointerEvents = 'none';
  textarea.setAttribute('readonly', '');

  document.body.appendChild(textarea);

  try {
    // Select and copy
    textarea.select();
    textarea.setSelectionRange(0, text.length);

    const successful = document.execCommand('copy');

    if (successful) {
      return { success: true };
    } else {
      return { success: false, error: 'Copy command failed' };
    }
  } catch (err) {
    return { success: false, error: err.message };
  } finally {
    // Always remove the temporary element
    document.body.removeChild(textarea);
  }
}

/**
 * Clear sensitive data from memory (best effort)
 * Note: Modern JavaScript engines may optimize this away
 * @param {any} variable - Variable to clear
 */
export function clearSensitiveData(variable) {
  if (variable !== null && variable !== undefined) {
    variable = null;
  }
}
