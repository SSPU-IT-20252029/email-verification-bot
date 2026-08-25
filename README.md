# sspu-verifier

Discord bot pro ověřování členů pomocí školního e-mailu `@sspu-opava.cz` a automatické přidělování rolí podle třídy.

## Požadavky

- **Go** 1.27+
- **Discord bot** — token z [Developer Portalu](https://discord.com/developers/applications), povolený **SERVER MEMBERS INTENT**, pozvánka s oprávněním `Manage Roles` a scope `bot` + `applications.commands`; botova role musí být **nad** všemi třídními rolemi
- **Resend** — API klíč z [resend.com](https://resend.com) a ověřená odesílací doména (`Domains → Add Domain` + DNS záznamy); bez ní Resend poštu studentům nedoručí
- Ověřovací kanál na Discordu a jeho ID (pravý klik na kanál → Kopírovat ID)

## Konfigurace

Zkopíruj `config.example.yml` na `config.yml` a vyplň:

```yaml
discord:
  token: ${DISCORD_TOKEN}
  guild_id: "123456789012345678"
  verify_channel_id: "1540821678219853864"

email:
  api_key: ${RESEND_API_KEY}
  from: "SŠPU Discord bot <discord-bot@sspu-opava.cz>"
  subject: "Discord ověření — ověřovací kód"

roles:
  ids:
    IT1: ""
    IT2: ""
    # ... všechny role jsou povinné

verification:
  code_ttl: 10m
  max_attempts: 5
  rate_limit_per_hour: 3

storage:
  dsn: "./data/verifier.db"
```

Tokeny se čtou z proměnných prostředí `DISCORD_TOKEN` a `RESEND_API_KEY`.

## Build a spuštění

```
go build ./cmd/bot
DISCORD_TOKEN=... RESEND_API_KEY=... ./bot -config config.yml
```

## Jak to funguje

1. Bot pošle do ověřovacího kanále zprávu s tlačítkem **Ověřit se** (znovu ji vytvoří jen když smažou).
2. Kliknutí → zadání školního e-mailu → bot pošle 6místný kód (platí 10 minut).
3. Zadání kódu → přidělení role podle ročníku v e-mailu; každý den ve 3:00 se role automaticky přepočítají.
