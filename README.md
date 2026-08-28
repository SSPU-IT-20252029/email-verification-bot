# Lightweight Multi-Guild Verification Bot

A Discord bot for verifying members via email and automatically assigning roles based on flexible rules. It supports multiple servers (multi-guild), Regex matching, and CSV mapping uploads.

## Requirements

- **Go** 1.27+
- **Discord Bot** — A token from the [Developer Portal](https://discord.com/developers/applications), with the **SERVER MEMBERS INTENT** enabled. Use an invite with `Manage Roles` permission and `bot` + `applications.commands` scopes. The bot's role must be placed **above** all the roles it needs to assign.
- **Resend** — An API key from [resend.com](https://resend.com) and a verified sender domain.

## Configuration (Global)

Copy `config.example.yml` to `config.yml` and fill it out:

```yaml
discord:
  token: ${DISCORD_TOKEN}

email:
  api_key: ${RESEND_API_KEY}
  from: "Discord bot <discord-bot@yourdomain.com>"

storage:
  dsn: "./data/verifier.db"
```

## Configuration (Per-Server / Slash Commands)

Once running, the bot is fully configured directly from Discord using slash commands (accessible only to administrators):

- `/setup` - Initializes the server, sets the allowed email domain, mode (REGEX or CSV), and generates a "Verify" button in the chosen channel.
- `/regex add/remove/list` - In REGEX mode, this allows you to set rules (e.g., email `.*2025@...` -> gets a specific role).
- `/csv upload` - In CSV mode, uploads a `.csv` file with `email` and `class` (or any other identifier) columns to the database.
- `/csv map` - Maps a specific `class` from the CSV to the corresponding Discord role.

## Workflow

1. A user clicks the "Verify" button (created via `/setup`).
2. A Modal window pops up, prompting the user for their email.
3. The bot checks the configured domain, generates a code, and sends it via email.
4. The bot sends an ephemeral message with an "Enter Code" button to the user.
5. The user enters the code into a second Modal.
6. The bot finds the matching role using either the configured **Regex** or **CSV Mapping** and assigns it.

## Build and Run (Docker / Local)

Running locally:
```bash
go build ./cmd/bot
./bot -config config.yml
```
*(Tokens can also be passed as environment variables: `DISCORD_TOKEN=... RESEND_API_KEY=...`)*

The bot is designed to run in a single Docker container with the database (`.db` file) mounted in a volume (`/data`). The memory footprint is optimized to stay under 50 MB RAM.

To run via Docker Compose:
1. Create a `.env` file with `DISCORD_TOKEN` and `RESEND_API_KEY`.
2. Run `docker-compose up -d --build`.
