
"use client";

import { useState, type SubmitEvent } from "react";
import { identityApi } from "../../lib/auth/api";

type LoginPayload = {
    email: string;
    password: string;
};

export default function LoginForm() {
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState("");
    const [success, setSuccess] = useState("");

    async function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
        event.preventDefault();

        setError("");
        setSuccess("");

        const form = event.currentTarget;
        const formData = new FormData(form);

        const email = String(formData.get("identifier") ?? "").trim();
        const password = String(formData.get("password") ?? "");

        if (!email) {
            setError("Please enter your email or phone number");
            return;
        }

        if (!password) {
            setError("Please enter your password");
            return;
        }

        const payload: LoginPayload = {
            email,
            password,
        };

        try {
            setLoading(true);

            const loggedUser = await identityApi.login(payload);
            console.log("Logged user:", loggedUser);

            setSuccess("You have logged in successfully!");
            form.reset();
        } catch (err) {
            console.error("Login error:", err);

            setError(
                err instanceof Error
                    ? err.message
                    : "Login failed, please try again."
            );
        } finally {
            setLoading(false);
        }
    }

    return (
        <form onSubmit={handleSubmit}>
            <h1>Welcome back</h1>
            <p>Sign in to your Campus++ account</p>

            <div>
                <label htmlFor="identifier">Email or Phone</label>
                <input
                    id="identifier"
                    name="identifier"
                    type="text"
                    placeholder="Email or phone number"
                    required
                />
            </div>

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

            {error && <p role="alert">{error}</p>}
            {success && <p role="status">{success}</p>}

            <button type="submit" disabled={loading}>
                {loading ? "Logging in..." : "Sign in"}
            </button>
        </form>
    );
}
