
"use client";

import { useState, type SubmitEvent } from "react";
import { identityApi } from "../../lib/auth/api";

type RegisterPayload = {
  fullName: string;
  password: string;
  userType: "student" | "worker" | "guest";
  gender?: "male" | "female" | "other";
  email?: string;
  phone?: string;
  subtypeAttributes?: Record<string, unknown>;
};

export default function RegisterForm() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");

  async function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();

    setError("");
    setSuccess("");

    const form = event.currentTarget;
    const formData = new FormData(form);

    const fullName = String(formData.get("fullName") ?? "").trim();
    const email = String(formData.get("email") ?? "").trim();
    const phone = String(formData.get("phone") ?? "").trim();
    const password = String(formData.get("password") ?? "");
    const confirmPassword = String(
      formData.get("confirmPassword") ?? "",
    );

    const userType = String(formData.get("userType") ?? "");
    const gender = String(formData.get("gender") ?? "");

    // At least one contact method is required.
    if (!email && !phone) {
      setError("Please enter an email address or phone number.");
      return;
    }

    // Password confirmation is checked only on the frontend.
    if (password !== confirmPassword) {
      setError("Passwords do not match.");
      return;
    }

    const payload: RegisterPayload = {
      fullName,
      password,
      userType: userType as RegisterPayload["userType"],
      ...(email && { email }),
      ...(phone && { phone }),
      ...(gender && {
        gender: gender as NonNullable<RegisterPayload["gender"]>,
      }),
    };

    try {
      setLoading(true);

      const registeredUser = await identityApi.register(payload);

      console.log("Registered user:", registeredUser);

      setSuccess("Your account has been created successfully.");
      form.reset();
    } catch (err) {
      console.error("Registration error:", err);

      setError(
        err instanceof Error
          ? err.message
          : "Registration failed. Please try again.",
      );
    } finally {
      setLoading(false);
    }
  }

  return (
    <form onSubmit={handleSubmit}>
      <h1>Register</h1>
      <p>Create an account</p>

      <div>
        <label htmlFor="fullName">Full Name</label>
        <input
          id="fullName"
          name="fullName"
          type="text"
          placeholder="Enter your full name"
          required
        />
      </div>

      <div>
        <label htmlFor="email">Email</label>
        <input
          id="email"
          name="email"
          type="email"
          placeholder="Enter your email"
        />
      </div>

      <div>
        <label htmlFor="phone">Phone</label>
        <input
          id="phone"
          name="phone"
          type="tel"
          placeholder="Enter your phone number"
        />
      </div>

      <p>You must provide an email address or a phone number.</p>

      <div>
        <label htmlFor="password">Password</label>
        <input
          id="password"
          name="password"
          type="password"
          placeholder="Enter your password"
          required
        />
      </div>

      <div>
        <label htmlFor="confirmPassword">Confirm Password</label>
        <input
          id="confirmPassword"
          name="confirmPassword"
          type="password"
          placeholder="Confirm your password"
          required
        />
      </div>

      <div>
        <label htmlFor="userType">User Type</label>
        <select id="userType" name="userType" required defaultValue="">
          <option value="" disabled>
            Select User Type
          </option>
          <option value="student">Student</option>
          <option value="worker">Worker</option>
          <option value="guest">Guest</option>
        </select>
      </div>

      <div>
        <label htmlFor="gender">Gender</label>
        <select id="gender" name="gender" defaultValue="">
          <option value="">Select Gender</option>
          <option value="male">Male</option>
          <option value="female">Female</option>
          <option value="other">Other</option>
        </select>
      </div>

      {error && <p role="alert">{error}</p>}

      {success && <p role="status">{success}</p>}

      <button type="submit" disabled={loading}>
        {loading ? "Creating account..." : "Register"}
      </button>
    </form>
  );
}
