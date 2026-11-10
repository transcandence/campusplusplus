
import type { components, operations } from "../generated/identity";

// Request DTOs
export type LoginRequest = components["schemas"]["LoginRequest"];
export type RefreshRequest = components["schemas"]["RefreshRequest"];
export type LogoutRequest = components["schemas"]["LogoutRequest"];

// Response DTOs
export type TokenResponse =
  operations["login"]["responses"][200]["content"]["application/json"];

export type UserMe =
  operations["getMe"]["responses"][200]["content"]["application/json"];

export type Problem = components["schemas"]["Problem"];
