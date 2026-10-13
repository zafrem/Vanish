// @vitest-environment node
import { describe, test, expect, beforeAll, afterAll, vi } from 'vitest';
import { webcrypto } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { importKey, decryptBytes } from '../src/lib/crypto';
import {
  MAX_FILE_BYTES, sanitizeFilename, packEnvelope, packFile, unpackEnvelope,
  validateFileForSend, formatBytes,
} from '../src/lib/fileEnvelope';

const vector = JSON.parse(
  readFileSync(fileURLToPath(new URL('../../shared/crypto/testdata/vector.json', import.meta.url)), 'utf8')
);

beforeAll(() => vi.stubGlobal('crypto', webcrypto));
afterAll(() => vi.unstubAllGlobals());

const b64 = (s) => Uint8Array.from(Buffer.from(s, 'base64'));
const enc = (s) => new TextEncoder().encode(s);

describe('envelope', () => {
  test('round-trips binary data with NUL and newlines', () => {
    const data = new Uint8Array([0, 10, 255, 0, 10]);
    const packed = packEnvelope('비밀 문서.bin', 'application/octet-stream', data);
    const out = unpackEnvelope(packed);
    expect(out.kind).toBe('file');
    expect(out.name).toBe('비밀 문서.bin');
    expect(out.type).toBe('application/octet-stream');
    expect(Array.from(out.data)).toEqual(Array.from(data));
  });

  test('empty type falls back to application/octet-stream', () => {
    const out = unpackEnvelope(packEnvelope('a', '', new Uint8Array([1])));
    expect(out.type).toBe('application/octet-stream');
  });

  test('size limits: 0 rejected, exactly max accepted, max+1 rejected', () => {
    expect(() => packEnvelope('a', 't', new Uint8Array(0))).toThrow(/empty/i);
    expect(() => packEnvelope('a', 't', new Uint8Array(MAX_FILE_BYTES))).not.toThrow();
    expect(() => packEnvelope('a', 't', new Uint8Array(MAX_FILE_BYTES + 1))).toThrow(/too large/i);
  });

  test('plain text is returned as text', () => {
    for (const s of ['hello', '', 'こんにちは 🔒', 'multi\nline']) {
      expect(unpackEnvelope(enc(s))).toEqual({ kind: 'text', text: s });
    }
  });

  test('magic prefix without a valid header is treated as text', () => {
    const magic = 'VNSHFILE1\n';
    const cases = [
      magic,
      magic + '{"name":"a","type":"b","size":1}',
      magic + 'not json\nrest',
      magic + '{}\nx',
      magic + '{"name":"a","type":"b"}\nx',
      magic + '{"name":"a","type":"b","size":-1}\nx',
      magic + 'x'.repeat(5000) + '\nrest',
      magic + '{"name":1,"type":"b","size":1}\nx',
    ];
    for (const s of cases) {
      expect(unpackEnvelope(enc(s))).toEqual({ kind: 'text', text: s });
    }
  });

  test('valid header with wrong size is an error', () => {
    const bad = enc('VNSHFILE1\n{"name":"a","type":"b","size":5}\nabc');
    expect(() => unpackEnvelope(bad)).toThrow(/size/i);
  });

  test('packFile reads a File', async () => {
    const file = new File([new Uint8Array([1, 2, 3])], 'x.bin', { type: 'application/x-test' });
    const out = unpackEnvelope(await packFile(file));
    expect(out.name).toBe('x.bin');
    expect(out.type).toBe('application/x-test');
    expect(Array.from(out.data)).toEqual([1, 2, 3]);
  });
});

describe('validateFileForSend / formatBytes', () => {
  test('validation messages', () => {
    expect(validateFileForSend({ size: 0 })).toMatch(/empty/i);
    expect(validateFileForSend({ size: MAX_FILE_BYTES + 1 })).toMatch(/5 MB/);
    expect(validateFileForSend({ size: MAX_FILE_BYTES })).toBeNull();
    expect(validateFileForSend({ size: 1 })).toBeNull();
  });

  test('formatBytes', () => {
    expect(formatBytes(0)).toBe('0 B');
    expect(formatBytes(1536)).toBe('1.5 KB');
    expect(formatBytes(5 * 1024 * 1024)).toBe('5.0 MB');
  });
});

describe('sanitizeFilename (shared cases)', () => {
  test.each(vector.sanitize.map((c) => [c.in, c.want]))('%j -> %j', (input, want) => {
    expect(sanitizeFilename(input)).toBe(want);
  });

  test('never returns path separators or control characters', () => {
    for (const s of ['..\\..\\x', '/etc/passwd', 'a\u0000b', '‮', '.', ' ']) {
      const out = sanitizeFilename(s);
      expect(out).not.toMatch(/[\\/\u0000-\u001f]/);
      expect(out.length).toBeGreaterThan(0);
    }
  });
});

describe('unpackEnvelope (shared cases)', () => {
  test('vector has unpack cases', () => {
    expect(vector.unpack.length).toBeGreaterThan(0);
  });

  test.each(vector.unpack.map((c) => [c.name, c]))('%s', (_label, c) => {
    const input = b64(c.input_b64);
    if (c.kind === 'error') {
      expect(() => unpackEnvelope(input)).toThrow('Corrupt file: size does not match header');
      return;
    }
    const out = unpackEnvelope(input);
    if (c.kind === 'text') {
      expect(out).toEqual({ kind: 'text', text: new TextDecoder().decode(input) });
      return;
    }
    expect(c.kind).toBe('file');
    expect(out.kind).toBe('file');
    expect(out.name).toBe(c.file_name);
    expect(out.type).toBe(c.file_type);
    expect(Array.from(out.data)).toEqual(Array.from(b64(c.data_b64)));
  });
});

describe('packEnvelope header bound', () => {
  const emptyHeader = '{"name":"","type":"t","size":1}';

  test('rejects a name or type that makes the header exceed 4096 bytes', () => {
    const one = new Uint8Array([1]);
    expect(() => packEnvelope('n'.repeat(5000), 't', one)).toThrow('File name or type is too long');
    expect(() => packEnvelope('a', 't'.repeat(5000), one)).toThrow('File name or type is too long');
    const fits = 'n'.repeat(4096 - emptyHeader.length);
    expect(unpackEnvelope(packEnvelope(fits, 't', one)).name).toBe(fits);
    expect(() => packEnvelope(fits + 'n', 't', one)).toThrow('File name or type is too long');
  });
});

describe('Go interoperability (shared vector)', () => {
  test('decrypts and unpacks the file message produced by Go', async () => {
    const key = await importKey(vector.key_url);
    const plain = await decryptBytes(vector.file_ciphertext, vector.iv, key);
    const out = unpackEnvelope(plain);
    expect(out.kind).toBe('file');
    expect(out.name).toBe(vector.file_name);
    expect(out.type).toBe(vector.file_type);
    expect(Array.from(out.data)).toEqual(Array.from(b64(vector.file_data_b64)));
  });

  test('decrypts the text message produced by Go', async () => {
    const [ct, iv] = vector.text_ciphertext.split('|');
    const key = await importKey(vector.key_url);
    const out = unpackEnvelope(await decryptBytes(ct, iv, key));
    expect(out).toEqual({ kind: 'text', text: vector.text_plain });
  });
});
