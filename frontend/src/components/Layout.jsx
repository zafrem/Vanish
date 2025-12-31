import React from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';

/**
 * Layout component providing consistent dark-themed styling
 */
export default function Layout({ children }) {
  const { isAuthenticated, user, logout, timeLeft } = useAuth();

  const formatTime = (seconds) => {
    if (seconds === null || seconds === undefined) return '';
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  };

  const getTimerColor = () => {
    if (timeLeft === null) return '';
    if (timeLeft <= 10) return 'text-red-400 animate-pulse';
    if (timeLeft <= 30) return 'text-yellow-400';
    return 'text-green-400';
  };

  return (
    <div className="min-h-screen bg-gradient-to-br from-dark-bg to-slate-900 text-gray-100">
      <div className="container mx-auto px-4 py-8">
        <header className="mb-12">
          <div className="flex justify-between items-center">
            <Link to="/" className="block">
              <h1 className="text-4xl font-bold mb-2 bg-gradient-to-r from-red-500 to-orange-500 bg-clip-text text-transparent">
                Vanish
              </h1>
              <p className="text-gray-400 text-sm">
                Ephemeral, zero-knowledge messaging for secure information transfer
              </p>
            </Link>

            {isAuthenticated && (
              <div className="flex items-center gap-4">
                {/* Show different navigation based on user role */}
                {!user?.is_admin ? (
                  // Regular user navigation
                  <>
                    <Link
                      to="/create"
                      className="text-gray-400 hover:text-gray-200 text-sm font-medium transition"
                    >
                      Create
                    </Link>
                    <Link
                      to="/history"
                      className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-lg text-sm font-medium transition"
                    >
                      History
                    </Link>
                  </>
                ) : (
                  // Admin navigation
                  <Link
                    to="/admin"
                    className="bg-orange-600 hover:bg-orange-700 text-white px-4 py-2 rounded-lg text-sm font-medium transition"
                  >
                    Dashboard
                  </Link>
                )}

                <div className="text-right">
                  <p className="text-sm text-gray-300">
                    {user?.name}
                    {user?.is_admin && (
                      <span className="ml-2 px-2 py-0.5 bg-orange-900/50 border border-orange-500 text-orange-400 text-xs rounded">
                        Admin
                      </span>
                    )}
                  </p>
                  <p className="text-xs text-gray-500">{user?.email}</p>
                  {timeLeft !== null && (
                    <p className={`text-xs font-mono font-bold mt-1 ${getTimerColor()}`}>
                      ⏱️ {formatTime(timeLeft)}
                    </p>
                  )}
                </div>
                <button
                  onClick={logout}
                  className="bg-dark-border hover:bg-slate-600 text-white px-4 py-2 rounded-lg text-sm transition"
                >
                  Logout
                </button>
              </div>
            )}
          </div>
        </header>

        <main className="flex justify-center items-center min-h-[60vh]">
          {children}
        </main>

        <footer className="text-center mt-12 text-gray-500 text-xs">
          <p>Messages are encrypted client-side and destroyed after reading</p>
          <p className="mt-1">Only the intended recipient can read each message</p>
        </footer>
      </div>
    </div>
  );
}
