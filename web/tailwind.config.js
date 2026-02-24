/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,jsx}"],
  theme: {
    extend: {
      colors: {
        bg: "#f5f6f8",
        panel: "#ffffff",
        border: "#e5e7eb",
        text: "#111827",
        muted: "#6b7280",
        accent: "#111827",
        ok: "#0d9488",
        warn: "#dc2626"
      }
    }
  },
  plugins: []
};
