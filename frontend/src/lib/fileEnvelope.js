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
  // Receivers treat an over-long header as text, so refuse to build one.
  if (header.length - 1 > MAX_HEADER_BYTES) {
    throw new Error('File name or type is too long');
  }
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

const REPLACEMENT_CHAR = String.fromCharCode(0xfffd);

// Replace lone UTF-16 surrogates with U+FFFD, as Go's JSON decoder does.
function toWellFormed(s) {
  let out = '';
  for (let i = 0; i < s.length; i++) {
    const c = s.charCodeAt(i);
    if (c >= 0xd800 && c <= 0xdbff && i + 1 < s.length) {
      const d = s.charCodeAt(i + 1);
      if (d >= 0xdc00 && d <= 0xdfff) {
        out += s[i] + s[i + 1];
        i++;
        continue;
      }
    }
    out += c >= 0xd800 && c <= 0xdfff ? REPLACEMENT_CHAR : s[i];
  }
  return out;
}

const own = (o, k) => Object.prototype.hasOwnProperty.call(o, k);

/*
 * Header rules, identical to Unpack in shared/crypto/envelope.go and pinned by
 * shared/crypto/testdata/vector.json:
 * - the header (bytes between the magic and the first \n, at most 4096) must
 *   be valid UTF-8 and a JSON object (a BOM is kept and makes JSON.parse fail);
 * - keys match exactly and case-sensitively; a duplicate key's last value wins;
 * - name and type must be strings;
 * - size must be an integral number (1.0 and 1e0 count as 1) in [0, MAX_FILE_BYTES].
 */
function parseHeader(rest) {
  const nl = rest.subarray(0, MAX_HEADER_BYTES + 1).indexOf(10);
  if (nl < 0) return null;
  let h;
  try {
    h = JSON.parse(
      new TextDecoder('utf-8', { fatal: true, ignoreBOM: true }).decode(rest.subarray(0, nl))
    );
  } catch {
    return null; // not an envelope header
  }
  if (h === null || typeof h !== 'object' || Array.isArray(h)) return null;
  if (!own(h, 'name') || !own(h, 'type') || !own(h, 'size')) return null;
  const { name, type, size } = h;
  if (typeof name !== 'string' || typeof type !== 'string') return null;
  if (typeof size !== 'number' || !Number.isInteger(size) || size < 0 || size > MAX_FILE_BYTES) {
    return null;
  }
  return { header: { name: toWellFormed(name), type: toWellFormed(type), size }, dataStart: nl + 1 };
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
