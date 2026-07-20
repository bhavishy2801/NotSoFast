# Optional cloud accounts

The app remains usable without this service. Cloud sync stores explicit, portable profile snapshots.

## Configure your project

1. Create a Supabase project and run `schema.sql` once in its SQL editor.
2. Enable GitHub and/or Google in Authentication providers. Configure that provider's OAuth app with the callback shown by Supabase, typically `https://YOUR-PROJECT.supabase.co/auth/v1/callback`. OAuth client secrets belong only in the Supabase dashboard.
3. Add `http://127.0.0.1:*/auth/callback**` to the allowed redirect URLs. The desktop uses a random loopback port and additionally validates a one-use flow identifier and PKCE verifier.
4. Open **Account → Configure cloud connection** and enter the project URL and public publishable/legacy anon key. Never enter a service-role key or OAuth secret.
5. Sign in with your enabled provider. Choose **Save to cloud** to upload a portable snapshot. On another computer, configure the same project, sign in, find saved work, preview it and explicitly restore it.

No live project configuration has been supplied. The integration is implemented and tested against a scripted service; hosted OAuth and deployed database policies still need validation against your project.

## Data boundaries

Uploaded only when you choose Save: display name, theme, reduced-motion preference, up to fifty GitHub URLs, and one draft path/content (64 KiB content limit). Private repository URLs and draft contents are included. Supabase administrators can access stored data; no end-to-end encryption is claimed.

Not uploaded: local repository paths, Git repositories, receipts, operation history, model API keys, GitHub CLI credentials or cloud session tokens. Insights remain local. Cloud accounts do not isolate users sharing one Windows account or application data directory.

Cloud tokens remain in server-process memory. Restart requires sign-in. Sessions refresh while running; offline sign-out clears local credentials but cannot immediately revoke the provider session. Saves are immutable snapshots, with preview-before-restore and ten-save retention, rather than automatic background merging.

## Verify before distributing a configured app

- Save as account A and restore on another computer.
- Verify account B cannot list, read, insert or delete A's rows through the REST API.
- Reject forged, expired and replayed callbacks and mismatched PKCE verifiers.
- Check retention after more than ten saves, including simultaneous saves.
- Confirm offline local use, visible cloud failures, session refresh and logout.

Automated contract checks cover callback binding, replay rejection, credential exclusion, profile persistence and ownership mismatch rejection. They do not constitute deployed RLS or live OAuth verification.

References: [Supabase PKCE](https://supabase.com/docs/guides/auth/sessions/pkce-flow), [redirect URLs](https://supabase.com/docs/guides/auth/redirect-urls), [row-level security](https://supabase.com/docs/guides/database/postgres/row-level-security).
