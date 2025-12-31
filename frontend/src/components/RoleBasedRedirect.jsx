import { Navigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';

export default function RoleBasedRedirect() {
  const { user } = useAuth();

  // Redirect based on user role
  if (user?.is_admin) {
    return <Navigate to="/admin" replace />;
  } else {
    return <Navigate to="/create" replace />;
  }
}
