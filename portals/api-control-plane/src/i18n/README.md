# i18n — api-control-plane

**react-intl** (ICU MessageFormat) + **@formatjs/cli** (extract/compile) + **babel-plugin-formatjs**
(`ast: true`, wired in `vite.config.ts`). `<I18nProvider>` is the outermost provider in `App.tsx`.

## Locale resolution

`?lang=` → stored preference → `navigator.languages` → runtime config → `en`.
First source naming a locale we ship wins (`en-GB` → `en`).

- `?lang=de-DE` is for QA/translator review — no login, outranks everything, survives reloads.
- `setLocale()` (from `useLocale()`) is the only way to change locale. Never write `localStorage` directly.
- `<html lang/dir>` is applied before paint, independent of catalog loading.

## Writing a translatable component

```tsx
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

// Module scope — extraction is static. A computed id ships untranslated.
const messages = defineMessages({
  title: {
    id: 'apiControlPlane.features.projects.NewProjectDialog.title',
    defaultMessage: 'Create project',
  },
  nameLabel: {
    id: 'apiControlPlane.features.projects.NewProjectDialog.nameLabel',
    defaultMessage: 'Name',
    description: 'Label for the project name field. Noun, not a command.',
  },
});

export function NewProjectDialog() {
  const intl = useIntl();
  return (
    <Dialog>
      {/* JSX children → <FormattedMessage> */}
      <DialogTitle><FormattedMessage {...messages.title} /></DialogTitle>
      {/* String-only props → intl.formatMessage */}
      <TextField label={intl.formatMessage(messages.nameLabel)} />
    </Dialog>
  );
}
```

`intl.formatMessage` also covers `aria-label`, `title`, `alt`, `placeholder`, column headers,
toasts, validation messages, `document.title`.

Import message APIs **from `react-intl` directly**, not through `../i18n` — the extractor and
`eslint-plugin-formatjs` only recognise them there.

## Rules

1. **IDs follow `apiControlPlane.<area>.<Module>.<slug>`** — `<area>` mirrors the `src/` path with
   `/` → `.` (file name dropped), `<Module>` is PascalCase, `<slug>` is lowerCamelCase.
   Renaming an ID discards its existing translations; `defaultMessage` can change freely.
2. **Add a `description`** for anything a translator could misread — short verbs, abbreviations,
   context-dependent copy. They see the string and description, never the UI.
3. **Never concatenate fragments — one sentence is one message.** Use ICU: placeholders
   (`'Deployed {count} of {total} APIs'`), plurals (`'{count, plural, one {# API} other {# APIs}}'`),
   rich text (`'Read the <link>docs</link>'` + `values={{ link: (c) => <Link>{c}</Link> }}`).
4. **Never translate user or backend data.** API names, handles, IDs, error strings pass through as
   *values*. Map error codes to descriptors instead of rendering `error.message`.
5. **Dates/times/numbers go through `<FormattedDate>`, `<FormattedNumber>`, or `useFormatters()`** —
   never `toLocaleString()` or a module-scope `Intl.*` formatter (it freezes its locale at import).
   ```tsx
   const { shortDate, dateTime, relativeTime } = useFormatters();
   relativeTime(api.updatedAt)      // "3 hours ago"
   dateTime(deployment.createdAt)   // "Aug 17, 2026, 5:45 PM"
   shortDate(key.expiresAt) || '-'  // "" when absent
   ```
   Add new named formats to `formats.ts` rather than passing ad-hoc option bags.
6. **After changing strings, run `npm run i18n` and commit the catalogs.** `npm run i18n:check`
   fails CI on drift.

## Commands

| Command | What it does |
| --- | --- |
| `npm run i18n` | extract + compile + pseudo. **The one you run.** |
| `npm run i18n:extract` | source → `src/i18n/messages/en.json` |
| `npm run i18n:compile` | `src/i18n/messages/*.json` → `src/i18n/compiled/*.json` (`--ast`) |
| `npm run i18n:pseudo` | `en.json` → `src/i18n/compiled/en-XA.json` |
| `npm run i18n:check` | regenerates and fails if committed catalogs drifted |

Because of `--ast`, a catalog is `Record<string, MessageFormatElement[]>` (`MessageCatalog` in
`loadCatalog.ts`), not `Record<string, string>`.

## Pseudo-locale (`?lang=en-XA`)

Append `?lang=en-XA` to any URL, every translated string renders accented, bracketed, ~30% longer:

```
Description                        →  [Ḓḗḗşƈřīƥŧīǿǿƞ]
Deployed {count} of {total} APIs   →  [Ḓḗḗƥŀǿǿẏḗḗḓ {count} ǿǿƒ {total} ȦƤĪş]
```

It shows **which strings are still hardcoded** (anything plain ASCII) and **which layouts break
when text grows**. ICU placeholders and tags survive intact. Generated from `en.json` at compile
time — there is no `messages/en-XA.json`. It's in `SUPPORTED_LOCALES` but not
`USER_SELECTABLE_LOCALES`, so no switcher offers it.

## Time zone

Timestamps render in the **viewer's own time zone** (`DISPLAY_TIME_ZONE = undefined`), so `dateTime`
always shows a date alongside the time. Switching to fixed UTC is a one-line change in `formats.ts`.

## Tests

`renderWithProviders` supplies an `IntlProvider` with `locale="en"` and `messages={{}}`, so every
message falls back to its `defaultMessage`, assert on the English copy, no catalog build needed:

```tsx
expect(await screen.findByText('Create project')).toBeInTheDocument();
```

Don't swap in `I18nProvider`: it loads catalogs asynchronously and renders `null` meanwhile. Tests
needing real translations wrap their own `IntlProvider` with an explicit `messages` map.

## Adding a locale

1. Add the tag to `SUPPORTED_LOCALES` in `config.ts` (and `RTL_LOCALES` if RTL).
2. Drop `src/i18n/messages/<tag>.json` in place (same keys as `en.json`).
3. `npm run i18n`: `compile-folder` picks up every file automatically.
4. Verify with `?lang=<tag>`, commit both catalogs.

RTL locales also need `theme.direction` and a stylis-RTL emotion cache; `<html dir>` alone flips
text but not component styles.
