import { useNavigate } from 'react-router-dom';
import Button from '../components/ui/Button';

export default function NotFound() {
  const navigate = useNavigate();

  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh] text-center px-4">
      <span className="material-symbols-outlined text-fg-subtle text-7xl mb-6" aria-hidden="true">explore_off</span>
      <h1 className="text-5xl font-semibold text-fg tabular-nums mb-2">404</h1>
      <p className="text-fg-muted text-sm max-w-md mb-6">
        The page you're looking for doesn't exist or has been moved. Check the URL or head back to the dashboard.
      </p>
      <div className="flex gap-3">
        <Button onClick={() => navigate('/')} icon="home">
          Go home
        </Button>
        <Button onClick={() => navigate(-1)} variant="secondary" icon="arrow_back">
          Go back
        </Button>
      </div>
    </div>
  );
}
