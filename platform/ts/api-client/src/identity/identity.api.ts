
import type { HttpClient } from "../client/types";

import type {
  LoginRequest,
  TokenResponse,
  UserMe,
} from "./identity.types";

export function createIdentityApi(http: HttpClient) {
  return {
    async login(credentials: LoginRequest,): Promise<TokenResponse>
    {
      const tokens = await http.request<TokenResponse>(
        "/auth/login",
        {
          method: "POST",
          body: JSON.stringify(credentials),
          credentials: "include",
        },
        false,
      );

      http.setTokens(tokens);
      return tokens;
    },

    getMe(): Promise<UserMe> 
    {
      return http.request<UserMe>("/me");
    },

    async logout(): Promise<void> 
    {
      try 
      {
        await http.request<void>(
          "/auth/logout",
          {
            method: "POST",
            credentials: "include",
          },
          false,
        );
      } 
      finally 
      {
        http.clearTokens();
      }
    },

    async logoutAll(): Promise<void> 
    {
      await http.request<void>("/auth/logout-all", {
        method: "POST",
      });

      http.clearTokens();
    },
  };
}
