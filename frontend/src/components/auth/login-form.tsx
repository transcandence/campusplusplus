"use client";

export function LoginForm() {
  return (
    <form>
      <h1>Welcome back</h1>
      <p>Sign in to your Campus++ account</p>

      <label htmlFor="identifier">Email or Phone</label>
      <input
        id="identifier"
        name="identifier"
        type="text"
        placeholder="Email or phone number"
        required
      />

      <label htmlFor="password">Password</label>
      <input
        id="password"
        name="password"
        type="password"
        placeholder="Enter your password"
        required
      />

      <button type="submit">Sign in</button>
    </form>
  );
}