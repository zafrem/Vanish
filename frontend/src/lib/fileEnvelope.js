/**
 * File envelope: how a file is carried inside an ordinary encrypted message.
 *
 *   VNSHFILE1\n  {"name":..,"type":..,"size":..}\n  <raw bytes>
 *
 * Mirrors shared/crypto/envelope.go - keep the two in sync. The envelope is
 * the plaintext that gets encrypted, so the server never sees name or type.
 */

export const MAX_FILE_BYTES = 5 * 1024 * 1024;
export const ENVELOPE_MAGIC = 'VNSHFILE1\n';

const MAGIC_BYTES = new TextEncoder().encode(ENVELOPE_MAGIC);
const MAX_HEADER_BYTES = 4096;
const MAX_FILENAME_BYTES = 255;
const DEFAULT_FILENAME = 'vanish-file';
const DEFAULT_TYPE = 'application/octet-stream';

/**
 * Reduce a sender-supplied name to a safe base name (same rules as Go).
 * @param {string} name
 * @returns {string}
 */
export function sanitizeFilename(name) {
  let s = String(name ?? '');
  const slash = Math.max(s.lastIndexOf('/'), s.lastIndexOf('\\'));
  if (slash >= 0) s = s.slice(slash + 1);

  s = Array.from(s)
    .filter((ch) => {
      const c = ch.codePointAt(0);
      const surrogate = c >= 0xd800 && c <= 0xdfff;
      return !(c < 0x20 || c === 0x7f || c === 0xfffd || surrogate || ':*?"<>|'.includes(ch));
    })
    .join('');

  const trim = (v) => v.replace(/^[ .]+|[ .]+$/g, '');
  s = trim(s);
  s = trim(truncateUtf8(s, MAX_FILENAME_BYTES));
  return s || DEFAULT_FILENAME;
}

function truncateUtf8(s, maxBytes) {
  const encoder = new TextEncoder();
  let out = '';
  let used = 0;
  for (const ch of s) {
    const len = encoder.encode(ch).length;
    if (used + len > maxBytes) break;
    out += ch;
    used += len;
  }
  return out;
}

/**
 * Message for an unusable file, or null when it can be sent.
 * @param {{size: number}} file
 * @returns {string|null}
 */
export function validateFileForSend(file) {
  if (!file || file.size === 0) return 'File is empty';
  if (file.size > MAX_FILE_BYTES) return 'File is too large (max 5 MB)';
  return null;
}

export function formatBytes(n) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

/**
 * Build the plaintext envelope for a file.
 * @param {string} name
 * @param {string} type
 * @param {Uint8Array} bytes
 * @returns {Uint8Array}
 */
export function packEnvelope(name, type, bytes) {
  const problem = validateFileForSend({ size: bytes.length });
  if (problem) throw new Error(problem);

  const header = new TextEncoder().encode(
    JSON.stringify({ name, type: type || DEFAULT_TYPE, size: bytes.length }) + '\n'
  );
  const out = new Uint8Array(MAGIC_BYTES.length + header.length + bytes.length);
  out.set(MAGIC_BYTES, 0);
  out.set(header, MAGIC_BYTES.length);
  out.set(bytes, MAGIC_BYTES.length + header.length);
  return out;
}

/**
 * @param {File} file
 * @returns {Promise<Uint8Array>}
 */
export async function packFile(file) {
  const problem = validateFileForSend(file);
  if (problem) throw new Error(problem);
  const bytes = new Uint8Array(await file.arrayBuffer());
  return packEnvelope(file.name, file.type, bytes);
}

function startsWithMagic(bytes) {
  if (bytes.length < MAGIC_BYTES.length) return false;
  for (let i = 0; i < MAGIC_BYTES.length; i++) {
    if (bytes[i] !== MAGIC_BYTES[i]) return false;
  }
  return true;
}

function parseHeader(rest) {
  const nl = rest.indexOf(10);
  if (nl < 0 || nl > MAX_HEADER_BYTES) return null;
  try {
    const h = JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(rest.subarray(0, nl)));
    if (
      h && typeof h.name === 'string' && typeof h.type === 'string' &&
      Number.isInteger(h.size) && h.size >= 0
    ) {
      return { header: h, dataStart: nl + 1 };
    }
  } catch {
    // not an envelope header
  }
  return null;
}

/**
 * Decode decrypted plaintext. Anything that is not a well-formed envelope is
 * text. A well-formed header whose size does not match the data is an error.
 * `name` is sender-supplied: run it through sanitizeFilename before use.
 * @param {Uint8Array} bytes
 * @returns {{kind:'file',name:string,type:string,data:Uint8Array}|{kind:'text',text:string}}
 */
export function unpackEnvelope(bytes) {
  if (startsWithMagic(bytes)) {
    const rest = bytes.subarray(MAGIC_BYTES.length);
    const parsed = parseHeader(rest);
    if (parsed) {
      const data = rest.subarray(parsed.dataStart);
      if (data.length !== parsed.header.size) {
        throw new Error('Corrupt file: size does not match header');
      }
      return { kind: 'file', name: parsed.header.name, type: parsed.header.type, data };
    }
  }
  return { kind: 'text', text: new TextDecoder().decode(bytes) };
}
