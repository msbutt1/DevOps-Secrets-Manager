import { Link, useLocation } from 'react-router-dom';
import { useEffect } from 'react';
import { Button } from '@/components/win95';
import { AlertTriangle } from 'lucide-react';
import { useDocumentTitle } from '@/hooks/use-document-title';

const NotFound = () => {
  useDocumentTitle('Page not found');
  const location = useLocation();

  useEffect(() => {
    console.error('404 Error: User attempted to access non-existent route:', location.pathname);
  }, [location.pathname]);

  return (
    <div className="min-h-screen bg-[#008080] flex items-center justify-center p-4">
      <div className="win-border-raised bg-background w-full max-w-[400px]">
        {/* Title Bar */}
        <div className="win-title-bar">
          <div className="flex items-center gap-2">
            <AlertTriangle size={14} strokeWidth={1.5} />
            <span>System Error</span>
          </div>
        </div>

        {/* Content */}
        <div className="p-4">
          <div className="flex gap-4 items-start mb-4">
            <AlertTriangle size={32} className="text-warning flex-shrink-0" strokeWidth={1.5} />
            <div>
              <h1 className="text-win-title font-semibold mb-2">404 — Resource Not Found</h1>
              <p className="text-win-body">
                The requested resource could not be located. The path may be incorrect or the
                resource may have been removed.
              </p>
              <p className="text-win-small text-muted-foreground mt-2">Path: {location.pathname}</p>
            </div>
          </div>

          <div className="flex justify-end gap-2 pt-2 border-t border-border">
            <Link to="/">
              <Button>Return to Dashboard</Button>
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
};

export default NotFound;
