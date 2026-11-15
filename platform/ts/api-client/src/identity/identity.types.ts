
import type { components, operations } from "../generated/identity";

// Request DTOs
export type LoginRequest = components["schemas"]["LoginRequest"];

// Response DTOs
export type TokenResponse =
  operations["login"]["responses"][200]["content"]["application/json"];

export type UserMe =
  operations["getMe"]["responses"][200]["content"]["application/json"];

export type Problem = components["schemas"]["Problem"];
