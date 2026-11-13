export const queryKeys = {
  images: ["images"] as const,
  image: (imageID: string) => ["image", imageID] as const,
  recentMatches: ["matches", "recent"] as const,
  recentMatchesPage: (limit: number) => ["matches", "recent", limit] as const,
  matchHistory: ["match-history"] as const,
  reviewHistory: (suggestionID: string) => ["review-history", suggestionID] as const,
};
