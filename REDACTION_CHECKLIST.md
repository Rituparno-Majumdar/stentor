# Redaction checklist

What must never appear in this repo, in any file or any git commit -- check both the working tree and `git log -p` before every push:

- A real OAuth client ID, client secret, access token, refresh token, app password, API key, or bot token for any platform.
- A real webhook URL (Discord, or any platform that uses one).
- A real Reddit/LinkedIn/Meta/Google/TikTok app's redirect URI if it reveals a real hostname or IP.
- A real Telegram chat ID, Mastodon instance handle, or account username tied to a real person.
- Any `~/.stentor/credentials.json` content, ever, from any machine.
- A real hostname, IP address, or VPS provider name presented as "this is what we run."

Every example value in this repo's README, tests, and comments is an obvious placeholder (`your-token-here`, `example.social`, `https://example.com`) -- never a value copied from a real account "to make the example realistic."
