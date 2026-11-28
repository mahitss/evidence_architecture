import { useState } from 'react';
import { Button, useToast } from './ui';
import { shareOrCopy } from '../lib/pwa';

export interface ShareButtonProps {
  title: string;
  text?: string;
  url?: string;
  className?: string;
}

export function ShareButton({ title, text, url, className = '' }: ShareButtonProps) {
  const { pushToast } = useToast();
  const [busy, setBusy] = useState(false);

  const handleShare = async () => {
    setBusy(true);
    try {
      const targetUrl = url ?? (typeof window !== 'undefined' ? window.location.href : '');
      const outcome = await shareOrCopy({ title, text, url: targetUrl });
      if (outcome === 'copied') {
        pushToast({ message: 'Link copied to clipboard.', variant: 'success' });
      } else if (outcome === 'failed') {
        pushToast({ message: 'Could not copy link on this device.', variant: 'warning' });
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <Button
      variant="secondary"
      size="sm"
      onClick={handleShare}
      disabled={busy}
      aria-label={`Share ${title}`}
      className={className}
    >
      <svg
        viewBox="0 0 20 20"
        fill="currentColor"
        className="w-4 h-4 shrink-0"
        aria-hidden="true"
        focusable="false"
      >
        <path d="M13 4.5a2.5 2.5 0 11.702 1.737L6.97 9.604a2.518 2.518 0 010 .792l6.733 3.367a2.5 2.5 0 11-.671 1.341l-6.733-3.367a2.5 2.5 0 110-3.475l6.733-3.366A2.52 2.52 0 0113 4.5z" />
      </svg>
      <span>Share</span>
    </Button>
  );
}
