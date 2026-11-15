import type { Metadata } from "next";
import { LocaleProvider } from "./components/locale";
import "./globals.css";

export const metadata: Metadata = {
  icons: { icon: "/picofarm_logo.png" },
  title: "PicoFarm — Organize your 3D printing",
  description:
    "Open-source 3D printer management for one printer or a small farm. Organize your queue, files and filament. Self-hosted BETA, developed with AI.",
  openGraph: {
    title: "PicoFarm — Organize your 3D printing",
    description:
      "Your printers, queue, files and filament in one place. Join the open-source, self-hosted PicoFarm BETA.",
    type: "website",
  },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="anonymous" />
        <link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500;600&family=Space+Grotesk:wght@400;500;600;700&display=swap" rel="stylesheet" />
      </head>
      <body className="bg-surface-950 text-surface-100 antialiased">
        <LocaleProvider>{children}</LocaleProvider>
      </body>
    </html>
  );
}
