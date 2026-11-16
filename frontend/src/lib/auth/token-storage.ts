let accessToken: string | null = null;

export function getAccessToken(): string | null 
{
  return accessToken;
}

export function setTokens(tokens: {
  accessToken: string;
}): void 
{
  accessToken = tokens.accessToken;
}

export function clearTokens(): void 
{
  accessToken = null;
}
