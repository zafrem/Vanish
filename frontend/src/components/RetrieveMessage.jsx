import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { importKey, decrypt } from '../lib/crypto';
import { getMessage, checkMessageExists } from '../lib/api';
import { extractKeyFromURL } from '../utils/urlHelpers';
import { copyToClipboard, clearSensitiveData } from '../lib/clipboard';

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
      decryptedSecret = await decrypt(ciphertext, iv, encryptionKey);

      // Step 5: Copy directly to clipboard without rendering
      const result = await copyToClipboard(decryptedSecret);

      if (!result.success) {
        throw new Error('Failed to copy to clipboard: ' + (result.error || 'Unknown error'));
      }

      // Success! Message is burned and copied
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
          <p className="text-gray-300 mb-2">Secret copied to your clipboard</p>
          <p className="text-sm text-gray-500 mb-6">
            The message has been permanently destroyed
          </p>

          <div className="bg-yellow-900/30 border border-yellow-600 text-yellow-300 px-4 py-3 rounded-lg text-sm mb-4">
            ⚠️ Paste the secret now - it's not stored anywhere
          </div>

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
        <h2 className="text-2xl font-bold mb-4">Secret Message Awaits</h2>
        <p className="text-gray-400 mb-6">
          Click the button below to reveal and copy the secret
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
              Copying & Burning...
            </span>
          ) : (
            '🔥 Copy to Clipboard & Destroy Message'
          )}
        </button>

        <div className="mt-6 text-xs text-gray-500 space-y-1">
          <p>🔐 Decryption happens locally in your browser</p>
          <p>👁️ Secret will NOT be displayed on screen</p>
          <p>📋 It will be copied directly to your clipboard</p>
        </div>
      </div>
    </div>
  );
}
