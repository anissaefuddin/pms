// Ilustrasi ringan untuk EmptyState. Warna lewat token (var(--...)), tanpa hex.
import type { CSSProperties, ReactNode } from "react";

const frame: CSSProperties = { fill: "var(--muted)", stroke: "var(--border)" };
const accent: CSSProperties = { fill: "var(--cat-5-subtle)", stroke: "var(--cat-5)" };
const warm: CSSProperties = { fill: "var(--cat-4-subtle)", stroke: "var(--cat-4)" };

// Decorative: the EmptyState title and text carry the meaning, so the SVG is hidden from assistive tech.
function Svg({ children }: { children: ReactNode }) {
  return (
    <svg viewBox="0 0 120 96" width="120" height="96" aria-hidden="true" focusable="false" className="mx-auto">
      {children}
    </svg>
  );
}

export function ProjectIllustration() {
  return <Svg><rect x="22" y="18" width="76" height="60" rx="6" style={frame} strokeWidth="2" />
    <rect x="32" y="30" width="34" height="5" rx="2.5" style={accent} strokeWidth="0" />
    <rect x="32" y="42" width="56" height="5" rx="2.5" style={frame} strokeWidth="0" />
    <rect x="32" y="54" width="44" height="5" rx="2.5" style={frame} strokeWidth="0" />
    <circle cx="92" cy="26" r="11" style={warm} strokeWidth="2" /></Svg>;
}

export function FilesIllustration() {
  return <Svg><path d="M26 26h28l6 6h34v40H26z" style={frame} strokeWidth="2" />
    <rect x="40" y="46" width="40" height="5" rx="2.5" style={accent} strokeWidth="0" />
    <rect x="40" y="56" width="26" height="5" rx="2.5" style={frame} strokeWidth="0" /></Svg>;
}

export function BlueprintIllustration() {
  return <Svg><rect x="20" y="16" width="80" height="64" rx="4" style={accent} strokeWidth="2" />
    <path d="M20 40h80M20 56h80M52 16v64" style={{ stroke: "var(--cat-5)" }} strokeWidth="1" strokeDasharray="3 3" />
    <rect x="58" y="46" width="24" height="16" rx="3" style={warm} strokeWidth="2" /></Svg>;
}

export function SearchIllustration() {
  return <Svg><circle cx="54" cy="44" r="22" style={frame} strokeWidth="3" />
    <path d="M70 60l22 22" style={{ stroke: "var(--cat-5)" }} strokeWidth="5" strokeLinecap="round" />
    <path d="M44 42h20" style={{ stroke: "var(--border)" }} strokeWidth="3" strokeLinecap="round" /></Svg>;
}

export function StarredIllustration() {
  return <Svg><path d="M60 16l12 26 28 3-21 19 6 28-25-14-25 14 6-28-21-19 28-3z" style={warm} strokeWidth="2" strokeLinejoin="round" /></Svg>;
}

export function GenericIllustration() {
  return <Svg><rect x="26" y="22" width="68" height="52" rx="8" style={frame} strokeWidth="2" />
    <circle cx="60" cy="48" r="8" style={accent} strokeWidth="2" /></Svg>;
}
