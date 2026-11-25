
"use client";

import {
  createHttpClient,
  createIdentityApi,
} from "@platform/api-client";

import * as tokenStorage from "./token-storage";

const http = createHttpClient({
  baseUrl: process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080",

  getAccessToken: tokenStorage.getAccessToken,
  setTokens: tokenStorage.setTokens,
  clearTokens: tokenStorage.clearTokens,
});

export const identityApi = createIdentityApi(http);
