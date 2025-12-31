import React, { useState, useEffect } from 'react';
import { getHistory } from '../lib/api';
import { useAuth } from '../context/AuthContext';
import { generateShareableURL } from '../utils/urlHelpers';
import { copyToClipboard } from '../lib/clipboard';

export default function MessageHistory() {
  const [history, setHistory] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [filter, setFilter] = useState('all'); // all, sent, received
  const { user } = useAuth();

  useEffect(() => {
    fetchHistory();
  }, []);

  const fetchHistory = async () => {
    try {
      setLoading(true);
      const data = await getHistory(50);
      setHistory(data || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const filteredHistory = history.filter(item => {
    if (filter === 'sent') return item.is_sender;
    if (filter === 'received') return item.is_recipient;
    return true;
  });

  const getStatusColor = (status) => {
    switch (status) {
      case 'read':
        return 'text-green-400 bg-green-900/30 border-green-500';
      case 'pending':
        return 'text-yellow-400 bg-yellow-900/30 border-yellow-500';
      case 'expired':
        return 'text-red-400 bg-red-900/30 border-red-500';
      default:
        return 'text-gray-400 bg-gray-900/30 border-gray-500';
    }
  };

  const getStatusIcon = (status) => {
    switch (status) {
      case 'read':
        return '✓';
      case 'pending':
        return '⏳';
      case 'expired':
        return '⌛';
      default:
        return '?';
    }
  };

  const formatDate = (dateString) => {
    const date = new Date(dateString);
    const now = new Date();
    const diffMs = now - date;
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return 'Just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    if (diffDays < 7) return `${diffDays}d ago`;

    return date.toLocaleDateString();
  };

  const handleCopyLink = async (messageId, encryptionKey) => {
    const url = generateShareableURL(messageId, encryptionKey);
    const result = await copyToClipboard(url);
    if (!result.success) {
      setError('Failed to copy link');
    }
  };

  if (loading) {
    return (
      <div className="w-full max-w-4xl mx-auto">
        <div className="bg-dark-card border border-dark-border rounded-lg p-8 text-center">
          <div className="text-gray-400">Loading message history...</div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="w-full max-w-4xl mx-auto">
        <div className="bg-red-900/30 border border-red-500 text-red-300 px-6 py-4 rounded-lg">
          Error loading history: {error}
        </div>
      </div>
    );
  }

  return (
    <div className="w-full max-w-4xl mx-auto">
      <div className="bg-dark-card border border-dark-border rounded-lg shadow-2xl">
        {/* Header */}
        <div className="border-b border-dark-border p-6">
          <h2 className="text-2xl font-bold mb-4">Message History</h2>

          {/* Filter Tabs */}
          <div className="flex gap-2">
            <button
              onClick={() => setFilter('all')}
              className={`px-4 py-2 rounded-lg font-medium transition ${
                filter === 'all'
                  ? 'bg-blue-600 text-white'
                  : 'bg-slate-800 text-gray-400 hover:bg-slate-700'
              }`}
            >
              All ({history.length})
            </button>
            <button
              onClick={() => setFilter('sent')}
              className={`px-4 py-2 rounded-lg font-medium transition ${
                filter === 'sent'
                  ? 'bg-blue-600 text-white'
                  : 'bg-slate-800 text-gray-400 hover:bg-slate-700'
              }`}
            >
              Sent ({history.filter(h => h.is_sender).length})
            </button>
            <button
              onClick={() => setFilter('received')}
              className={`px-4 py-2 rounded-lg font-medium transition ${
                filter === 'received'
                  ? 'bg-blue-600 text-white'
                  : 'bg-slate-800 text-gray-400 hover:bg-slate-700'
              }`}
            >
              Received ({history.filter(h => h.is_recipient).length})
            </button>
          </div>
        </div>

        {/* Message List */}
        <div className="divide-y divide-dark-border max-h-[600px] overflow-y-auto">
          {filteredHistory.length === 0 ? (
            <div className="p-8 text-center text-gray-500">
              <div className="text-6xl mb-4">📭</div>
              <p>No messages found</p>
              <p className="text-sm mt-2">
                {filter === 'sent' && "You haven't sent any secrets yet"}
                {filter === 'received' && "You haven't received any secrets yet"}
                {filter === 'all' && "Your message history is empty"}
              </p>
            </div>
          ) : (
            filteredHistory.map((item, index) => (
              <div key={index} className="p-6 hover:bg-slate-800/50 transition">
                <div className="flex items-start justify-between">
                  {/* Left side - Message info */}
                  <div className="flex-1">
                    <div className="flex items-center gap-3 mb-2">
                      {/* Direction indicator */}
                      <span className="text-2xl">
                        {item.is_sender ? '📤' : '📥'}
                      </span>

                      {/* Participants */}
                      <div>
                        <p className="font-medium text-gray-200">
                          {item.is_sender ? (
                            <>To: <span className="text-blue-400">{item.recipient_name}</span></>
                          ) : (
                            <>From: <span className="text-orange-400">{item.sender_name}</span></>
                          )}
                        </p>
                        <p className="text-xs text-gray-500">
                          {formatDate(item.created_at)}
                        </p>
                      </div>
                    </div>

                    {/* Status details */}
                    <div className="ml-11 text-sm text-gray-400">
                      {item.status === 'read' && item.read_at && (
                        <p>Read {formatDate(item.read_at)}</p>
                      )}
                      {item.status === 'pending' && (
                        <p>Expires {formatDate(item.expires_at)}</p>
                      )}
                      {item.status === 'expired' && (
                        <p className="text-red-400">Expired without being read</p>
                      )}
                    </div>

                    {/* Show link for received pending messages */}
                    {item.is_recipient && item.status === 'pending' && item.encryption_key && (
                      <div className="ml-11 mt-3">
                        <a
                          href={generateShareableURL(item.message_id, item.encryption_key)}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg transition"
                        >
                          🔓 Open Message
                        </a>
                        <button
                          onClick={() => handleCopyLink(item.message_id, item.encryption_key)}
                          className="ml-2 inline-flex items-center gap-2 px-4 py-2 bg-slate-700 hover:bg-slate-600 text-white text-sm font-medium rounded-lg transition"
                        >
                          📋 Copy Link
                        </button>
                      </div>
                    )}
                  </div>

                  {/* Right side - Status badge */}
                  <div>
                    <span className={`inline-flex items-center gap-2 px-3 py-1 rounded-full text-xs font-medium border ${getStatusColor(item.status)}`}>
                      {getStatusIcon(item.status)} {item.status.toUpperCase()}
                    </span>
                  </div>
                </div>

                {/* Message ID (for debugging/support) */}
                {item.status !== 'pending' && (
                  <div className="mt-3 ml-11">
                    <details className="text-xs text-gray-600">
                      <summary className="cursor-pointer hover:text-gray-400">Message ID</summary>
                      <code className="block mt-1 p-2 bg-slate-900 rounded text-gray-500">
                        {item.message_id}
                      </code>
                    </details>
                  </div>
                )}
              </div>
            ))
          )}
        </div>

        {/* Footer */}
        {filteredHistory.length > 0 && (
          <div className="border-t border-dark-border p-4 text-center text-xs text-gray-500">
            Showing {filteredHistory.length} message{filteredHistory.length !== 1 ? 's' : ''}
          </div>
        )}
      </div>

      {/* Info box */}
      <div className="mt-6 bg-blue-900/20 border border-blue-500/30 rounded-lg p-4 text-sm text-gray-400">
        <p className="font-semibold text-blue-400 mb-2">📊 About Message History</p>
        <ul className="space-y-1 text-xs">
          <li>• This shows metadata only (who/when) - message content is never stored</li>
          <li>• Messages are permanently deleted from Redis after reading</li>
          <li>• Expired messages were never accessed before their TTL</li>
          <li>• History is stored for audit purposes only</li>
        </ul>
      </div>
    </div>
  );
}
