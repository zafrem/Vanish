import React, { useState, useEffect, useRef } from 'react';
import { generateKey, exportKey, encrypt, encryptBytes } from '../lib/crypto';
import { packFile, validateFileForSend, formatBytes } from '../lib/fileEnvelope';
import { createMessage, getUsers, sendSlackNotification, sendEmailNotification } from '../lib/api';
import { generateShareableURL } from '../utils/urlHelpers';
import { copyToClipboard } from '../lib/clipboard';
import { useAuth } from '../context/AuthContext';

/**
 * CreateMessage component for composing and encrypting secrets
 */
export default function CreateMessage() {
  const [secretText, setSecretText] = useState('');
  const [mode, setMode] = useState('text'); // 'text' | 'file'
  const [file, setFile] = useState(null);
  const [isDragging, setIsDragging] = useState(false);
  const fileInputRef = useRef(null);
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
  const [notificationStatus, setNotificationStatus] = useState(null); // 'slack_sent', 'email_sent', 'error'
  const [isSendingNotification, setIsSendingNotification] = useState(false);
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

  const acceptFile = (candidate) => {
    if (!candidate) return;
    const problem = validateFileForSend(candidate);
    if (problem) {
      setFile(null);
      setError(problem);
      return;
    }
    setError(null);
    setFile(candidate);
  };

  const handleFileSelected = (e) => acceptFile(e.target.files && e.target.files[0]);

  const handleDrop = (e) => {
    e.preventDefault();
    setIsDragging(false);
    if (isCreating) return;
    acceptFile(e.dataTransfer.files && e.dataTransfer.files[0]);
  };

  const handleRemoveFile = () => {
    setFile(null);
    if (fileInputRef.current) fileInputRef.current.value = '';
  };

  const switchMode = (next) => {
    setMode(next);
    setError(null);
  };

  const handleCreate = async (e) => {
    e.preventDefault();
    setError(null);
    setIsCreating(true);

    try {
      // Validation
      if (mode === 'text' && !secretText.trim()) {
        throw new Error('Please enter a secret message');
      }

      if (mode === 'file' && !file) {
        throw new Error('Please choose a file');
      }

      if (!recipientId) {
        throw new Error('Please select a recipient');
      }

      // Step 1: Generate encryption key (client-side)
      const key = await generateKey();
      const keyString = await exportKey(key);

      // Step 2: Encrypt the message or file (client-side)
      const { ciphertext, iv } = mode === 'file'
        ? await encryptBytes(await packFile(file), key)
        : await encrypt(secretText, key);

      // Step 3: Send encrypted data to server with recipient ID (the key never leaves the browser)
      const response = await createMessage(ciphertext, iv, parseInt(recipientId), ttl);

      // Step 4: Generate shareable URL with key in fragment
      const url = generateShareableURL(response.id, keyString);
      setShareableURL(url);

      // Clear the input
      setSecretText('');
      setFile(null);
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

  const handleSendSlack = async () => {
    setIsSendingNotification(true);
    setNotificationStatus(null);
    try {
      await sendSlackNotification(parseInt(recipientId), shareableURL);
      setNotificationStatus('slack_sent');
    } catch (err) {
      setError('Failed to send Slack notification: ' + err.message);
      setNotificationStatus('error');
    } finally {
      setIsSendingNotification(false);
    }
  };

  const handleSendEmail = async () => {
    setIsSendingNotification(true);
    setNotificationStatus(null);
    try {
      await sendEmailNotification(parseInt(recipientId), shareableURL);
      setNotificationStatus('email_sent');
    } catch (err) {
      setError('Failed to send Email notification: ' + err.message);
      setNotificationStatus('error');
    } finally {
      setIsSendingNotification(false);
    }
  };

  const handleReset = () => {
    setShareableURL(null);
    setError(null);
    setCopied(false);
    setRecipientId('');
    setSelectedUser(null);
    setNotificationStatus(null);
  };

  if (shareableURL) {
    return (
      <div className="w-full max-w-2xl mx-auto">
        <div className="bg-dark-card border border-dark-border rounded-lg p-8 shadow-2xl">
          <div className="text-center mb-6">
            <div className="text-6xl mb-4">🔗</div>
            <h2 className="text-2xl font-bold text-green-400 mb-2">Secret Created!</h2>
            <p className="text-gray-400 text-sm">Share this link securely with the recipient{mode === 'file' ? ' (it opens a file download)' : ''}</p>
          </div>

          <div className="bg-slate-900 rounded-lg p-4 mb-4 break-all font-mono text-sm">
            {shareableURL}
          </div>

          {error && (
            <div className="bg-red-900/30 border border-red-500 text-red-300 px-4 py-3 rounded-lg text-sm mb-4">
              {error}
            </div>
          )}

          {notificationStatus === 'slack_sent' && (
            <div className="bg-green-900/30 border border-green-500 text-green-300 px-4 py-3 rounded-lg text-sm mb-4 flex items-center justify-center">
              <span className="mr-2">✓</span> Notification sent via Slack!
            </div>
          )}

          {notificationStatus === 'email_sent' && (
            <div className="bg-green-900/30 border border-green-500 text-green-300 px-4 py-3 rounded-lg text-sm mb-4 flex items-center justify-center">
              <span className="mr-2">✓</span> Notification sent via Email!
            </div>
          )}

          <div className="grid grid-cols-2 gap-3 mb-3">
             <button
              onClick={handleSendSlack}
              disabled={isSendingNotification || notificationStatus === 'slack_sent'}
              className="bg-[#4A154B] hover:bg-[#611f69] text-white font-semibold py-3 px-6 rounded-lg transition duration-200 flex items-center justify-center disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {isSendingNotification ? 'Sending...' : 'Send via Slack'}
            </button>
            <button
              onClick={handleSendEmail}
              disabled={isSendingNotification || notificationStatus === 'email_sent'}
              className="bg-blue-600 hover:bg-blue-700 text-white font-semibold py-3 px-6 rounded-lg transition duration-200 flex items-center justify-center disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {isSendingNotification ? 'Sending...' : 'Send via Email'}
            </button>
          </div>

          <div className="space-y-3">
            <button
              onClick={handleCopyURL}
              className="w-full bg-slate-700 hover:bg-slate-600 text-white font-semibold py-3 px-6 rounded-lg transition duration-200"
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
            <div className="flex items-center justify-between mb-2">
              <label className="block text-sm font-medium text-gray-300">
                {mode === 'file' ? 'File' : 'Secret Message'}
              </label>
              <div className="inline-flex rounded-lg border border-dark-border overflow-hidden text-sm">
                <button
                  type="button"
                  onClick={() => switchMode('text')}
                  disabled={isCreating}
                  aria-pressed={mode === 'text'}
                  className={`px-3 py-1 ${mode === 'text' ? 'bg-slate-700 text-white' : 'text-gray-400 hover:bg-slate-800'}`}
                >
                  Text
                </button>
                <button
                  type="button"
                  onClick={() => switchMode('file')}
                  disabled={isCreating}
                  aria-pressed={mode === 'file'}
                  className={`px-3 py-1 ${mode === 'file' ? 'bg-slate-700 text-white' : 'text-gray-400 hover:bg-slate-800'}`}
                >
                  File
                </button>
              </div>
            </div>

            {mode === 'text' ? (
              <textarea
                value={secretText}
                onChange={(e) => setSecretText(e.target.value)}
                placeholder="Enter password, API key, or sensitive data..."
                rows={6}
                className="w-full bg-slate-900 border border-dark-border rounded-lg px-4 py-3 text-gray-100 placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none"
                disabled={isCreating}
              />
            ) : (
              <div>
                <input
                  id="secret-file"
                  ref={fileInputRef}
                  type="file"
                  className="sr-only"
                  onChange={handleFileSelected}
                  disabled={isCreating}
                />
                {file ? (
                  <div className="flex items-center justify-between bg-slate-900 border border-dark-border rounded-lg px-4 py-3">
                    <span className="text-gray-100 break-all">
                      {file.name} <span className="text-gray-400 text-sm">({formatBytes(file.size)})</span>
                    </span>
                    <button
                      type="button"
                      onClick={handleRemoveFile}
                      disabled={isCreating}
                      aria-label="Remove file"
                      className="ml-3 text-gray-400 hover:text-gray-200 text-2xl leading-none"
                    >
                      ×
                    </button>
                  </div>
                ) : (
                  <label
                    htmlFor="secret-file"
                    onDragOver={(e) => { e.preventDefault(); setIsDragging(true); }}
                    onDragLeave={() => setIsDragging(false)}
                    onDrop={handleDrop}
                    className={`block cursor-pointer text-center border-2 border-dashed rounded-lg px-4 py-8 text-gray-400 ${isDragging ? 'border-blue-500 bg-slate-800' : 'border-dark-border bg-slate-900'}`}
                  >
                    Drop a file here or click to choose a file (max 5 MB)
                  </label>
                )}
              </div>
            )}
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
            disabled={isCreating || !recipientId || (mode === 'file' ? !file : !secretText.trim())}
            className="w-full bg-gradient-to-r from-red-500 to-orange-500 hover:from-red-600 hover:to-orange-600 text-white font-semibold py-3 px-6 rounded-lg transition duration-200 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isCreating ? 'Creating...' : 'Create Secret Link'}
          </button>
        </form>

        <div className="mt-6 text-xs text-gray-500 space-y-1">
          <p>🔐 Encrypted client-side before transmission</p>
          <p>🔗 Share the link yourself - anyone who has it can read the message once</p>
          <p>🚫 Server never sees your plaintext message, file or key</p>
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
