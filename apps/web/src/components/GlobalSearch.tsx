import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Search } from 'lucide-react';
import { Input } from '@/components/win95';
import { searchApi } from '@/lib/api-client';
import { useCurrentOrganization } from '@/contexts/OrganizationContext';

/** Searches secret names across every vault the user can read. Values are never involved. */
export const GlobalSearch = () => {
  const navigate = useNavigate();
  const { currentOrganization } = useCurrentOrganization();
  const [term, setTerm] = useState('');
  const [debounced, setDebounced] = useState('');
  const [open, setOpen] = useState(false);
  const container = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const timer = setTimeout(() => setDebounced(term.trim()), 250);
    return () => clearTimeout(timer);
  }, [term]);

  useEffect(() => {
    const close = (event: MouseEvent) => {
      if (!container.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', close);
    return () => document.removeEventListener('mousedown', close);
  }, []);

  const { data: results = [], isFetching } = useQuery({
    queryKey: ['search', debounced, currentOrganization?.id],
    queryFn: () => searchApi.secrets(debounced, currentOrganization?.id),
    enabled: debounced.length > 0,
  });

  const goTo = (vaultId: string, environment: string, keyName: string) => {
    setOpen(false);
    setTerm('');
    navigate(
      `/vaults/${vaultId}?env=${encodeURIComponent(environment)}&q=${encodeURIComponent(keyName)}`,
    );
  };

  return (
    <div ref={container} className="relative">
      <div className="flex items-center gap-1">
        <Search size={12} strokeWidth={1.5} aria-hidden="true" />
        <Input
          type="search"
          aria-label="Search secrets by name"
          placeholder="Search secrets..."
          className="!w-[180px] !py-[1px]"
          value={term}
          onChange={(e) => {
            setTerm(e.target.value);
            setOpen(true);
          }}
          onFocus={() => setOpen(true)}
          onKeyDown={(e) => {
            if (e.key === 'Escape') setOpen(false);
            if (e.key === 'Enter' && results[0]) {
              goTo(results[0].vaultId, results[0].environmentName, results[0].keyName);
            }
          }}
        />
      </div>

      {open && debounced && (
        <div className="absolute right-0 top-[22px] z-50 w-[360px] win-border-raised bg-background">
          <div className="win-border-sunken bg-input max-h-[280px] overflow-auto">
            {isFetching && results.length === 0 && (
              <p className="px-2 py-1 text-win-small text-muted-foreground">Searching...</p>
            )}
            {!isFetching && results.length === 0 && (
              <p className="px-2 py-1 text-win-small text-muted-foreground">
                No secret names match “{debounced}”.
              </p>
            )}
            {results.map((result) => (
              <button
                key={result.secretId}
                onClick={() => goTo(result.vaultId, result.environmentName, result.keyName)}
                className="block w-full text-left px-2 py-1 hover:bg-primary/10 border-b border-border/50"
              >
                <span className="font-mono text-win-body">{result.keyName}</span>
                <span className="block text-win-small text-muted-foreground">
                  {result.vaultName} / {result.environmentName}
                </span>
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};
