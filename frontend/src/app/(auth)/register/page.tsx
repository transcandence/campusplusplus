import type { Metadata } from "next";
import { Hanken_Grotesk } from "next/font/google";
import Link from "next/link";

import RegisterForm from "@/components/auth/register-form";
import ThemeToggle from "@/components/auth/theme-toggle";

const hanken = Hanken_Grotesk({ subsets: ["latin"], weight: ["300", "400", "500", "600", "700", "800"], display: "swap" });

export const metadata: Metadata = { title: "Register · Campus++" };

// Applies the saved/system theme before first paint (no flash)
const THEME_SCRIPT = `try{var t=localStorage.getItem("theme");var d=t==="dark"||(t!=="light"&&matchMedia("(prefers-color-scheme: dark)").matches);document.documentElement.classList.toggle("dark",d);document.documentElement.style.colorScheme=d?"dark":"light"}catch(e){}`;

const SECTIONS = [
  { label: "Housing & Facilities", href: "/#housing" },
  { label: "Transit & Shuttles", href: "/#transit" },
  { label: "Dining & Nutrition", href: "/#dining" },
  { label: "Events", href: "/#events" },
  { label: "Our Story", href: "/#story" },
];

const focus = "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-fuchsia-300";

export default function RegisterPage() {
  return (
    <div className={`${hanken.className} flex min-h-screen flex-1 flex-col justify-between bg-[#FEF6FF] text-slate-800 antialiased dark:bg-[#0F0133]`}>
      <script dangerouslySetInnerHTML={{ __html: THEME_SCRIPT }} />

      <header className="relative z-30 border-b border-white/10 bg-[#5D005D] text-white dark:bg-[#241344]">
        <div className="mx-auto flex max-w-7xl items-center justify-between gap-4 px-6 py-4 lg:px-12">
          <Link href="/" title="Campus++ home" className={`group flex items-center gap-2 text-xl font-bold tracking-tight text-white ${focus}`}>
            <span className="hidden size-9 place-items-center rounded-xl border border-white/20 bg-white/10 transition-transform group-hover:scale-105 min-[480px]:grid">
              <svg className="size-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 14l9-5-9-5-9 5 9 5zM12 14l6.16-3.422a12.083 12.083 0 01.665 6.479A11.952 11.952 0 0012 20.055a11.952 11.952 0 00-6.824-2.998 12.078 12.078 0 01.665-6.479L12 14z" />
              </svg>
            </span>
            <span>Campus<span className="text-blue-600">++</span></span>
          </Link>

          <nav className="hidden items-center gap-8 text-sm font-medium md:flex" aria-label="Main">
            {SECTIONS.map((s) => (
              <Link
                key={s.href}
                href={s.href}
                className={`relative py-1 text-white/90 after:absolute after:-bottom-1.5 after:left-1/2 after:h-0.5 after:w-0 after:-translate-x-1/2 after:rounded-full after:bg-white after:transition-all hover:after:w-full ${focus}`}
              >
                {s.label}
              </Link>
            ))}
          </nav>

          <div className="flex items-center gap-4">
            <ThemeToggle />
            <Link href="/login" className={`rounded-full border border-white/20 bg-black/30 px-5 py-2 text-sm font-semibold text-white transition hover:-translate-y-0.5 hover:bg-black/40 hover:shadow-lg motion-reduce:hover:translate-y-0 ${focus}`}>
              Log In
            </Link>
            <Link href="/login" aria-label="Account" className={`grid size-10 place-items-center rounded-full border border-white/20 bg-white/10 text-white transition hover:scale-105 hover:bg-white/20 ${focus}`}>
              <svg className="size-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
              </svg>
            </Link>
          </div>
        </div>
      </header>

      <main className="flex flex-1 items-center justify-center px-4 py-12">
        <div className="w-full max-w-lg min-w-0 rounded-3xl border border-purple-100/80 bg-white p-6 shadow-xl sm:p-10 dark:border-[#39245B] dark:bg-[#241344]">
          <RegisterForm />
        </div>
      </main>

      <footer className="border-t border-slate-200 bg-white text-xs text-slate-500 dark:border-[#39245B] dark:bg-[#18083D] dark:text-[#B8B5C8]">
        <div className="mx-auto flex max-w-7xl flex-col items-center justify-between gap-4 px-6 py-6 text-center sm:flex-row sm:text-left lg:px-12">
          <div className="flex flex-wrap items-center justify-center gap-2">
            <span className="text-sm font-bold tracking-tight text-slate-900 dark:text-white">Campus<span className="text-blue-600">++</span></span>
            <span>|</span>
            <span>Collegiate Living &amp; Smart Operations Portal</span>
          </div>
          <div className="flex flex-wrap items-center justify-center gap-x-6 gap-y-2 font-medium">
            <a href="#" className="transition-colors hover:text-slate-900 dark:hover:text-white">Privacy Policy</a>
            <a href="#" className="transition-colors hover:text-slate-900 dark:hover:text-white">Campus Security</a>
            <a href="#" className="transition-colors hover:text-slate-900 dark:hover:text-white">Help Desk</a>
            <span>&copy; 2026 Campus++ Portal</span>
          </div>
        </div>
      </footer>
    </div>
  );
}