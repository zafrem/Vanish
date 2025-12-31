import React, { useState, useEffect } from 'react';
import { useAuth } from '../context/AuthContext';
import { getToken } from '../lib/api';

export default function AdminDashboard() {
  const { user } = useAuth();
  const [activeTab, setActiveTab] = useState('users');
  const [users, setUsers] = useState([]);
  const [statistics, setStatistics] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(null);

  // User form state
  const [showUserForm, setShowUserForm] = useState(false);
  const [editingUser, setEditingUser] = useState(null);
  const [userForm, setUserForm] = useState({
    email: '',
    name: '',
    password: '',
    is_admin: false,
  });

  // Session timeout setting
  const [sessionTimeout, setSessionTimeout] = useState(60);

  useEffect(() => {
    if (activeTab === 'users') {
      fetchUsers();
    } else if (activeTab === 'statistics') {
      fetchStatistics();
    }
  }, [activeTab]);

  const fetchUsers = async () => {
    try {
      setLoading(true);
      const response = await fetch('/api/users', {
        headers: {
          'Authorization': `Bearer ${getToken()}`,
        },
      });
      if (response.ok) {
        const data = await response.json();
        setUsers(data);
      } else {
        throw new Error('Failed to fetch users');
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const fetchStatistics = async () => {
    try {
      setLoading(true);
      const response = await fetch('/api/admin/statistics', {
        headers: {
          'Authorization': `Bearer ${getToken()}`,
        },
      });
      if (response.ok) {
        const data = await response.json();
        setStatistics(data);
      } else {
        throw new Error('Failed to fetch statistics');
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleCreateUser = async (e) => {
    e.preventDefault();
    try {
      setLoading(true);
      setError(null);
      const response = await fetch('/api/admin/users', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${getToken()}`,
        },
        body: JSON.stringify(userForm),
      });

      if (response.ok) {
        setSuccess('User created successfully');
        setShowUserForm(false);
        setUserForm({ email: '', name: '', password: '', is_admin: false });
        fetchUsers();
      } else {
        const data = await response.json();
        throw new Error(data.error || 'Failed to create user');
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleUpdateUser = async (e) => {
    e.preventDefault();
    try {
      setLoading(true);
      setError(null);
      const response = await fetch(`/api/admin/users/${editingUser.id}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${getToken()}`,
        },
        body: JSON.stringify(userForm),
      });

      if (response.ok) {
        setSuccess('User updated successfully');
        setEditingUser(null);
        setShowUserForm(false);
        setUserForm({ email: '', name: '', password: '', is_admin: false });
        fetchUsers();
      } else {
        const data = await response.json();
        throw new Error(data.error || 'Failed to update user');
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleDeleteUser = async (userId) => {
    if (!confirm('Are you sure you want to delete this user?')) return;

    try {
      setLoading(true);
      setError(null);
      const response = await fetch(`/api/admin/users/${userId}`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${getToken()}`,
        },
      });

      if (response.ok) {
        setSuccess('User deleted successfully');
        fetchUsers();
      } else {
        const data = await response.json();
        throw new Error(data.error || 'Failed to delete user');
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const startEditUser = (user) => {
    setEditingUser(user);
    setUserForm({
      email: user.email,
      name: user.name,
      password: '',
      is_admin: user.is_admin,
    });
    setShowUserForm(true);
  };

  const cancelUserForm = () => {
    setShowUserForm(false);
    setEditingUser(null);
    setUserForm({ email: '', name: '', password: '', is_admin: false });
  };

  const handleCleanup = async () => {
    if (!confirm('Manually trigger cleanup of expired messages?')) return;

    try {
      setLoading(true);
      setError(null);
      const response = await fetch('/api/admin/cleanup', {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${getToken()}`,
        },
      });

      if (response.ok) {
        const data = await response.json();
        setSuccess(`Cleanup completed: ${data.expired_count} messages removed`);
      } else {
        throw new Error('Failed to run cleanup');
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const tabs = [
    { id: 'users', label: 'User Management', icon: '👥' },
    { id: 'statistics', label: 'Statistics', icon: '📊' },
    { id: 'settings', label: 'Settings', icon: '⚙️' },
    { id: 'api', label: 'API Documentation', icon: '📚' },
  ];

  return (
    <div className="w-full max-w-6xl mx-auto">
      <div className="bg-dark-card border border-dark-border rounded-lg shadow-2xl">
        {/* Header */}
        <div className="border-b border-dark-border p-6">
          <h2 className="text-2xl font-bold">Admin Dashboard</h2>
          <p className="text-gray-400 text-sm mt-1">
            Logged in as: {user?.name} ({user?.email})
          </p>
        </div>

        {/* Tabs */}
        <div className="border-b border-dark-border px-6">
          <div className="flex gap-2">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`px-4 py-3 font-medium transition border-b-2 ${
                  activeTab === tab.id
                    ? 'border-blue-500 text-blue-400'
                    : 'border-transparent text-gray-400 hover:text-gray-200'
                }`}
              >
                {tab.icon} {tab.label}
              </button>
            ))}
          </div>
        </div>

        {/* Notifications */}
        {error && (
          <div className="mx-6 mt-6 bg-red-900/30 border border-red-500 text-red-300 px-4 py-3 rounded-lg">
            {error}
            <button onClick={() => setError(null)} className="float-right font-bold">×</button>
          </div>
        )}
        {success && (
          <div className="mx-6 mt-6 bg-green-900/30 border border-green-500 text-green-300 px-4 py-3 rounded-lg">
            {success}
            <button onClick={() => setSuccess(null)} className="float-right font-bold">×</button>
          </div>
        )}

        {/* Content */}
        <div className="p-6">
          {/* User Management Tab */}
          {activeTab === 'users' && (
            <div>
              <div className="flex justify-between items-center mb-6">
                <h3 className="text-xl font-semibold">Users</h3>
                <button
                  onClick={() => setShowUserForm(!showUserForm)}
                  className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-lg text-sm font-medium transition"
                >
                  {showUserForm ? 'Cancel' : '+ Add User'}
                </button>
              </div>

              {/* User Form */}
              {showUserForm && (
                <div className="bg-slate-800 border border-dark-border rounded-lg p-6 mb-6">
                  <h4 className="text-lg font-semibold mb-4">
                    {editingUser ? 'Edit User' : 'Create New User'}
                  </h4>
                  <form onSubmit={editingUser ? handleUpdateUser : handleCreateUser} className="space-y-4">
                    <div>
                      <label className="block text-sm font-medium mb-2">Email</label>
                      <input
                        type="email"
                        value={userForm.email}
                        onChange={(e) => setUserForm({ ...userForm, email: e.target.value })}
                        required
                        className="w-full bg-slate-900 border border-dark-border rounded-lg px-4 py-2 text-gray-100"
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium mb-2">Name</label>
                      <input
                        type="text"
                        value={userForm.name}
                        onChange={(e) => setUserForm({ ...userForm, name: e.target.value })}
                        required
                        className="w-full bg-slate-900 border border-dark-border rounded-lg px-4 py-2 text-gray-100"
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium mb-2">
                        Password {editingUser && '(leave blank to keep current)'}
                      </label>
                      <input
                        type="password"
                        value={userForm.password}
                        onChange={(e) => setUserForm({ ...userForm, password: e.target.value })}
                        required={!editingUser}
                        className="w-full bg-slate-900 border border-dark-border rounded-lg px-4 py-2 text-gray-100"
                      />
                    </div>
                    <div className="flex items-center">
                      <input
                        type="checkbox"
                        id="is_admin"
                        checked={userForm.is_admin}
                        onChange={(e) => setUserForm({ ...userForm, is_admin: e.target.checked })}
                        className="mr-2"
                      />
                      <label htmlFor="is_admin" className="text-sm">Admin User</label>
                    </div>
                    <div className="flex gap-2">
                      <button
                        type="submit"
                        disabled={loading}
                        className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-lg text-sm font-medium transition disabled:opacity-50"
                      >
                        {loading ? 'Saving...' : editingUser ? 'Update User' : 'Create User'}
                      </button>
                      <button
                        type="button"
                        onClick={cancelUserForm}
                        className="bg-slate-700 hover:bg-slate-600 text-white px-4 py-2 rounded-lg text-sm font-medium transition"
                      >
                        Cancel
                      </button>
                    </div>
                  </form>
                </div>
              )}

              {/* User List */}
              {loading && !showUserForm ? (
                <p className="text-gray-400">Loading users...</p>
              ) : (
                <div className="space-y-2">
                  {users.map((u) => (
                    <div
                      key={u.id}
                      className="bg-slate-800 border border-dark-border rounded-lg p-4 flex justify-between items-center"
                    >
                      <div>
                        <p className="font-medium">{u.name}</p>
                        <p className="text-sm text-gray-400">{u.email}</p>
                        {u.is_admin && (
                          <span className="inline-block mt-1 px-2 py-1 bg-orange-900/30 border border-orange-500 text-orange-400 text-xs rounded">
                            Admin
                          </span>
                        )}
                      </div>
                      <div className="flex gap-2">
                        <button
                          onClick={() => startEditUser(u)}
                          className="bg-blue-600 hover:bg-blue-700 text-white px-3 py-1 rounded text-sm transition"
                        >
                          Edit
                        </button>
                        {u.id !== user?.id && (
                          <button
                            onClick={() => handleDeleteUser(u.id)}
                            className="bg-red-600 hover:bg-red-700 text-white px-3 py-1 rounded text-sm transition"
                          >
                            Delete
                          </button>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Statistics Tab */}
          {activeTab === 'statistics' && (
            <div>
              <div className="flex justify-between items-center mb-6">
                <h3 className="text-xl font-semibold">System Statistics</h3>
                <button
                  onClick={handleCleanup}
                  disabled={loading}
                  className="bg-orange-600 hover:bg-orange-700 text-white px-4 py-2 rounded-lg text-sm font-medium transition disabled:opacity-50"
                >
                  {loading ? 'Running...' : 'Run Cleanup'}
                </button>
              </div>

              {loading ? (
                <p className="text-gray-400">Loading statistics...</p>
              ) : statistics ? (
                <div className="grid grid-cols-2 gap-4">
                  {/* Users Stats */}
                  <div className="bg-slate-800 border border-dark-border rounded-lg p-6">
                    <h4 className="text-lg font-semibold mb-4 text-blue-400">👥 Users</h4>
                    <div className="space-y-2">
                      <div className="flex justify-between">
                        <span className="text-gray-400">Total Users:</span>
                        <span className="font-bold">{statistics.users.total}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-gray-400">Admin Users:</span>
                        <span className="font-bold text-orange-400">{statistics.users.admins}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-gray-400">Regular Users:</span>
                        <span className="font-bold text-green-400">{statistics.users.regular}</span>
                      </div>
                    </div>
                  </div>

                  {/* Messages Stats */}
                  <div className="bg-slate-800 border border-dark-border rounded-lg p-6">
                    <h4 className="text-lg font-semibold mb-4 text-purple-400">💬 Messages</h4>
                    <div className="space-y-2">
                      <div className="flex justify-between">
                        <span className="text-gray-400">Total Messages:</span>
                        <span className="font-bold">{statistics.messages.total}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-gray-400">Pending:</span>
                        <span className="font-bold text-yellow-400">{statistics.messages.pending}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-gray-400">Read:</span>
                        <span className="font-bold text-green-400">{statistics.messages.read}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-gray-400">Expired:</span>
                        <span className="font-bold text-red-400">{statistics.messages.expired}</span>
                      </div>
                    </div>
                  </div>
                </div>
              ) : (
                <p className="text-gray-400">No statistics available</p>
              )}
            </div>
          )}

          {/* Settings Tab */}
          {activeTab === 'settings' && (
            <div>
              <h3 className="text-xl font-semibold mb-6">System Settings</h3>

              <div className="bg-slate-800 border border-dark-border rounded-lg p-6 max-w-md">
                <h4 className="text-lg font-semibold mb-4">Session Timeout</h4>
                <p className="text-sm text-gray-400 mb-4">
                  Set the session timeout duration for regular users (admins have no timeout).
                </p>
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium mb-2">Timeout (seconds)</label>
                    <input
                      type="number"
                      value={sessionTimeout}
                      onChange={(e) => setSessionTimeout(Number(e.target.value))}
                      min="30"
                      max="3600"
                      className="w-full bg-slate-900 border border-dark-border rounded-lg px-4 py-2 text-gray-100"
                    />
                    <p className="text-xs text-gray-500 mt-1">
                      Current: {Math.floor(sessionTimeout / 60)}m {sessionTimeout % 60}s
                    </p>
                  </div>
                  <div className="bg-blue-900/20 border border-blue-500/30 rounded p-3 text-xs text-gray-400">
                    ℹ️ Note: Changing this value requires updating the SESSION_TIMEOUT constant in AuthContext.jsx and restarting the frontend.
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* API Documentation Tab */}
          {activeTab === 'api' && (
            <div>
              <h3 className="text-xl font-semibold mb-6">API Documentation</h3>

              <div className="bg-slate-800 border border-dark-border rounded-lg p-6">
                <div className="space-y-4">
                  <div>
                    <h4 className="text-lg font-semibold mb-2">Base URL</h4>
                    <code className="block bg-slate-900 p-3 rounded text-sm text-green-400">
                      http://localhost:8080
                    </code>
                  </div>

                  <div>
                    <h4 className="text-lg font-semibold mb-2">Authentication</h4>
                    <p className="text-sm text-gray-400 mb-2">
                      All protected endpoints require a Bearer token in the Authorization header:
                    </p>
                    <code className="block bg-slate-900 p-3 rounded text-sm text-green-400">
                      Authorization: Bearer YOUR_TOKEN_HERE
                    </code>
                  </div>

                  <div>
                    <h4 className="text-lg font-semibold mb-2">Complete Documentation</h4>
                    <p className="text-sm text-gray-400 mb-3">
                      Full API reference is available in the project repository:
                    </p>
                    <code className="block bg-slate-900 p-3 rounded text-sm text-gray-400">
                      /API_REFERENCE.md
                    </code>
                  </div>

                  <div className="border-t border-dark-border pt-4 mt-4">
                    <h4 className="text-lg font-semibold mb-3">Quick Reference - Admin Endpoints</h4>
                    <div className="space-y-3 text-sm">
                      <div className="bg-slate-900 p-3 rounded">
                        <div className="flex items-center gap-2 mb-1">
                          <span className="px-2 py-1 bg-green-600 text-white text-xs rounded font-mono">GET</span>
                          <code className="text-gray-300">/api/admin/statistics</code>
                        </div>
                        <p className="text-gray-500 text-xs">Get system statistics</p>
                      </div>

                      <div className="bg-slate-900 p-3 rounded">
                        <div className="flex items-center gap-2 mb-1">
                          <span className="px-2 py-1 bg-blue-600 text-white text-xs rounded font-mono">POST</span>
                          <code className="text-gray-300">/api/admin/users</code>
                        </div>
                        <p className="text-gray-500 text-xs">Create a new user</p>
                      </div>

                      <div className="bg-slate-900 p-3 rounded">
                        <div className="flex items-center gap-2 mb-1">
                          <span className="px-2 py-1 bg-yellow-600 text-white text-xs rounded font-mono">PUT</span>
                          <code className="text-gray-300">/api/admin/users/:id</code>
                        </div>
                        <p className="text-gray-500 text-xs">Update a user</p>
                      </div>

                      <div className="bg-slate-900 p-3 rounded">
                        <div className="flex items-center gap-2 mb-1">
                          <span className="px-2 py-1 bg-red-600 text-white text-xs rounded font-mono">DELETE</span>
                          <code className="text-gray-300">/api/admin/users/:id</code>
                        </div>
                        <p className="text-gray-500 text-xs">Delete a user</p>
                      </div>

                      <div className="bg-slate-900 p-3 rounded">
                        <div className="flex items-center gap-2 mb-1">
                          <span className="px-2 py-1 bg-blue-600 text-white text-xs rounded font-mono">POST</span>
                          <code className="text-gray-300">/api/admin/cleanup</code>
                        </div>
                        <p className="text-gray-500 text-xs">Manually trigger expired message cleanup</p>
                      </div>

                      <div className="bg-slate-900 p-3 rounded">
                        <div className="flex items-center gap-2 mb-1">
                          <span className="px-2 py-1 bg-blue-600 text-white text-xs rounded font-mono">POST</span>
                          <code className="text-gray-300">/api/admin/users/import</code>
                        </div>
                        <p className="text-gray-500 text-xs">Import users from CSV file</p>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
