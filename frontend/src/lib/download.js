/**
 * Save bytes as a file download without rendering them anywhere.
 * Returns a function that triggers the same download again (useful when a
 * browser blocked the first attempt because it ran after an await).
 * @param {Uint8Array} data
 * @param {string} name - already sanitized
 * @param {string} [type]
 * @returns {() => void}
 */
export function saveBlob(data, name, type = 'application/octet-stream') {
  const blob = new Blob([data], { type });

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
