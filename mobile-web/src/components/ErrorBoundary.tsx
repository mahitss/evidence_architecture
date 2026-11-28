import { Component, type ErrorInfo, type ReactNode } from 'react';
import { Navigate } from 'react-router-dom';
import { GeoblockError } from '../lib/api';
import { Button, Callout } from './ui';

interface ErrorBoundaryProps {
  children: ReactNode;
}

interface ErrorBoundaryState {
  error: Error | null;
}

export class AppErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error };
  }

  componentDidCatch(_error: Error, _info: ErrorInfo) {
    // Error boundary captures unexpected UI crashes without sending third-party telemetry.
  }

  render() {
    const { error } = this.state;
    if (!error) {
      return this.props.children;
    }

    if (error instanceof GeoblockError) {
      return <Navigate to="/geoblocked" replace />;
    }

    return (
      <div className="max-w-xl mx-auto px-4 py-12">
        <Callout variant="danger" title="Something went wrong displaying this record">
          <p className="m-0 mb-4">
            An unexpected display error occurred. You can reload the view or return to the case
            index.
          </p>
          <div className="flex flex-wrap gap-3">
            <Button
              variant="primary"
              onClick={() => {
                this.setState({ error: null });
              }}
            >
              Try again
            </Button>
            <Button
              variant="secondary"
              onClick={() => {
                this.setState({ error: null });
                window.location.assign('/');
              }}
            >
              Return to Cases
            </Button>
          </div>
        </Callout>
      </div>
    );
  }
}
