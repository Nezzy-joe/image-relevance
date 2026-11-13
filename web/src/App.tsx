import { QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import { AppShell, PlaceholderPage } from "./components/layout/AppShell";
import { queryClient } from "./lib/query-client";
import { ImageLibraryPage } from "./pages/ImageLibraryPage";
import { OverviewPage } from "./pages/OverviewPage";
import { MatchingStudioPage } from "./pages/MatchingStudioPage";
import { DecisionInspectorPage } from "./pages/DecisionInspectorPage";
import { ReviewCentrePage } from "./pages/ReviewCentrePage";

function NotFoundPage() {
  return <PlaceholderPage title="Page not found" description="This route is not part of the current workspace." />;
}

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AppShell>
          <Routes>
            <Route path="/" element={<OverviewPage />} />
            <Route path="/images" element={<ImageLibraryPage />} />
            <Route path="/matching" element={<MatchingStudioPage />} />
            <Route path="/decisions" element={<DecisionInspectorPage />} />
            <Route path="/reviews" element={<ReviewCentrePage />} />
            <Route path="*" element={<NotFoundPage />} />
          </Routes>
        </AppShell>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
