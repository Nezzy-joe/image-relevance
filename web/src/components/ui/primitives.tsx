import type { ButtonHTMLAttributes, HTMLAttributes, PropsWithChildren } from "react";

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "secondary" | "ghost" | "danger";
  size?: "sm" | "md" | "icon";
};

export function Button({ className = "", variant = "secondary", size = "md", type = "button", ...props }: ButtonProps) {
  return <button type={type} className={`button button-${variant} button-${size} ${className}`} {...props} />;
}

export function Card({ className = "", ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={`panel ${className}`} {...props} />;
}

export function CardHeading({ children, className = "" }: PropsWithChildren<{ className?: string }>) {
  return <h2 className={`card-heading ${className}`}>{children}</h2>;
}

export function StatusPill({ status, children }: PropsWithChildren<{ status: string }>) {
  const kind = status.toLowerCase().replaceAll("_", "-");
  return <span className={`status-pill status-${kind}`}>{children ?? status.replaceAll("_", " ")}</span>;
}

export function Skeleton({ className = "" }: { className?: string }) {
  return <div aria-hidden="true" className={`skeleton ${className}`} />;
}

export function SectionEyebrow({ children }: PropsWithChildren) {
  return <p className="eyebrow">{children}</p>;
}
