import type { TokenResponse } from "../identity/identity.types";

export interface HttpClientConfig {
  baseUrl: string;
  getAccessToken: () => string | null;
  setTokens: (tokens: TokenResponse) => void;
  clearTokens: () => void;
  onAuthFailure?: () => void;
}

export interface HttpClient {
  request<T>(
    path: string,
    options?: RequestInit,
    requiresAuth?: boolean,
  ): Promise<T>;

  setTokens(tokens: TokenResponse): void;
  clearTokens(): void;
}