
import { describe, it, expect, beforeEach, vi } from "vitest";

import type { HttpClient } from "../client/types";

import { createIdentityApi } from "./identity.api";

import type {
  LoginRequest,
  RegisterRequest,
  RegisterResponse,
  TokenResponse,
  UserMe,
} from "./identity.types";

describe("Identity API", () => {
  let http: HttpClient;
  let api: ReturnType<typeof createIdentityApi>;

  beforeEach(() => {
    http = {
      request: vi.fn(),
      setTokens: vi.fn(),
      clearTokens: vi.fn(),
    } as unknown as HttpClient;

    api = createIdentityApi(http);
  });

  // --------------------------------------------------
  // REGISTER
  // --------------------------------------------------

  describe("register", () => {
    it("should send registration data to POST /auth/register", async () => {
      const registrationData = {
        fullName: "John Doe",
        email: "john@example.com",
        password: "Password123!",
        userType: "student",
      } as RegisterRequest;

      const createdUser = {
        id: "user-123",
        email: "john@example.com",
        firstName: "John",
        lastName: "Doe",
      } as RegisterResponse;

      vi.mocked(http.request).mockResolvedValue(createdUser);

      const result = await api.register(registrationData);

      expect(http.request).toHaveBeenCalledWith(
        "/auth/register",
        {
          method: "POST",
          body: JSON.stringify(registrationData),
        },
        false,
      );

      expect(result).toEqual(createdUser);
      expect(http.setTokens).not.toHaveBeenCalled();
    });

    it("should propagate registration errors", async () => {
      const registrationData = {
        fullName: "John Doe",
        email: "john@example.com",
        password: "Password123!",
        userType: "student",
      } as RegisterRequest;

      const error = new Error("Email already exists");

      vi.mocked(http.request).mockRejectedValue(error);

      await expect(api.register(registrationData)).rejects.toThrow(
        "Email already exists",
      );
    });
  });

  // --------------------------------------------------
  // LOGIN
  // --------------------------------------------------

  describe("login", () => {
    it("should send credentials and save returned tokens", async () => {
      const credentials = {
        email: "john@example.com",
        password: "Password123!",
      } as LoginRequest;

      const tokens = {
        accessToken: "access-token",
        refreshToken: "refresh-token",
      } as TokenResponse;

      vi.mocked(http.request).mockResolvedValue(tokens);

      const result = await api.login(credentials);

      expect(http.request).toHaveBeenCalledWith(
        "/auth/login",
        {
          method: "POST",
          body: JSON.stringify(credentials),
          credentials: "include",
        },
        false,
      );

      expect(http.setTokens).toHaveBeenCalledWith(tokens);
      expect(result).toEqual(tokens);
    });

    it("should not save tokens if login fails", async () => {
      vi.mocked(http.request).mockRejectedValue(
        new Error("Invalid credentials"),
      );

      await expect(
        api.login({
          email: "john@example.com",
          password: "wrong-password",
        } as LoginRequest),
      ).rejects.toThrow("Invalid credentials");

      expect(http.setTokens).not.toHaveBeenCalled();
    });
  });

  // --------------------------------------------------
  // GET ME
  // --------------------------------------------------

  describe("getMe", () => {
    it("should fetch the current user from GET /me", async () => {
      const user = {
        id: "user-123",
        email: "john@example.com",
        firstName: "John",
        lastName: "Doe",
      } as UserMe;

      vi.mocked(http.request).mockResolvedValue(user);

      const result = await api.getMe();

      expect(http.request).toHaveBeenCalledWith("/me");
      expect(result).toEqual(user);
    });

    it("should propagate errors when fetching the user", async () => {
      vi.mocked(http.request).mockRejectedValue(
        new Error("Unauthorized"),
      );

      await expect(api.getMe()).rejects.toThrow("Unauthorized");
    });
  });

  // --------------------------------------------------
  // LOGOUT
  // --------------------------------------------------

  describe("logout", () => {
    it("should call POST /auth/logout and clear tokens", async () => {
      vi.mocked(http.request).mockResolvedValue(undefined);

      await api.logout();

      expect(http.request).toHaveBeenCalledWith(
        "/auth/logout",
        {
          method: "POST",
          credentials: "include",
        },
        false,
      );

      expect(http.clearTokens).toHaveBeenCalledTimes(1);
    });

    it("should clear tokens even if logout fails", async () => {
      vi.mocked(http.request).mockRejectedValue(
        new Error("Logout failed"),
      );

      await expect(api.logout()).rejects.toThrow("Logout failed");

      expect(http.clearTokens).toHaveBeenCalledTimes(1);
    });
  });

  // --------------------------------------------------
  // LOGOUT ALL
  // --------------------------------------------------

  describe("logoutAll", () => {
    it("should call POST /auth/logout-all and clear tokens", async () => {
      vi.mocked(http.request).mockResolvedValue(undefined);

      await api.logoutAll();

      expect(http.request).toHaveBeenCalledWith(
        "/auth/logout-all",
        {
          method: "POST",
        },
      );

      expect(http.clearTokens).toHaveBeenCalledTimes(1);
    });

    it("should not clear tokens if logout-all fails", async () => {
      vi.mocked(http.request).mockRejectedValue(
        new Error("Request failed"),
      );

      await expect(api.logoutAll()).rejects.toThrow("Request failed");

      expect(http.clearTokens).not.toHaveBeenCalled();
    });
  });
});
