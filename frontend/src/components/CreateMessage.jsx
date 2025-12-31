import React, { useState, useEffect } from 'react';
import { generateKey, exportKey, encrypt } from '../lib/crypto';
import { createMessage, getUsers } from '../lib/api';
import { generateShareableURL } from '../utils/urlHelpers';
import { copyToClipboard } from '../lib/clipboard';
import { useAuth } from '../context/AuthContext';

/**
 * CreateMessage component for composing and encrypting secrets
 */
export default function CreateMessage() {
  const [secretText, setSecretText] = useState('');
  const [recipientId, setRecipientId] = useState('');
  const [ttl, setTTL] = useState(86400); // 24 hours default
  const [isCreating, setIsCreating] = useState(false);
  const [shareableURL, setShareableURL] = useState(null);
  const [error, setError] = useState(null);
  const [copied, setCopied] = useState(false);
  const [users, setUsers] = useState([]);
  const [loadingUsers, setLoadingUsers] = useState(true);
  const [searchTerm, setSearchTerm] = useState('');
  const [isDropdownOpen, setIsDropdownOpen] = useState(false);
  const [selectedUser, setSelectedUser] = useState(null);
  const { user } = useAuth();

  useEffect(() => {
    // Fetch list of users for recipient selection
    async function fetchUsers() {
      try {
        const userList = await getUsers();
        // Filter out current user from recipient list
        const otherUsers = userList.filter(u => u.id !== user.id);
        setUsers(otherUsers);
      } catch (err) {
        setError('Failed to load users');
      } finally {
        setLoadingUsers(false);
      }
    }

    fetchUsers();
  }, [user]);

  // Click outside detection to close dropdown
  useEffect(() => {
    const handleClickOutside = (e) => {
      if (!e.target.closest('.recipient-selector')) {
        setIsDropdownOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  // Filter users based on search term
  const filteredUsers = users.filter(user => {
    if (!searchTerm.trim()) return true;
    const search = searchTerm.toLowerCase();
    return (
      user.name.toLowerCase().includes(search) ||
      user.email.toLowerCase().includes(search)
    );
  });

  // Handle recipient selection
  const handleSelectUser = (user) => {
    setSelectedUser(user);
    setRecipientId(user.id.toString());
    setSearchTerm('');
    setIsDropdownOpen(false);
  };

  // Clear selected user
  const handleClearSelection = () => {
    setSelectedUser(null);
    setRecipientId('');
    setSearchTerm('');
    setIsDropdownOpen(true);
  };

  const handleCreate = async (e) => {
    e.preventDefault();
    setError(null);
    setIsCreating(true);

    try {
      // Validation
      if (!secretText.trim()) {
        throw new Error('Please enter a secret message');
      }

      if (!recipientId) {
        throw new Error('Please select a recipient');
      }

      // Step 1: Generate encryption key (client-side)
      const key = await generateKey();
      const keyString = await exportKey(key);

      // Step 2: Encrypt the message (client-side)
      const { ciphertext, iv } = await encrypt(secretText, key);

      // Step 3: Send encrypted data to server with recipient ID and encryption key
      const response = await createMessage(ciphertext, iv, parseInt(recipientId), keyString, ttl);

      // Step 4: Generate shareable URL with key in fragment
      const url = generateShareableURL(response.id, keyString);
      setShareableURL(url);

      // Clear the input
      setSecretText('');
      setRecipientId('');
      setSelectedUser(null);
      setSearchTerm('');
    } catch (err) {
      setError(err.message);
    } finally {
      setIsCreating(false);
    }
  };

  const handleCopyURL = async () => {
    if (!shareableURL) return;

    const result = await copyToClipboard(shareableURL);
    if (result.success) {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } else {
      setError('Failed to copy URL');
    }
  };

  const handleReset = () => {
    setShareableURL(null);
    setError(null);
    setCopied(false);
  };

  if (shareableURL) {
    return (
      <div className="w-full max-w-2xl mx-auto">
        <div className="bg-dark-card border border-dark-border rounded-lg p-8 shadow-2xl">
          <div className="text-center mb-6">
            <div className="text-6xl mb-4">🔗</div>
            <h2 className="text-2xl font-bold text-green-400 mb-2">Secret Created!</h2>
            <p className="text-gray-400 text-sm">Share this link securely with the recipient</p>
          </div>

          <div className="bg-slate-900 rounded-lg p-4 mb-4 break-all font-mono text-sm">
            {shareableURL}
          </div>

          <div className="space-y-3">
            <button
              onClick={handleCopyURL}
              className="w-full bg-blue-600 hover:bg-blue-700 text-white font-semibold py-3 px-6 rounded-lg transition duration-200"
            >
              {copied ? '✓ Copied!' : 'Copy Link'}
            </button>

            <button
              onClick={handleReset}
              className="w-full bg-dark-border hover:bg-slate-600 text-white font-semibold py-3 px-6 rounded-lg transition duration-200"
            >
              Create Another Secret
            </button>
          </div>

          <div className="mt-6 text-xs text-gray-500 space-y-1">
            <p>⚠️ This link can only be opened once</p>
            <p>🔥 The secret will be permanently destroyed after reading</p>
            <p>⏰ Expires automatically in {formatTTL(ttl)}</p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="w-full max-w-2xl mx-auto">
      <div className="bg-dark-card border border-dark-border rounded-lg p-8 shadow-2xl">
        <div className="text-center mb-6">
          <div className="text-6xl mb-4">🔒</div>
          <h2 className="text-2xl font-bold mb-2">Create Secret Message</h2>
          <p className="text-gray-400 text-sm">
            Enter sensitive information to share securely
          </p>
        </div>

        <form onSubmit={handleCreate} className="space-y-4">
          <div className="recipient-selector">
            <label className="block text-sm font-medium text-gray-300 mb-2">
              Recipient
            </label>
            {loadingUsers ? (
              <div className="text-gray-400 text-sm">Loading users...</div>
            ) : (
              <div className="relative">
                {/* Search Input */}
                <div className="relative">
                  <input
                    type="text"
                    value={selectedUser ? `${selectedUser.name} (${selectedUser.email})` : searchTerm}
                    onChange={(e) => {
                      setSearchTerm(e.target.value);
                      setSelectedUser(null);
                      setRecipientId('');
                    }}
                    onFocus={() => setIsDropdownOpen(true)}
                    placeholder="Search by name or email..."
                    className="w-full bg-slate-900 border border-dark-border rounded-lg px-4 py-3 pr-10 text-gray-100 placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
                    disabled={isCreating}
                  />
                  {/* Clear button */}
                  {selectedUser && !isCreating && (
                    <button
                      type="button"
                      onClick={handleClearSelection}
                      className="absolute right-3 top-1/2 transform -translate-y-1/2 text-gray-400 hover:text-gray-200 text-2xl leading-none"
                    >
                      ×
                    </button>
                  )}
                </div>

                {/* Dropdown List */}
                {isDropdownOpen && !isCreating && (
                  <div className="absolute z-10 w-full mt-1 bg-slate-900 border border-dark-border rounded-lg shadow-lg max-h-60 overflow-y-auto">
                    {filteredUsers.length > 0 ? (
                      filteredUsers.map((u) => (
                        <div
                          key={u.id}
                          onClick={() => handleSelectUser(u)}
                          className="px-4 py-3 hover:bg-slate-700 cursor-pointer border-b border-dark-border last:border-b-0 transition"
                        >
                          <span className="text-gray-100">{u.name}</span>
                          <span className="text-gray-400 text-sm ml-2">({u.email})</span>
                        </div>
                      ))
                    ) : (
                      <div className="px-4 py-3 text-gray-400 text-sm">
                        No users found
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-300 mb-2">
              Secret Message
            </label>
            <textarea
              value={secretText}
              onChange={(e) => setSecretText(e.target.value)}
              placeholder="Enter password, API key, or sensitive data..."
              rows={6}
              className="w-full bg-slate-900 border border-dark-border rounded-lg px-4 py-3 text-gray-100 placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none"
              disabled={isCreating}
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-300 mb-2">
              Expires In
            </label>
            <select
              value={ttl}
              onChange={(e) => setTTL(Number(e.target.value))}
              className="w-full bg-slate-900 border border-dark-border rounded-lg px-4 py-3 text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
              disabled={isCreating}
            >
              <option value={3600}>1 Hour</option>
              <option value={21600}>6 Hours</option>
              <option value={86400}>24 Hours (Recommended)</option>
              <option value={259200}>3 Days</option>
              <option value={604800}>7 Days</option>
            </select>
          </div>

          {error && (
            <div className="bg-red-900/30 border border-red-500 text-red-300 px-4 py-3 rounded-lg text-sm">
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={isCreating || !secretText.trim() || !recipientId}
            className="w-full bg-gradient-to-r from-red-500 to-orange-500 hover:from-red-600 hover:to-orange-600 text-white font-semibold py-3 px-6 rounded-lg transition duration-200 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isCreating ? 'Creating...' : 'Create Secret Link'}
          </button>
        </form>

        <div className="mt-6 text-xs text-gray-500 space-y-1">
          <p>🔐 Encrypted client-side before transmission</p>
          <p>👤 Only the selected recipient can read this message</p>
          <p>🚫 Server never sees your plaintext message</p>
          <p>🔥 One-time access with automatic destruction</p>
        </div>
      </div>
    </div>
  );
}

function formatTTL(seconds) {
  if (seconds < 3600) return `${seconds / 60} minutes`;
  if (seconds < 86400) return `${seconds / 3600} hours`;
  return `${seconds / 86400} days`;
}
