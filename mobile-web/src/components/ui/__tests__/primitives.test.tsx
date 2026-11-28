import { useState } from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import {
  Badge,
  Button,
  Callout,
  Card,
  Dialog,
  LicenseStatusBadge,
  VerificationTierBadge,
} from '../index';
import { parseHighlightSegments } from '../../../lib/format';

describe('UI Primitives', () => {
  it('renders VerificationTierBadge for tiers 1 through 4', () => {
    render(
      <div>
        <VerificationTierBadge tier={1} />
        <VerificationTierBadge tier={2} />
        <VerificationTierBadge tier={3} />
        <VerificationTierBadge tier={4} />
      </div>,
    );

    expect(screen.getByText('T1')).toBeInTheDocument();
    expect(screen.getByText('T2')).toBeInTheDocument();
    expect(screen.getByText('T3')).toBeInTheDocument();
    expect(screen.getByText('T4')).toBeInTheDocument();
  });

  it('renders LicenseStatusBadge with warning icon on RECREATED', () => {
    const { container } = render(
      <div>
        <LicenseStatusBadge status="PUBLIC_DOMAIN" />
        <LicenseStatusBadge status="LICENSED" />
        <LicenseStatusBadge status="RECREATED" />
      </div>,
    );

    expect(screen.getByText('PUBLIC DOMAIN')).toBeInTheDocument();
    expect(screen.getByText('LICENSED')).toBeInTheDocument();
    expect(screen.getByText('RECREATED')).toBeInTheDocument();
    // Only RECREATED includes an SVG warning triangle icon
    expect(container.querySelectorAll('svg').length).toBe(1);
  });

  it('renders paper-toned Card and generic Badge', () => {
    render(
      <Card data-testid="dossier-card">
        <Badge variant="category">Financial Crimes</Badge>
        <p>Case file contents</p>
      </Card>,
    );

    expect(screen.getByTestId('dossier-card')).toBeInTheDocument();
    expect(screen.getByText('Financial Crimes')).toBeInTheDocument();
    expect(screen.getByText('Case file contents')).toBeInTheDocument();
  });

  it('renders RECREATED warning Callout with alert role', () => {
    render(
      <Callout variant="warning" title="Recreated Document Notice">
        This is a recreation based on public court filings. It is not the original document.
      </Callout>,
    );

    const alert = screen.getByRole('alert', { name: 'Recreated Document Notice' });
    expect(alert).toBeInTheDocument();
    expect(alert).toHaveTextContent(/not the original document/i);
  });

  it('traps focus in Dialog, closes on Escape, and restores focus', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();

    function Harness() {
      const [open, setOpen] = useState(false);
      return (
        <div>
          <Button onClick={() => setOpen(true)}>Open Dialog</Button>
          <Dialog
            open={open}
            onClose={() => {
              onClose();
              setOpen(false);
            }}
            title="Verification Help"
          >
            <p>Check the SHA-256 digest.</p>
            <Button onClick={() => setOpen(false)}>Confirm</Button>
          </Dialog>
        </div>
      );
    }

    render(<Harness />);

    const trigger = screen.getByRole('button', { name: 'Open Dialog' });
    await user.click(trigger);

    const dialog = screen.getByRole('dialog', { name: 'Verification Help' });
    expect(dialog).toBeInTheDocument();

    const closeBtn = screen.getByRole('button', { name: 'Close dialog' });
    const confirmBtn = screen.getByRole('button', { name: 'Confirm' });
    expect(closeBtn).toHaveFocus();

    // Tab moves to Confirm, Tab again wraps back to Close dialog (focus trap)
    await user.tab();
    expect(confirmBtn).toHaveFocus();
    await user.tab();
    expect(closeBtn).toHaveFocus();

    // Shift+Tab wraps backwards to Confirm
    await user.tab({ shift: true });
    expect(confirmBtn).toHaveFocus();

    // Escape closes dialog and restores focus to trigger
    await user.keyboard('{Escape}');
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });

  it('parses search <mark> highlights safely without executing raw HTML', () => {
    const segments = parseHighlightSegments(
      'Docket <mark>Meridian</mark> <script>alert(1)</script> exhibit',
    );
    expect(segments).toEqual([
      { text: 'Docket ', highlighted: false },
      { text: 'Meridian', highlighted: true },
      { text: ' <script>alert(1)</script> exhibit', highlighted: false },
    ]);
  });
});
