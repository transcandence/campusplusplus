
"use client";

import { useEffect, useRef, useState, type SubmitEvent } from "react";
import type { SVGProps, KeyboardEvent } from "react";
import Link from "next/link";
import { identityApi } from "../../lib/auth/api";
import styles from "./register-form.module.css";

type RegisterPayload = {
  fullName: string;
  password: string;
  userType: "student" | "worker" | "guest";
  gender?: "male" | "female" | "other";
  email?: string;
  phone?: string;
  subtypeAttributes?: Record<string, unknown>;
};

function Svg({d,...props}: { d: string } & SVGProps<SVGSVGElement>) 
{
  return (
    <svg
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
      aria-hidden="true"
      {...props}
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d={d}
      />
    </svg>
  );
}

const ICON = {
  user: "M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z",
  users: "M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z",
  mail: "M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z",
  phone: "M3 5a2 2 0 012-2h3.28a1 1 0 01.948.684l1.498 4.493a1 1 0 01-.502 1.21l-2.257 1.13a11.042 11.042 0 005.516 5.516l1.13-2.257a1 1 0 011.21-.502l4.493 1.498a1 1 0 01.684.949V19a2 2 0 01-2 2h-1C9.716 21 3 14.284 3 6V5z",
  lock: "M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z",
  idCard: "M10 6H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V8a2 2 0 00-2-2h-5m-4 0V5a2 2 0 114 0v1m-4 0a2 2 0 104 0m-5 8a2 2 0 100-4 2 2 0 000 4zm0 0c1.306 0 2.417.835 2.83 2M9 14a3.001 3.001 0 00-2.83 2M15 11h3m-3 4h2",
  eye: "M15 12a3 3 0 11-6 0 3 3 0 016 0zM2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z",
  eyeOff: "M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21",
  alert: "M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z",
  check: "M5 13l4 4L19 7",
  arrow: "M14 5l7 7m0 0l-7 7m7-7H3",
  chevron: "M19 9l-7 7-7-7",
};

/* Password input */

function PasswordInput({
  id,
  name,
  placeholder,
  autoComplete,
  value,
  onChange,
}: {
  id: string;
  name: string;
  placeholder: string;
  autoComplete: string;
  value: string;
  onChange: (value: string) => void;
}) {
  const [show, setShow] = useState(false);

  return (
    <div className={styles.field}>
      <Svg d={ICON.lock} className={styles.fieldIcon} />

      <input
        id={id}
        name={name}
        type={show ? "text" : "password"}
        placeholder={placeholder}
        autoComplete={autoComplete}
        required
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className={styles.input}
      />

      <button
        type="button"
        className={styles.passwordToggle}
        onClick={() => setShow((current) => !current)}
        aria-label={show ? "Hide password" : "Show password"}
        aria-pressed={show}
      >
        <Svg
          d={show ? ICON.eyeOff : ICON.eye}
          className={styles.passwordToggleIcon}
        />
      </button>
    </div>
  );
}

/* Custom dropdown */

type Option = {
  value: string;
  label: string;
};

function Select({
  id,
  name,
  placeholder,
  options,
  iconPath,
}: {
  id: string;
  name: string;
  placeholder: string;
  options: Option[];
  iconPath: string;
}) {
  const [value, setValue] = useState("");
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);

  const wrapRef = useRef<HTMLDivElement>(null);
  const hiddenRef = useRef<HTMLInputElement>(null);

  const selected = options.find(
    (option) => option.value === value && option.value !== "",
  );

  useEffect(() => {
    const form = hiddenRef.current?.form;
    if (!form) return;

    const reset = () => setValue("");

    form.addEventListener("reset", reset);

    return () => form.removeEventListener("reset", reset);
  }, []);

  useEffect(() => {
    if (!open) return;

    const onDown = (event: MouseEvent) => {
      if (!wrapRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };

    document.addEventListener("mousedown", onDown);

    return () => document.removeEventListener("mousedown", onDown);
  }, [open]);

  function openList() {
    setActive(Math.max(0, options.findIndex((option) => option.value === value)));
    setOpen(true);
  }

  function choose(index: number) {
    setValue(options[index].value);
    setOpen(false);
  }

  function onKeyDown(event: KeyboardEvent<HTMLButtonElement>) {
    switch (event.key) {
      case "ArrowDown":
        event.preventDefault();

        if (!open) {
          openList();
        } else {
          setActive((current) => Math.min(current + 1, options.length - 1));
        }
        break;

      case "ArrowUp":
        event.preventDefault();

        if (!open) {
          openList();
        } else {
          setActive((current) => Math.max(current - 1, 0));
        }
        break;

      case "Home":
        if (open) {
          event.preventDefault();
          setActive(0);
        }
        break;

      case "End":
        if (open) {
          event.preventDefault();
          setActive(options.length - 1);
        }
        break;

      case "Enter":
      case " ":
        event.preventDefault();

        if (open) {
          choose(active);
        } else {
          openList();
        }
        break;

      case "Escape":
        if (open) {
          event.preventDefault();
          setOpen(false);
        }
        break;

      case "Tab":
        setOpen(false);
        break;
    }
  }

  return (
    <div ref={wrapRef} className={styles.selectWrapper}>
      <input
        ref={hiddenRef}
        type="hidden"
        name={name}
        value={value}
      />

      <button
        type="button"
        id={id}
        role="combobox"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={`${id}-list`}
        aria-activedescendant={open ? `${id}-opt-${active}` : undefined}
        onClick={() => (open ? setOpen(false) : openList())}
        onKeyDown={onKeyDown}
        className={styles.selectButton}
      >
        <Svg d={iconPath} className={styles.fieldIcon} />

        <span
          className={`${styles.selectText} ${
            selected ? styles.selectTextSelected : styles.selectTextPlaceholder
          }`}
        >
          {selected ? selected.label : placeholder}
        </span>

        <Svg
          d={ICON.chevron}
          className={`${styles.chevron} ${open ? styles.chevronOpen : ""}`}
        />
      </button>

      {open && (
        <ul
          id={`${id}-list`}
          role="listbox"
          aria-labelledby={id}
          className={styles.optionsList}
        >
          {options.map((option, index) => {
            const isSelected = option.value === value;

            return (
              <li
                key={option.value || "none"}
                id={`${id}-opt-${index}`}
                role="option"
                aria-selected={isSelected}
                onMouseEnter={() => setActive(index)}
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => choose(index)}
                className={`${styles.option} ${
                  isSelected
                    ? styles.optionSelected
                    : index === active
                      ? styles.optionActive
                      : ""
                }`}
              >
                <span>{option.label}</span>

                {isSelected && (
                  <Svg d={ICON.check} className={styles.optionCheck} />
                )}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}

/* Registration form */

export default function RegisterForm() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");

  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");

  const hasMinLength = password.length >= 8;
  const hasLetter = /[a-zA-Z]/.test(password);
  const hasDigit = /[0-9]/.test(password);

  const isPasswordValid = hasMinLength && hasLetter && hasDigit;

  const passwordsMatch =
    password === confirmPassword && confirmPassword.length > 0;

  async function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();

    setError("");
    setSuccess("");

    if (!isPasswordValid) {
      setError(
        "Password must contain at least 8 characters, one letter, and one number.",
      );
      return;
    }

    if (!passwordsMatch) {
      setError("Passwords do not match.");
      return;
    }

    const form = event.currentTarget;
    const formData = new FormData(form);

    const fullName = String(formData.get("fullName") ?? "").trim();
    const email = String(formData.get("email") ?? "").trim();
    const phone = String(formData.get("phone") ?? "").trim();

    const userType = String(formData.get("userType") ?? "");
    const gender = String(formData.get("gender") ?? "");

    if (!email && !phone) {
      setError("Please enter an email address or phone number.");
      return;
    }

    if (!userType) {
      setError("Please select a user type.");
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
      setPassword("");
      setConfirmPassword("");
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

  const passwordRules = [
    { valid: hasMinLength, text: "At least 8 characters" },
    { valid: hasLetter, text: "At least one letter (A-Z)" },
    { valid: hasDigit, text: "At least one number (0-9)" },
  ];

  return (
    <form onSubmit={handleSubmit} className={styles.form}>
      <div className={styles.heading}>
        <h1 className={styles.title}>Register</h1>
        <p className={styles.subtitle}>Create an account</p>
      </div>

      <fieldset disabled={loading} className={styles.fields}>
        {error && (
          <div className={styles.errorMessage} role="alert">
            <Svg d={ICON.alert} className={styles.messageIcon} />
            <span>{error}</span>
          </div>
        )}

        {success && (
          <div className={styles.successMessage} role="status">
            <Svg d={ICON.check} className={styles.messageIcon} />
            <span>{success}</span>
          </div>
        )}

        <div className={styles.formGroup}>
          <label className={styles.label} htmlFor="fullName">
            Full Name
          </label>

          <div className={styles.field}>
            <Svg d={ICON.user} className={styles.fieldIcon} />
            <input
              id="fullName"
              name="fullName"
              type="text"
              placeholder="Enter your full name"
              autoComplete="name"
              required
              className={styles.input}
            />
          </div>
        </div>

        <div className={styles.formGroup}>
          <label className={styles.label} htmlFor="email">
            Email
          </label>

          <div className={styles.field}>
            <Svg d={ICON.mail} className={styles.fieldIcon} />
            <input
              id="email"
              name="email"
              type="email"
              placeholder="Enter your email"
              autoComplete="email"
              className={styles.input}
            />
          </div>
        </div>

        <div className={styles.formGroup}>
          <label className={styles.label} htmlFor="phone">
            Phone
          </label>

          <div className={styles.field}>
            <Svg d={ICON.phone} className={styles.fieldIcon} />
            <input
              id="phone"
              name="phone"
              type="tel"
              placeholder="Enter your phone number"
              autoComplete="tel"
              className={styles.input}
            />
          </div>

          <p className={styles.helper}>
            You must provide an email address or a phone number.
          </p>
        </div>

        <div className={styles.formGroup}>
          <label className={styles.label} htmlFor="password">
            Password
          </label>

          <PasswordInput
            id="password"
            name="password"
            placeholder="Enter your password"
            autoComplete="new-password"
            value={password}
            onChange={setPassword}
          />

          <div className={styles.passwordRules}>
            {passwordRules.map((rule) => (
              <div
                key={rule.text}
                className={`${styles.passwordRule} ${
                  rule.valid ? styles.ruleValid : styles.ruleInvalid
                }`}
              >
                <Svg
                  d={rule.valid ? ICON.check : ICON.alert}
                  className={styles.ruleIcon}
                />
                {rule.text}
              </div>
            ))}
          </div>
        </div>

        <div className={styles.formGroup}>
          <label className={styles.label} htmlFor="confirmPassword">
            Confirm Password
          </label>

          <PasswordInput
            id="confirmPassword"
            name="confirmPassword"
            placeholder="Confirm your password"
            autoComplete="new-password"
            value={confirmPassword}
            onChange={setConfirmPassword}
          />

          {confirmPassword.length > 0 && (
            <p
              className={`${styles.passwordFeedback} ${
                passwordsMatch ? styles.feedbackValid : styles.feedbackInvalid
              }`}
            >
              {passwordsMatch
                ? "Passwords match."
                : "Passwords do not match."}
            </p>
          )}
        </div>

        <div className={styles.formGroup}>
          <label className={styles.label} htmlFor="userType">
            User Type
          </label>

          <Select
            id="userType"
            name="userType"
            placeholder="Select User Type"
            iconPath={ICON.idCard}
            options={[
              { value: "student", label: "Student" },
              { value: "worker", label: "Worker" },
              { value: "guest", label: "Guest" },
            ]}
          />
        </div>

        <div className={styles.formGroup}>
          <label className={styles.label} htmlFor="gender">
            Gender
          </label>

          <Select
            id="gender"
            name="gender"
            placeholder="Select Gender"
            iconPath={ICON.users}
            options={[
              { value: "", label: "Prefer not to say" },
              { value: "male", label: "Male" },
              { value: "female", label: "Female" },
              { value: "other", label: "Other" },
            ]}
          />
        </div>

        <button
          type="submit"
          disabled={loading || !isPasswordValid || !passwordsMatch}
          aria-busy={loading}
          className={styles.submitButton}
        >
          {loading && (
            <svg
              className={styles.spinner}
              viewBox="0 0 24 24"
              fill="none"
              aria-hidden="true"
            >
              <circle
                cx="12"
                cy="12"
                r="9"
                stroke="currentColor"
                strokeWidth="3"
                opacity="0.25"
              />
              <path
                d="M21 12a9 9 0 00-9-9"
                stroke="currentColor"
                strokeWidth="3"
                strokeLinecap="round"
              />
            </svg>
          )}

          <span>{loading ? "Creating account..." : "Register"}</span>

          {!loading && (
            <Svg d={ICON.arrow} className={styles.submitArrow} />
          )}
        </button>

        <p className={styles.loginPrompt}>
          Already have an account?{" "}
          <Link href="/login" className={styles.loginLink}>
            Log in
          </Link>
        </p>
      </fieldset>
    </form>
  );
}
