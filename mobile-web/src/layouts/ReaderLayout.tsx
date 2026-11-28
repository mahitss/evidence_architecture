import { useState, type ReactNode } from 'react';
import { Link, NavLink, Outlet } from 'react-router-dom';
import { Button, Dialog, useToast } from '../components/ui';
import { useOnlineStatus, usePWAInstall } from '../lib/pwa';

interface NavItem {
  to: string;
  label: string;
  end?: boolean;
  icon: ReactNode;
}

const NAV_ITEMS: NavItem[] = [
  {
    to: '/',
    label: 'Cases',
    end: true,
    icon: (
      <svg viewBox="0 0 20 20" fill="currentColor" className="w-5 h-5" aria-hidden="true">
        <path d="M3.75 3A1.75 1.75 0 002 4.75v10.5c0 .966.784 1.75 1.75 1.75h12.5A1.75 1.75 0 0018 15.25v-8.5A1.75 1.75 0 0016.25 5h-4.836a.25.25 0 01-.177-.073L9.823 3.513A1.75 1.75 0 008.586 3H3.75z" />
      </svg>
    ),
  },
  {
    to: '/search',
    label: 'Search',
    icon: (
      <svg viewBox="0 0 20 20" fill="currentColor" className="w-5 h-5" aria-hidden="true">
        <path
          fillRule="evenodd"
          d="M9 3.5a5.5 5.5 0 100 11 5.5 5.5 0 000-11zM2 9a7 7 0 1112.452 4.391l3.328 3.329a.75.75 0 11-1.06 1.06l-3.329-3.328A7 7 0 012 9z"
          clipRule="evenodd"
        />
      </svg>
    ),
  },
  {
    to: '/about',
    label: 'About',
    icon: (
      <svg viewBox="0 0 20 20" fill="currentColor" className="w-5 h-5" aria-hidden="true">
        <path
          fillRule="evenodd"
          d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a.75.75 0 000 1.5h.253a.25.25 0 01.244.304l-.459 2.066A1.75 1.75 0 0010.747 15H11a.75.75 0 000-1.5h-.253a.25.25 0 01-.244-.304l.459-2.066A1.75 1.75 0 009.253 9H9z"
          clipRule="evenodd"
        />
      </svg>
    ),
  },
];

export function ReaderLayout() {
  const online = useOnlineStatus();
  const { canInstall, isIOS, promptInstall, dismiss } = usePWAInstall();
  const { pushToast } = useToast();
  const [iosDialogOpen, setIosDialogOpen] = useState(false);

  const handleInstallClick = async () => {
    const result = await promptInstall();
    if (result === 'ios') {
      setIosDialogOpen(true);
    } else if (result === 'accepted') {
      pushToast({ message: 'Case Files installed for offline reading.', variant: 'success' });
    }
  };

  return (
    <div className="min-h-dvh flex flex-col bg-[var(--surface-light)] text-[var(--ink)]">
      <a href="#main-content" className="skip-link">
        Skip to main content
      </a>

      {/* Top navigation bar (compact 56px height to keep mobile sticky surface <= 15%) */}
      <header className="sticky top-0 z-30 h-14 bg-[var(--surface-dark)] text-[var(--ink-on-dark)] border-b border-[var(--border)]">
        <div className="max-w-6xl mx-auto h-full px-4 flex items-center justify-between gap-4">
          <Link
            to="/"
            className="font-[var(--font-serif)] text-lg sm:text-xl font-bold tracking-[0.12em] uppercase text-[var(--ink-on-dark)] no-underline hover:text-[var(--paper)]"
          >
            CASE FILES
          </Link>

          <div className="flex items-center gap-2 sm:gap-4">
            <nav aria-label="Primary navigation" className="hidden md:flex items-center gap-1">
              {NAV_ITEMS.map((item) => (
                <NavLink
                  key={item.to}
                  to={item.to}
                  end={item.end}
                  className={({ isActive }) =>
                    `px-3 py-2 text-sm font-medium rounded-[var(--radius-sm)] no-underline transition-colors duration-[var(--duration-fast)] ${
                      isActive
                        ? 'text-[var(--ink-on-dark)] bg-[#1d2028] border border-[var(--accent)]'
                        : 'text-[var(--ink-on-dark-muted)] hover:text-[var(--ink-on-dark)] hover:bg-[#1a1d24] border border-transparent'
                    }`
                  }
                >
                  {item.label}
                </NavLink>
              ))}
            </nav>

            {canInstall && (
              <div className="flex items-center gap-1">
                <Button
                  variant="primary"
                  size="sm"
                  onClick={handleInstallClick}
                  aria-label="Add Case Files to home screen"
                  className="text-xs px-2.5 min-h-[36px]"
                >
                  {isIOS ? 'Save App' : 'Install App'}
                </Button>
                <button
                  type="button"
                  onClick={dismiss}
                  aria-label="Dismiss install prompt"
                  className="min-w-[36px] min-h-[36px] flex items-center justify-center text-[var(--ink-on-dark-muted)] hover:text-[var(--ink-on-dark)] cursor-pointer"
                >
                  &times;
                </button>
              </div>
            )}
          </div>
        </div>
      </header>

      {/* Offline status notice */}
      {!online && (
        <div
          role="status"
          className="bg-[var(--t3-bg)] text-[var(--t3-text)] border-b border-[var(--t3-border)] px-4 py-2 text-center text-xs font-medium"
        >
          Offline mode — showing cached case files where available.
        </div>
      )}

      {/* Main page content */}
      <main id="main-content" tabIndex={-1} className="flex-1 w-full max-w-6xl mx-auto px-4 py-6 sm:py-8 pb-24 md:pb-12">
        <Outlet />
      </main>

      {/* Editorial footer */}
      <footer className="border-t border-[var(--surface-border-subtle)] bg-[var(--paper)] text-[var(--ink-secondary)] pb-20 md:pb-6 pt-5 px-4">
        <div className="max-w-6xl mx-auto flex flex-col sm:flex-row items-start sm:items-center justify-between gap-2 text-xs">
          <p className="m-0">
            Every claim is sourced. Every source is tiered.{' '}
            <span aria-hidden="true" className="mx-1.5 text-[var(--surface-border-strong)]">
              |
            </span>
            <Link
              to="/correction"
              className="underline font-medium text-[var(--accent-text)] hover:text-[var(--ink)]"
            >
              Report a correction
            </Link>
          </p>
          <p className="m-0 font-[var(--font-mono)] text-[var(--ink-on-paper-muted)]">
            Anonymous public-record archive · No accounts · No tracking
          </p>
        </div>
      </footer>

      {/* Mobile bottom tab navigation (thumb-zone ergonomic anchor, 56px height) */}
      <nav
        aria-label="Mobile navigation"
        className="md:hidden fixed bottom-0 left-0 right-0 z-30 bg-[var(--surface-dark)] border-t border-[var(--border)] pb-safe"
      >
        <div className="grid grid-cols-3 h-14">
          {NAV_ITEMS.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) =>
                `flex flex-col items-center justify-center gap-0.5 min-h-[44px] no-underline text-[11px] font-medium transition-colors duration-[var(--duration-fast)] ${
                  isActive
                    ? 'text-[var(--paper)] border-t-2 border-[var(--accent)]'
                    : 'text-[var(--ink-on-dark-muted)] border-t-2 border-transparent hover:text-[var(--ink-on-dark)]'
                }`
              }
            >
              {item.icon}
              <span>{item.label}</span>
            </NavLink>
          ))}
        </div>
      </nav>

      {/* iOS Safari Add to Home Screen instructions dialog */}
      <Dialog
        open={iosDialogOpen}
        onClose={() => setIosDialogOpen(false)}
        title="Add Case Files to Home Screen"
      >
        <ol className="m-0 pl-5 space-y-2 text-sm text-[var(--ink)]">
          <li>Tap the Share button in Safari&apos;s toolbar.</li>
          <li>Scroll down and tap &ldquo;Add to Home Screen&rdquo;.</li>
          <li>Tap &ldquo;Add&rdquo; to save Case Files for offline reading.</li>
        </ol>
      </Dialog>
    </div>
  );
}
