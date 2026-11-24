"use client";

export default function RegisterForm() {
    return (
        <form>
            <h1>Register</h1>
            <p>create an account</p>
            <label htmlFor="Fullname">Full Name</label>
            <input
                id="Fullname"
                name="Fullname"
                type="text"
                placeholder="Enter your full name"
                required
            />
            <label htmlFor="email">Email</label>
            <input
                id="email"
                name="email"
                type="email"
                placeholder="Enter your email"
                required
            />
            <label htmlFor="phone">Phone</label>
            <input
                id="phone"
                name="phone"
                type="tel"
                placeholder="Enter your phone number"
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
            <label htmlFor="confirmPassword">Confirm Password</label>
            <input
                id="confirmPassword"
                name="confirmPassword"
                type="password"
                placeholder="Confirm your password"
                required
            />
            <label htmlFor="UserType">User Type</label>
            <select id="UserType" name="UserType" required>
                <option value="">Select User Type</option>
                <option value="student">Student</option>
                <option value="guest">Guest</option>
            </select>
            <label htmlFor="gender">Gender</label>
            <select id="gender" name="gender" required>
                <option value="">Select Gender</option>
                <option value="male">Male</option>
                <option value="female">Female</option>
                <option value="other">Other</option>
            </select>
            <button type="submit">Register</button>
        </form>
    )
}