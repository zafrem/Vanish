import { Navigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';

export default function ProtectedRoute({ children, adminOnly = false, userOnly = false }) {
  const { isAuthenticated, loading, user } = useAuth();

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-gray-400">Loading...</div>
      </div>
    );
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  // Admin-only route accessed by non-admin
  if (adminOnly && !user?.is_admin) {
    return <Navigate to="/create" replace />;
  }

  // User-only route accessed by admin
  if (userOnly && user?.is_admin) {
    return <Navigate to="/admin" replace />;
  }

  return children;
}
