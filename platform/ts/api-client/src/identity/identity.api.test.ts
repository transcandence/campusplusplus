
import { describe, it, expect, vi, beforeEach } from "vitest";
import { createIdentityApi } from "./identity.api";
import type { HttpClient } from "../client/types";

describe("Identity API", () => {
  const tokens = {
    accessToken: "fake-access-token",
  };

  const request = vi.fn();

  const http = {
    request,
    setTokens: vi.fn(),
    clearTokens: vi.fn(),
  } as unknown as HttpClient;

  const identityApi = createIdentityApi(http);

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should login and save tokens", async () => {
    request.mockResolvedValue(tokens);

    const result = await identityApi.login({
      email: "test@example.com",
      password: "password123",
    });

    expect(result).toEqual(tokens);

    expect(request).toHaveBeenCalledWith(
      "/auth/login",
      {
        method: "POST",
        body: JSON.stringify({
          email: "test@example.com",
          password: "password123",
        }),
        credentials: "include",
      },
      false,
    );

    expect(http.setTokens).toHaveBeenCalledWith(tokens);
  });

  it("should get the current user without explicitly including cookies", async () => {
    const user = {
      id: "123",
      email: "test@example.com",
    };

    request.mockResolvedValue(user);

    const result = await identityApi.getMe();

    expect(result).toEqual(user);
    expect(request).toHaveBeenCalledWith("/me");
  });

  it("should logout and clear tokens", async () => {
    request.mockResolvedValue(undefined);

    await identityApi.logout();

    expect(request).toHaveBeenCalledWith(
      "/auth/logout",
      {
        method: "POST",
        credentials: "include",
      },
      false,
    );

    expect(http.clearTokens).toHaveBeenCalled();
  });

  it("should clear tokens even if logout request fails", async () => {
    request.mockRejectedValue(new Error("Network error"));

    await expect(identityApi.logout()).rejects.toThrow("Network error");

    expect(http.clearTokens).toHaveBeenCalled();
  });

  it("should logout all sessions", async () => {
    request.mockResolvedValue(undefined);

    await identityApi.logoutAll();

    expect(request).toHaveBeenCalledWith(
      "/auth/logout-all",
      { method: "POST" },
    );

    expect(http.clearTokens).toHaveBeenCalled();
  });
});
