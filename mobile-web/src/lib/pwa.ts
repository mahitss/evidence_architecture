import { useCallback, useEffect, useState } from 'react';

const VIEWED_CASES_KEY = 'casefiles:viewed_cases:v1';
const INSTALL_DISMISSED_KEY = 'casefiles:install_dismissed:v1';
const INSTALL_VIEW_THRESHOLD = 3;

interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
}

function readViewedCases(): string[] {
  try {
    const raw = window.localStorage.getItem(VIEWED_CASES_KEY);
    if (!raw) return [];
    const parsed: unknown = JSON.parse(raw);
    if (Array.isArray(parsed)) {
      return parsed.filter((item): item is string => typeof item === 'string');
    }
    return [];
  } catch {
    return [];
  }
}

export function getViewedCasesCount(): number {
  return readViewedCases().length;
}

export function recordCaseView(slug: string): number {
  if (!slug) return getViewedCasesCount();
  try {
    const current = readViewedCases();
    if (!current.includes(slug)) {
      const next = [...current.slice(-49), slug];
      window.localStorage.setItem(VIEWED_CASES_KEY, JSON.stringify(next));
      window.dispatchEvent(new CustomEvent('casefiles:viewed-updated', { detail: next.length }));
      return next.length;
    }
    return current.length;
  } catch {
    return 0;
  }
}

function isStandaloneMode(): boolean {
  if (typeof window === 'undefined') return false;
  const mql = window.matchMedia?.('(display-mode: standalone)');
  const iosStandalone = (window.navigator as Navigator & { standalone?: boolean }).standalone === true;
  return Boolean(mql?.matches || iosStandalone);
}

function isIOSDevice(): boolean {
  if (typeof navigator === 'undefined') return false;
  return /iphone|ipad|ipod/i.test(navigator.userAgent);
}

export function usePWAInstall() {
  const [deferredPrompt, setDeferredPrompt] = useState<BeforeInstallPromptEvent | null>(null);
  const [viewedCount, setViewedCount] = useState<number>(() => getViewedCasesCount());
  const [standalone, setStandalone] = useState<boolean>(() => isStandaloneMode());
  const [dismissed, setDismissed] = useState<boolean>(() => {
    try {
      return window.localStorage.getItem(INSTALL_DISMISSED_KEY) === '1';
    } catch {
      return false;
    }
  });

  useEffect(() => {
    const handleBeforeInstall = (event: Event) => {
      event.preventDefault();
      setDeferredPrompt(event as BeforeInstallPromptEvent);
    };

    const handleInstalled = () => {
      setStandalone(true);
      setDeferredPrompt(null);
    };

    const handleViewedUpdate = () => {
      setViewedCount(getViewedCasesCount());
    };

    window.addEventListener('beforeinstallprompt', handleBeforeInstall);
    window.addEventListener('appinstalled', handleInstalled);
    window.addEventListener('casefiles:viewed-updated', handleViewedUpdate);

    return () => {
      window.removeEventListener('beforeinstallprompt', handleBeforeInstall);
      window.removeEventListener('appinstalled', handleInstalled);
      window.removeEventListener('casefiles:viewed-updated', handleViewedUpdate);
    };
  }, []);

  const dismiss = useCallback(() => {
    setDismissed(true);
    try {
      window.localStorage.setItem(INSTALL_DISMISSED_KEY, '1');
    } catch {
      // Ignore storage quota errors
    }
  }, []);

  const promptInstall = useCallback(async (): Promise<'accepted' | 'dismissed' | 'ios' | 'unavailable'> => {
    if (deferredPrompt) {
      await deferredPrompt.prompt();
      const choice = await deferredPrompt.userChoice;
      setDeferredPrompt(null);
      if (choice.outcome === 'accepted') {
        setStandalone(true);
      }
      return choice.outcome;
    }
    if (isIOSDevice()) {
      return 'ios';
    }
    return 'unavailable';
  }, [deferredPrompt]);

  const eligibleAfterReading = viewedCount >= INSTALL_VIEW_THRESHOLD;
  const canInstall = !standalone && !dismissed && eligibleAfterReading && (deferredPrompt !== null || isIOSDevice());

  return {
    canInstall,
    isIOS: isIOSDevice() && deferredPrompt === null,
    viewedCount,
    promptInstall,
    dismiss,
  };
}

export function useOnlineStatus(): boolean {
  const [online, setOnline] = useState<boolean>(() =>
    typeof navigator === 'undefined' ? true : navigator.onLine,
  );

  useEffect(() => {
    const goOnline = () => setOnline(true);
    const goOffline = () => setOnline(false);
    window.addEventListener('online', goOnline);
    window.addEventListener('offline', goOffline);
    return () => {
      window.removeEventListener('online', goOnline);
      window.removeEventListener('offline', goOffline);
    };
  }, []);

  return online;
}

export async function shareOrCopy(payload: {
  title: string;
  text?: string;
  url: string;
}): Promise<'shared' | 'copied' | 'failed'> {
  if (typeof navigator !== 'undefined' && typeof navigator.share === 'function') {
    try {
      await navigator.share(payload);
      return 'shared';
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') {
        return 'shared';
      }
    }
  }

  if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(payload.url);
      return 'copied';
    } catch {
      return 'failed';
    }
  }

  return 'failed';
}
