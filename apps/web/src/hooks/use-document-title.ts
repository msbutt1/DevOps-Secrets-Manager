import { useEffect } from 'react';

const SUFFIX = 'Vault Console';

/** Sets the browser tab title while the page is shown, e.g. "payments-api — Vault Console". */
export function useDocumentTitle(title?: string) {
  useEffect(() => {
    const previous = document.title;
    document.title = title ? `${title} — ${SUFFIX}` : `${SUFFIX} — DevOps Secrets Manager`;
    return () => {
      document.title = previous;
    };
  }, [title]);
}
