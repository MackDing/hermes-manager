# HermesManager — Dracula Gradient Theme Override

**Overrides MASTER.md theme system. This is the active theme for v1.0.0.**

The Dracula Gradient theme combines the classic [Dracula](https://draculatheme.com) color palette with gradient accents and subtle neon glow effects. Dark-first, production-readable, but with enough visual punch to make a platform engineer pause and say "that's a good-looking dashboard."

## Design Philosophy

- **Dark-first, but not dark-only.** Light mode is desaturated/muted, dark mode is the hero.
- **Gradients are functional, not decorative.** Gradient = interactive element (button, active nav, chart accent). Static elements use solid Dracula colors.
- **Glow is SUBTLE.** `0 0 8px` max on focused elements. No neon text, no scanlines, no CRT effects. This is an ops dashboard, not a game UI.
- **Dracula palette is the base.** Purple/pink/cyan/green directly from draculatheme.com, not approximations.

## Color Tokens (replaces MASTER.md §2)

```css
:root {
  /* Light mode — desaturated Dracula, muted gradients */
  --color-bg:          #F8F8F2;
  --color-bg-elevated: #FFFFFF;
  --color-bg-subtle:   #F0F0EC;
  --color-border:      #D6D6CC;
  --color-border-strong: #BFBFB6;

  --color-fg:          #282A36;
  --color-fg-muted:    #6272A4; /* Dracula Comment */
  --color-fg-subtle:   #8B95B5;

  --color-primary:     #282A36; /* Dracula Background as text */
  --color-primary-fg:  #F8F8F2;
  --color-accent:      #BD93F9; /* Dracula Purple */

  /* Semantic status — Dracula palette */
  --color-success:     #50FA7B; /* Dracula Green */
  --color-success-bg:  #E8FFF0;
  --color-warning:     #FFB86C; /* Dracula Orange */
  --color-warning-bg:  #FFF5E8;
  --color-error:       #FF5555; /* Dracula Red */
  --color-error-bg:    #FFE8E8;
  --color-info:        #8BE9FD; /* Dracula Cyan */
  --color-info-bg:     #E8FAFF;

  /* Runtime badges */
  --color-runtime-local:  #BD93F9; /* Purple */
  --color-runtime-docker: #8BE9FD; /* Cyan */
  --color-runtime-k8s:    #FF79C6; /* Pink */

  /* Gradients (functional — buttons, active states, chart accents) */
  --gradient-primary:  linear-gradient(135deg, #BD93F9, #FF79C6);  /* purple → pink */
  --gradient-success:  linear-gradient(135deg, #50FA7B, #8BE9FD);  /* green → cyan */
  --gradient-accent:   linear-gradient(135deg, #FF79C6, #FFB86C);  /* pink → orange */
  --gradient-info:     linear-gradient(135deg, #8BE9FD, #BD93F9);  /* cyan → purple */

  /* Glow (light mode: no glow) */
  --glow-primary:    none;
  --glow-accent:     none;
}

[data-theme="dark"] {
  /* Dark mode — TRUE Dracula with gradient accents */
  --color-bg:          #282A36; /* Dracula Background */
  --color-bg-elevated: #44475A; /* Dracula Current Line */
  --color-bg-subtle:   #21222C; /* Slightly deeper */
  --color-border:      #44475A; /* Dracula Current Line */
  --color-border-strong: #6272A4; /* Dracula Comment */

  --color-fg:          #F8F8F2; /* Dracula Foreground */
  --color-fg-muted:    #6272A4; /* Dracula Comment */
  --color-fg-subtle:   #4B5574;

  --color-primary:     #F8F8F2;
  --color-primary-fg:  #282A36;
  --color-accent:      #BD93F9; /* Dracula Purple */

  --color-success:     #50FA7B;
  --color-success-bg:  rgba(80, 250, 123, 0.12);
  --color-warning:     #FFB86C;
  --color-warning-bg:  rgba(255, 184, 108, 0.12);
  --color-error:       #FF5555;
  --color-error-bg:    rgba(255, 85, 85, 0.12);
  --color-info:        #8BE9FD;
  --color-info-bg:     rgba(139, 233, 253, 0.12);

  --color-runtime-local:  #BD93F9;
  --color-runtime-docker: #8BE9FD;
  --color-runtime-k8s:    #FF79C6;

  /* Gradients (hero treatment in dark mode) */
  --gradient-primary:  linear-gradient(135deg, #BD93F9, #FF79C6);
  --gradient-success:  linear-gradient(135deg, #50FA7B, #8BE9FD);
  --gradient-accent:   linear-gradient(135deg, #FF79C6, #FFB86C);
  --gradient-info:     linear-gradient(135deg, #8BE9FD, #BD93F9);

  /* Glow (dark mode: subtle neon) */
  --glow-primary:    0 0 8px rgba(189, 147, 249, 0.4);  /* purple glow */
  --glow-accent:     0 0 8px rgba(255, 121, 198, 0.3);  /* pink glow */
  --glow-success:    0 0 8px rgba(80, 250, 123, 0.3);   /* green glow */
  --glow-info:       0 0 8px rgba(139, 233, 253, 0.3);  /* cyan glow */
}
```

## Typography Override

- **UI headings:** `Orbitron` (weights 700/900) — sci-fi/tactical feel on page titles + stat card numbers
- **UI body:** `Inter` (unchanged from MASTER)
- **Data / code:** `JetBrains Mono` (unchanged)

Orbitron is used SPARINGLY: page titles (`text-xl`), stat card values (`text-2xl`), and the logo text only. Everything else stays Inter. Orbitron on body text would kill readability.

```css
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&family=Orbitron:wght@700;900&display=swap');

:root {
  --font-display: 'Orbitron', system-ui, sans-serif; /* page titles + stat numbers */
  --font-sans: 'Inter', system-ui, sans-serif;       /* body + UI chrome */
  --font-mono: 'JetBrains Mono', monospace;           /* data + code */
}
```

## Component Overrides

### Stat cards (Dashboard)
```css
.stat-card {
  background: var(--color-bg-elevated);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  position: relative;
  overflow: hidden;
}
.stat-card::before {
  content: '';
  position: absolute;
  top: 0; left: 0; right: 0;
  height: 3px;
  background: var(--gradient-primary); /* purple→pink gradient top accent */
}
.stat-card .value {
  font-family: var(--font-display);
  font-size: var(--text-2xl);
  font-weight: 900;
}
```

### Primary button (gradient)
```css
.btn-primary {
  background: var(--gradient-primary);
  color: #FFFFFF;
  border: none;
  border-radius: 6px;
  font-weight: 600;
  box-shadow: var(--glow-primary);
  transition: opacity 150ms, box-shadow 150ms;
}
.btn-primary:hover {
  opacity: 0.9;
  box-shadow: 0 0 12px rgba(189, 147, 249, 0.5);
}
```

### Sidebar nav — active item
```css
.nav-active {
  background: var(--gradient-primary);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  font-weight: 600;
}
```

### Table row hover (subtle glow)
```css
[data-theme="dark"] tr:hover {
  background: rgba(189, 147, 249, 0.06);
  box-shadow: inset 3px 0 0 var(--color-accent);
}
```

### Charts
- Line chart stroke: `var(--gradient-primary)` via SVG linearGradient
- Donut slices: use runtime badge colors (Purple, Cyan, Pink)
- Sparklines: gradient stroke from `#BD93F9` to `#FF79C6`
- Tooltip background: `var(--color-bg-elevated)` with `var(--glow-primary)` shadow

## What stays the same
- Spacing, density, icon library (Lucide), layout structure — all per MASTER.md
- Accessibility rules — focus rings, contrast ratios, reduced-motion
- Table structure, keyboard shortcuts, empty states
- No scanlines, no CRT effects, no glitch animations, no parallax
