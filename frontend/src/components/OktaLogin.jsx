import React, { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';

/**
 * OktaCallback component handles the OAuth callback from Okta
 */
export function OktaCallback() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { setUser, setToken } = useAuth();
  const [error, setError] = useState('');
  const [processing, setProcessing] = useState(true);

  useEffect(() => {
    handleCallback();
  }, []);

  const handleCallback = async () => {
    try {
      // Get code and state from URL
      const code = searchParams.get('code');
      const state = searchParams.get('state');
      const errorParam = searchParams.get('error');

      if (errorParam) {
        const errorDesc = searchParams.get('error_description');
        throw new Error(errorDesc || errorParam);
      }

      if (!code || !state) {
        throw new Error('Missing authorization code or state');
      }

      // Send to backend to exchange for our JWT
      const response = await fetch(`/api/auth/okta/callback?code=${code}&state=${state}`);

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Authentication failed');
      }

      const data = await response.json();

      // Store token and user
      localStorage.setItem('token', data.token);
      setToken(data.token);
      setUser(data.user);

      // Redirect to home
      navigate('/');
    } catch (err) {
      setError(err.message);
      setProcessing(false);
    }
  };

  if (processing) {
    return (
      <div className="w-full max-w-md mx-auto">
        <div className="bg-dark-card border border-dark-border rounded-lg p-8 text-center">
          <div className="text-6xl mb-4 animate-pulse">🔐</div>
          <h2 className="text-2xl font-bold mb-4">Signing you in...</h2>
          <p className="text-gray-400">Completing authentication with Okta</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="w-full max-w-md mx-auto">
        <div className="bg-dark-card border border-dark-border rounded-lg p-8">
          <div className="text-6xl mb-4 text-center">❌</div>
          <h2 className="text-2xl font-bold mb-4 text-center text-red-400">Authentication Failed</h2>
          <div className="bg-red-900/30 border border-red-500 text-red-300 px-4 py-3 rounded-lg mb-4">
            {error}
          </div>
          <button
            onClick={() => navigate('/login')}
            className="w-full bg-dark-border hover:bg-slate-600 text-white font-semibold py-3 px-6 rounded-lg transition"
          >
            Back to Login
          </button>
        </div>
      </div>
    );
  }

  return null;
}

/**
 * OktaLoginButton component triggers SSO login
 */
export function OktaLoginButton() {
  const handleOktaLogin = () => {
    // Redirect to backend Okta login endpoint
    window.location.href = '/api/auth/okta/login';
  };

  return (
    <button
      onClick={handleOktaLogin}
      className="w-full bg-blue-600 hover:bg-blue-700 text-white font-semibold py-3 px-6 rounded-lg transition duration-200 flex items-center justify-center gap-2"
    >
      <svg className="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
        <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8z"/>
        <circle cx="12" cy="12" r="3"/>
      </svg>
      Sign in with Okta
    </button>
  );
}

export default OktaCallback;
