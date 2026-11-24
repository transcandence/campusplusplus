
import type { components, operations } from "../generated/identity";

// Request DTOs
export type LoginRequest = components["schemas"]["LoginRequest"];
export type RefreshRequest = components["schemas"]["RefreshRequest"];
export type LogoutRequest = components["schemas"]["LogoutRequest"];
export type RegisterRequest = components["schemas"]["RegisterRequest"]
// Response DTOs
export type TokenResponse =
  operations["login"]["responses"][200]["content"]["application/json"];
  
export type RegisterResponse =
  operations["register"]["responses"][201]["content"]["application/json"];

export type UserMe =
  operations["getMe"]["responses"][200]["content"]["application/json"];

export type Problem = components["schemas"]["Problem"];
