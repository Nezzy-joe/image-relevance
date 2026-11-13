import { QueryClient } from "@tanstack/react-query";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 15_000,
      refetchOnWindowFocus: false,
      retry: (failureCount, error) => {
        if (failureCount >= 1 || (error instanceof Error && "status" in error && error.status === 0)) return false;
        return true;
      },
    },
    mutations: { retry: false },
  },
});
