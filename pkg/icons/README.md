# icons — NerdFont Icons with ASCII Fallback

> Package: `sysq-libs/pkg/icons`

Shared NerdFont Mono icons used across lazarusd and MCP prompts.
Provides automatic fallback to ASCII equivalents when NerdFont is
unavailable (terminal without UTF-8 support, `LAZARUS_NERDFONT=0`).

## API

| Symbol | NerdFont | ASCII |
|--------|----------|-------|
| `Check` | 󰄬 | ✓ |
| `Close` | 󰅖 | ✗ |
| `Warn` | 󰀚 | ⚠ |
| `Gear` | 󰘛 | ● |
| `Info` | 󰍿 | ? |
| `Heart` | 󰋡 | ♥ |
| `Shield` | 󰚇 | ⛊ |

| Function | Signature | Description |
|----------|-----------|-------------|
| `Icon` | `(nerd, ascii string) string` | Returns NerdFont or ASCII depending on terminal |
| `UseNerd` | `() bool` | Returns true when NerdFont is available |

## Detection

- `LAZARUS_NERDFONT=0` forces ASCII fallback
- `TERM=linux` (VGA console) always falls back to ASCII
- Terminals with `256color`, `alacritty`, `kitty`, `ghostty` + UTF-8 locale → NerdFont

## Files

| File | LOC | Purpose |
|------|-----|---------|
| `nerd.go` | 49 | Icons, `UseNerd()`, `Icon()` |

## History

Migrated from `lazarus/internal/icons/` as part of BLOK 1.3 (2026-07-02).
