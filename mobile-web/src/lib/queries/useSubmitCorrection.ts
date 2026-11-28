import { useMutation } from '@tanstack/react-query';
import { submitCorrection } from '../api';
import type { SubmitCorrectionInput } from '../schemas';

export function useSubmitCorrection() {
  return useMutation({
    mutationFn: (input: SubmitCorrectionInput) => submitCorrection(input),
  });
}
