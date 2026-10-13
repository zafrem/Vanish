/**
 * Save bytes as a file download without rendering them anywhere.
 * Returns a function that triggers the same download again (useful when a
 * browser blocked the first attempt because it ran after an await).
 * @param {Uint8Array} data
 * @param {string} name - already sanitized
 * @returns {() => void}
 */
export function saveBlob(data, name) {
  // Never use the sender's MIME type: a blob URL is same-origin with the app,
  // so a text/html or image/svg+xml blob that a browser renders could run
  // script with access to the session. octet-stream always downloads.
  const blob = new Blob([data], { type: 'application/octet-stream' });

  const save = () => {
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = name;
    a.style.display = 'none';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  };

  save();
  return save;
}
