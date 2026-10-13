import React, { useState, useEffect, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { importKey, decryptBytes } from '../lib/crypto';
import { getMessage, checkMessageExists } from '../lib/api';
import { extractKeyFromURL } from '../utils/urlHelpers';
import { copyToClipboard, clearSensitiveData } from '../lib/clipboard';
import { unpackEnvelope, sanitizeFilename } from '../lib/fileEnvelope';
import { saveBlob } from '../lib/download';

/**
 * RetrieveMessage component for viewing and burning secrets
 * CRITICAL: Never renders the secret to DOM - copies directly to clipboard
 */
export default function RetrieveMessage() {
  const { id } = useParams();
  const navigate = useNavigate();

  const [messageExists, setMessageExists] = useState(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isBurning, setIsBurning] = useState(false);
  const [isBurned, setIsBurned] = useState(false);
  const [error, setError] = useState(null);
  const [burnedInfo, setBurnedInfo] = useState(null); // { kind: 'text' } | { kind: 'file', name }
  const resaveRef = useRef(null); // closure over the decrypted file; never put in state

  useEffect(() => () => { resaveRef.current = null; }, []);

  useEffect(() => {
    checkMessage();
  }, [id]);

  const checkMessage = async () => {
    try {
      // Check if encryption key exists in URL fragment
      const key = extractKeyFromURL();
      if (!key) {
        setError('Invalid link: missing encryption key');
        setMessageExists(false);
        setIsLoading(false);
        return;
      }

      // Check if message exists without burning it
      const exists = await checkMessageExists(id);
      setMessageExists(exists);

      if (!exists) {
        setError('Message not found or already burned');
      }
    } catch (err) {
      setError(err.message);
      setMessageExists(false);
    } finally {
      setIsLoading(false);
    }
  };

  /**
   * Handle the "Copy & Burn" action
   * CRITICAL SECURITY: Secret is decrypted in closure and copied directly
   * to clipboard WITHOUT ever entering React state or DOM
   */
  const handleCopyAndBurn = async () => {
    setError(null);
    setIsBurning(true);

    // Use closure to keep secret out of state
    let decryptedSecret = null;
    let encryptionKey = null;

    try {
      // Step 1: Get encryption key from URL fragment (client-side only)
      const keyString = extractKeyFromURL();
      if (!keyString) {
        throw new Error('Encryption key missing from URL');
      }

      // Step 2: Import the key
      encryptionKey = await importKey(keyString);

      // Step 3: Fetch encrypted message from server (burns it atomically)
      const { ciphertext, iv } = await getMessage(id);

      // Step 4: Decrypt in memory (NOT in state - stays in closure)
      const bytes = await decryptBytes(ciphertext, iv, encryptionKey);
      const payload = unpackEnvelope(bytes);

      if (payload.kind === 'file') {
        // Step 5a: Save the file straight to disk without rendering it
        const safeName = sanitizeFilename(payload.name);
        resaveRef.current = saveBlob(payload.data, safeName);
        setBurnedInfo({ kind: 'file', name: safeName });
      } else {
        // Step 5b: Copy text directly to clipboard without rendering
        decryptedSecret = payload.text;
        const result = await copyToClipboard(decryptedSecret);

        if (!result.success) {
          throw new Error('Failed to copy to clipboard: ' + (result.error || 'Unknown error'));
        }
        setBurnedInfo({ kind: 'text' });
      }

      // Success! Message is burned and delivered
      setIsBurned(true);

      // Clear sensitive data from memory (best effort)
      decryptedSecret = null;
      encryptionKey = null;

      // Clear URL fragment to remove key from address bar
      window.history.replaceState(null, '', window.location.pathname);
    } catch (err) {
      setError(err.message);
    } finally {
      // Ensure cleanup even on error
      if (decryptedSecret) {
        clearSensitiveData(decryptedSecret);
        decryptedSecret = null;
      }
      if (encryptionKey) {
        clearSensitiveData(encryptionKey);
        encryptionKey = null;
      }

      setIsBurning(false);
    }
  };

  if (isLoading) {
    return (
      <div className="w-full max-w-md mx-auto">
        <div className="bg-dark-card border border-dark-border rounded-lg p-8 shadow-2xl text-center">
          <div className="text-6xl mb-4 animate-pulse">🔍</div>
          <p className="text-gray-400">Loading message...</p>
        </div>
      </div>
    );
  }

  if (isBurned) {
    return (
      <div className="w-full max-w-md mx-auto">
        <div className="bg-dark-card border border-dark-border rounded-lg p-8 shadow-2xl text-center">
          <div className="text-6xl mb-4 animate-burn">🔥</div>
          <h2 className="text-2xl font-bold text-orange-400 mb-4">Message Burned!</h2>
          {burnedInfo && burnedInfo.kind === 'file' ? (
            <>
              <p className="text-gray-300 mb-2">
                File downloaded as <span className="font-mono break-all">{burnedInfo.name}</span>
              </p>
              <p className="text-sm text-gray-500 mb-6">
                The message has been permanently destroyed
              </p>
              <div className="bg-yellow-900/30 border border-yellow-600 text-yellow-300 px-4 py-3 rounded-lg text-sm mb-4">
                ⚠️ Didn't get the file? Save it now - it can't be fetched again
              </div>
              <button
                onClick={() => resaveRef.current && resaveRef.current()}
                className="w-full bg-slate-700 hover:bg-slate-600 text-white font-semibold py-3 px-6 rounded-lg transition duration-200 mb-3"
              >
                Save again
              </button>
            </>
          ) : (
            <>
              <p className="text-gray-300 mb-2">Secret copied to your clipboard</p>
              <p className="text-sm text-gray-500 mb-6">
                The message has been permanently destroyed
              </p>
              <div className="bg-yellow-900/30 border border-yellow-600 text-yellow-300 px-4 py-3 rounded-lg text-sm mb-4">
                ⚠️ Paste the secret now - it's not stored anywhere
              </div>
            </>
          )}

          <button
            onClick={() => navigate('/')}
            className="w-full bg-dark-border hover:bg-slate-600 text-white font-semibold py-3 px-6 rounded-lg transition duration-200"
          >
            Create Your Own Secret
          </button>
        </div>
      </div>
    );
  }

  if (!messageExists || error) {
    return (
      <div className="w-full max-w-md mx-auto">
        <div className="bg-dark-card border border-dark-border rounded-lg p-8 shadow-2xl text-center">
          <div className="text-6xl mb-4">❌</div>
          <h2 className="text-2xl font-bold text-red-400 mb-4">Message Not Found</h2>
          <p className="text-gray-400 mb-6">
            {error || 'This message does not exist or has already been read'}
          </p>

          <button
            onClick={() => navigate('/')}
            className="w-full bg-dark-border hover:bg-slate-600 text-white font-semibold py-3 px-6 rounded-lg transition duration-200"
          >
            Create a Secret
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="w-full max-w-md mx-auto">
      <div className="bg-dark-card border border-dark-border rounded-lg p-8 shadow-2xl text-center">
        <div className="text-6xl mb-4">🔒</div>
        <h2 className="text-2xl font-bold mb-4">Secret Awaits</h2>
        <p className="text-gray-400 mb-6">
          Click the button below to reveal the secret. Text is copied to your clipboard; a file is downloaded.
        </p>

        <div className="bg-red-900/30 border border-red-500 text-red-300 px-4 py-3 rounded-lg text-sm mb-6 space-y-2">
          <p className="font-semibold">⚠️ Warning:</p>
          <p>This message will be permanently destroyed after you click the button</p>
          <p>Make sure you're ready to save it</p>
        </div>

        {error && (
          <div className="bg-red-900/30 border border-red-500 text-red-300 px-4 py-3 rounded-lg text-sm mb-4">
            {error}
          </div>
        )}

        <button
          onClick={handleCopyAndBurn}
          disabled={isBurning}
          className="w-full bg-gradient-to-r from-red-500 to-orange-500 hover:from-red-600 hover:to-orange-600 text-white font-bold py-4 px-6 rounded-lg transition duration-200 text-lg disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {isBurning ? (
            <span className="flex items-center justify-center">
              <svg className="animate-spin -ml-1 mr-3 h-5 w-5 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              Decrypting & Burning...
            </span>
          ) : (
            '🔥 Reveal & Destroy Message'
          )}
        </button>

        <div className="mt-6 text-xs text-gray-500 space-y-1">
          <p>🔐 Decryption happens locally in your browser</p>
          <p>👁️ Secrets are NOT displayed on screen</p>
          <p>📋 Text goes to your clipboard; files are saved to your downloads</p>
        </div>
      </div>
    </div>
  );
}
