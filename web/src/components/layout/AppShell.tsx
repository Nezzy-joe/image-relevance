import { useQuery } from "@tanstack/react-query";
import {
  Activity,
  ArrowUpRight,
  Check,
  ChevronRight,
  ClipboardCheck,
  GitCompareArrows,
  Image as ImageIcon,
  LayoutDashboard,
  Menu,
  ScanEye,
  X,
} from "lucide-react";
import { useState, type PropsWithChildren } from "react";
import { Link, NavLink, useLocation } from "react-router-dom";
import { api } from "../../lib/api";
import { Button } from "../ui/primitives";

const navItems = [
  { label: "Overview", to: "/", icon: LayoutDashboard },
  { label: "Image Library", to: "/images", icon: ImageIcon },
  { label: "Matching Studio", to: "/matching", icon: GitCompareArrows },
  { label: "Decision Inspector", to: "/decisions", icon: ScanEye },
  { label: "Review Centre", to: "/reviews", icon: ClipboardCheck },
];

const pageTitles: Record<string, string> = {
  "/": "Overview",
  "/images": "Image Library",
  "/matching": "Matching Studio",
  "/decisions": "Decision Inspector",
  "/reviews": "Review Centre",
};

function ConnectionStatus() {
  const health = useQuery({ queryKey: ["health"], queryFn: api.health, refetchInterval: 20_000 });
  const ready = useQuery({ queryKey: ["readiness"], queryFn: api.readiness, refetchInterval: 20_000 });
  const state = health.isError ? "offline" : ready.data?.status === "ready" ? "ready" : ready.isError || ready.data?.status === "not_ready" ? "degraded" : "checking";
  const label = state === "ready" ? "API connected" : state === "degraded" ? "Database unavailable" : state === "offline" ? "API offline" : "Connecting";
  return (
    <div className={`connection-status connection-${state}`} role="status" aria-live="polite">
      <span className="connection-dot" />
      <span>{label}</span>
    </div>
  );
}

export function AppShell({ children }: PropsWithChildren) {
  const [drawerOpen, setDrawerOpen] = useState(false);
  const { pathname } = useLocation();
  const title = pageTitles[pathname] ?? "Workspace";

  const sidebar = (
    <>
      <div className="brand-row">
        <div className="brand-mark" aria-hidden="true"><span /><span /><span /></div>
        <div className="brand-copy">
          <span className="brand-name">Image Relevance</span>
          <span className="brand-product">STUDIO</span>
        </div>
        {drawerOpen && <Button variant="ghost" size="icon" className="drawer-close" aria-label="Close navigation" onClick={() => setDrawerOpen(false)}><X size={18} /></Button>}
      </div>
      <div className="workspace-switcher">
        <div className="workspace-avatar">IR</div>
        <div className="workspace-copy"><strong>Local workspace</strong><span>Development tenant</span></div>
        <ChevronRight size={15} className="muted-icon" />
      </div>
      <nav className="side-nav" aria-label="Main navigation">
        <span className="nav-label">WORKSPACE</span>
        {navItems.map(({ label, to, icon: Icon }) => (
          <NavLink key={to} to={to} end={to === "/"} onClick={() => setDrawerOpen(false)} className={({ isActive }) => `nav-link${isActive ? " active" : ""}`}>
            <Icon size={17} strokeWidth={1.8} />
            <span>{label}</span>
          </NavLink>
        ))}
      </nav>
      <div className="sidebar-spacer" />
      <div className="sidebar-note">
        <div className="sidebar-note-icon"><Activity size={16} /></div>
        <div><strong>Local development</strong><p>Data stays on this machine.</p></div>
        <Check size={14} className="sidebar-note-check" />
      </div>
      <a className="sidebar-project-link" href="https://github.com/Nezzy-joe/image-relevance" target="_blank" rel="noreferrer">
        <span>image-relevance</span><ArrowUpRight size={14} />
      </a>
    </>
  );

  return (
    <div className="app-frame">
      <aside className="sidebar desktop-sidebar">{sidebar}</aside>
      {drawerOpen && <button className="drawer-backdrop" aria-label="Close navigation" onClick={() => setDrawerOpen(false)} />}
      <aside className={`sidebar mobile-sidebar${drawerOpen ? " open" : ""}`} aria-hidden={!drawerOpen}>{sidebar}</aside>
      <div className="main-column">
        <header className="topbar">
          <div className="topbar-left">
            <Button variant="ghost" size="icon" className="menu-trigger" aria-label="Open navigation" aria-expanded={drawerOpen} onClick={() => setDrawerOpen(true)}><Menu size={19} /></Button>
            <div className="crumb"><span>Workspace</span><ChevronRight size={13} /><strong>{title}</strong></div>
          </div>
          <div className="topbar-right">
            <ConnectionStatus />
            <span className="environment-chip"><span />LOCAL DEVELOPMENT</span>
          </div>
        </header>
        <main className="page-content" id="main-content">{children}</main>
        <footer className="app-footer"><span>Image Relevance Studio</span><span>Development workspace <i /> API backed</span></footer>
      </div>
    </div>
  );
}

export function PlaceholderPage({ title, description }: { title: string; description: string }) {
  return (
    <div className="placeholder-page">
      <div className="placeholder-orbit"><GitCompareArrows size={27} strokeWidth={1.35} /></div>
      <span className="environment-chip"><span />NOT IMPLEMENTED</span>
      <h1>{title}</h1>
      <p>{description}</p>
      <Link to="/" className="text-link">Return to overview <ChevronRight size={15} /></Link>
    </div>
  );
}
