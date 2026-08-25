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

verification:
  code_ttl: 10m
  max_attempts: 5
  rate_limit_per_hour: 3

storage:
  dsn: "./data/verifier.db"
```

Role (`IT1–IT4`, `Uo1–Uo4`, `Sv1A–Sv4A`, `Absolvent`) si bot spravuje sám — při prvním spuštění je najde na serveru podle názvů (chybějící vytvoří) a uloží do své databáze. Názvy rolí lze změnit v nepovinné sekci `roles` (viz `config.example.yml`) — pak bot hledá/vytváří role s těmi názvy.

Tokeny se čtou z proměnných prostředí `DISCORD_TOKEN` a `RESEND_API_KEY`.

## Build a spuštění

```
go build ./cmd/bot
DISCORD_TOKEN=... RESEND_API_KEY=... ./bot -config config.yml
```

## Jak to funguje

1. Bot pošle do ověřovacího kanále zprávu s tlačítkem **Ověřit se** (znovu ji vytvoří jen když smažou).
2. Kliknutí → zadání školního e-mailu → bot pošle 6místný kód (platí 10 minut).
3. Zadání kódu → přidělení role podle ročníku v e-mailu.

Každý e-mail může být přiřazen pouze jednomu Discord uživateli. Pokud se jiný uživatel pokusí ověřit stejným e-mailem, bude odmítnut.

### Přechod na nový školní rok

Každý den ve 3:00 (a při startu bota) zkontroluje bot, zda nezačal nový školní rok. Pokud ano, pro každý obor (IT, Uo, SvA, SvB):

1. maturanti (`##4`) dostanou místo své role **Absolvent**,
2. role se přejmenují o ročník výš (`##3→##4`, `##2→##3`, `##1→##2`),
3. uvolněná role `##4` se stane rolí prváků (`##1`) a bot ji zbaví oprávnění kanálů.

Protože se mění jen název role, členové si zachovají přístup do svých kanálů i po přechodu do vyššího ročníku.
