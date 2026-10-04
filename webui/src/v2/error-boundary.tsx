import { Component, type ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export class AppErrorBoundary extends Component<
  { children: ReactNode },
  { failed: boolean }
> {
  state = { failed: false };
  static getDerivedStateFromError() {
    return { failed: true };
  }
  render() {
    if (!this.state.failed) return this.props.children;
    return (
      <main className="flex min-h-screen items-center justify-center px-5 py-12">
        <Card className="w-full max-w-md" role="alert">
          <CardHeader>
            <CardTitle>This page could not load</CardTitle>
          </CardHeader>
          <CardContent className="space-y-5">
            <p className="text-sm leading-6 text-muted-foreground">
              Refresh the application to get the latest version and try again.
            </p>
            <Button onClick={() => window.location.reload()}>
              Reload application
            </Button>
          </CardContent>
        </Card>
      </main>
    );
  }
}
