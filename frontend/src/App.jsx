import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider } from './context/AuthContext';
import Layout from './components/Layout';
import Login from './components/Login';
import Register from './components/Register';
import CreateMessage from './components/CreateMessage';
import RetrieveMessage from './components/RetrieveMessage';
import MessageHistory from './components/MessageHistory';
import OktaCallback from './components/OktaLogin';
import ProtectedRoute from './components/ProtectedRoute';
import AdminDashboard from './components/AdminDashboard';
import RoleBasedRedirect from './components/RoleBasedRedirect';

function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Layout>
          <Routes>
            <Route path="/login" element={<Login />} />
            <Route path="/register" element={<Register />} />
            <Route path="/auth/callback" element={<OktaCallback />} />

            {/* Root - redirects based on role */}
            <Route
              path="/"
              element={
                <ProtectedRoute>
                  <RoleBasedRedirect />
                </ProtectedRoute>
              }
            />

            {/* Admin routes */}
            <Route
              path="/admin"
              element={
                <ProtectedRoute adminOnly={true}>
                  <AdminDashboard />
                </ProtectedRoute>
              }
            />

            {/* User routes (non-admin only) */}
            <Route
              path="/create"
              element={
                <ProtectedRoute userOnly={true}>
                  <CreateMessage />
                </ProtectedRoute>
              }
            />
            <Route
              path="/history"
              element={
                <ProtectedRoute userOnly={true}>
                  <MessageHistory />
                </ProtectedRoute>
              }
            />
            <Route
              path="/m/:id"
              element={
                <ProtectedRoute userOnly={true}>
                  <RetrieveMessage />
                </ProtectedRoute>
              }
            />

            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </Layout>
      </AuthProvider>
    </BrowserRouter>
  );
}

export default App;
