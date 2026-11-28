import { screen } from '@testing-library/react';
import axe from 'axe-core';
import { describe, expect, it } from 'vitest';
import { MOCK_EVIDENCE_RECREATED } from './mswHandlers';
import { renderAppAt } from './renderWithProviders';

async function expectNoAxeViolations(container: HTMLElement) {
  const results = await axe.run(container, {
    rules: {
      // jsdom does not compute visual layout/color-contrast pixels; contrast is verified programmatically separately
      'color-contrast': { enabled: false },
    },
  });
  expect(
    results.violations,
    results.violations.map((v) => `${v.id}: ${v.help} (${v.nodes.length} nodes)`).join('\n'),
  ).toHaveLength(0);
}

describe('Accessibility (axe-core)', () => {
  it('CaseList (/) has zero axe violations', async () => {
    const { container } = renderAppAt('/');
    await screen.findByText('SEC v. Meridian Clearinghouse Trust');
    await expectNoAxeViolations(container);
  });

  it('CaseDetail (/case/:slug) has zero axe violations', async () => {
    const { container } = renderAppAt('/case/us-sec-clearinghouse-2024');
    await screen.findByText('Genuine gaps — what we could not verify');
    await expectNoAxeViolations(container);
  });

  it('EvidenceDetail (/evidence/:id) has zero axe violations', async () => {
    const { container } = renderAppAt(`/evidence/${MOCK_EVIDENCE_RECREATED.id}`);
    await screen.findByTestId('mandatory-source-panel');
    await expectNoAxeViolations(container);
  });

  it('CorrectionForm (/correction) has zero axe violations', async () => {
    const { container } = renderAppAt('/correction?case=us-sec-clearinghouse-2024');
    await screen.findByRole('heading', { level: 1, name: 'Report a Correction' });
    await expectNoAxeViolations(container);
  });

  it('About (/about) has zero axe violations', async () => {
    const { container } = renderAppAt('/about');
    await screen.findByRole('heading', { level: 1, name: 'About Case Files' });
    await expectNoAxeViolations(container);
  });
});
