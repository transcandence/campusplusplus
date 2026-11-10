
import { ApiError } from "./errors";
import type { HttpClient, HttpClientConfig } from "./types";
import type { TokenResponse } from "../identity/identity.types";

export function createHttpClient(
  config: HttpClientConfig,
): HttpClient {
  const baseUrl = config.baseUrl.replace(/\/+$/, "");

  let refreshPromise: Promise<TokenResponse> | null = null;

  async function parseResponse<T>(response: Response,): Promise<T> 
  {
    if (response.status === 204)
    {
      return undefined as T;
    }

    // null if backend sends something that isn't valid JSON or empty body
    let data;
    try 
    {
      data = await response.json();
    }

    catch 
    {
      data = null;
    }

    if (!response.ok)
    {
      throw new ApiError(response.status, data);
    }

    return data as T;
  }

  async function send<T>(path: string, options: RequestInit, accessToken: string | null,): Promise<T> 
  {
    const headers = new Headers(options.headers);

    if (options.body && !headers.has("Content-Type"))
    {
      headers.set("Content-Type", "application/json");
    }

    if (accessToken)
    {
      headers.set("Authorization", `Bearer ${accessToken}`);
    }

    const response = await fetch(`${baseUrl}${path}`, {...options, headers, cache: "no-store", credentials: options.credentials ?? "same-origin",});

    return parseResponse<T>(response);
  }

  async function refreshTokens(): Promise<TokenResponse> 
  {
    if (!refreshPromise) 
    {
      refreshPromise = (async () => {
        const tokens = await send<TokenResponse>(
          "/auth/refresh",
          {
            method: "POST",
          },
          null,
        );

        config.setTokens(tokens);
        return tokens;
      })();
    }

    const currentRefresh = refreshPromise;

    try 
    {
      return await currentRefresh;
    }

    finally 
    {
        if (refreshPromise === currentRefresh)
            {
                refreshPromise = null;
            }
    }
  }

  async function request<T>( path: string, options: RequestInit = {}, requiresAuth = true,): Promise<T> 
  {
    let tokenUsed;

    if (requiresAuth) 
      tokenUsed = config.getAccessToken(); 
    else 
      tokenUsed = null;

    try 
    {
      return await send<T>(path, options, tokenUsed);
    } 
    catch (error)
    {
      if (!(error instanceof ApiError) || error.status !== 401
      || !requiresAuth)
      {
        throw error;
      }

      const currentToken = config.getAccessToken();

      // Another request may already have refreshed the token.
      // Instead of refreshing the token again, retry with the new token.
      if (currentToken && currentToken !== tokenUsed)
      {
        return send<T>(path, options, currentToken);
      }

      let tokens: TokenResponse;

      try
      {
        tokens = await refreshTokens();
      } 
      catch (refreshError) 
      {
        config.clearTokens();
        config.onAuthFailure?.();
        throw refreshError;
      }

      // Retry: this request refreshed the token itself.
      try 
      {
        return await send<T>(path, options, tokens.accessToken,);
      } 
      catch (retryError) 
      {
        // The new access token was rejected.
        if (
          retryError instanceof ApiError &&
          retryError.status === 401
        ) {
          config.clearTokens();
          config.onAuthFailure?.();
        }

        throw retryError;
      }
    }
  }

  return {
    request,
    setTokens: config.setTokens,
    clearTokens: config.clearTokens,
  };
}
