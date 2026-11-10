"use client";

import {
  createHttpClient,
  createIdentityApi,
} from "@platform/api-client";

import * as tokenStorage from "./token-storage";

// 1. Create the shared HTTP client
const http = createHttpClient({
  baseUrl: process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080",

  getAccessToken: tokenStorage.getAccessToken,

  setTokens: tokenStorage.setTokens,
  clearTokens: tokenStorage.clearTokens,
});

// 2. Create the Identity API using that HTTP client
export const identityApi = createIdentityApi(http);
identityApi.login()
