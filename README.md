# Lightweight Multi-Guild Verification Bot

Discord bot pro ověřování členů pomocí e-mailu a automatické přidělování rolí podle flexibilních pravidel. Podporuje více serverů (multi-guild), Regex matching i nahrávání mapovacích CSV souborů.

## Požadavky

- **Go** 1.27+
- **Discord bot** — token z [Developer Portalu](https://discord.com/developers/applications), povolený **SERVER MEMBERS INTENT**, pozvánka s oprávněním `Manage Roles` a scope `bot` + `applications.commands`; botova role musí být **nad** všemi přidělovanými rolemi.
- **Resend** — API klíč z [resend.com](https://resend.com) a ověřená odesílací doména.

## Konfigurace (Globální)

Zkopíruj `config.example.yml` na `config.yml` a vyplň:

```yaml
discord:
  token: ${DISCORD_TOKEN}

email:
  api_key: ${RESEND_API_KEY}
  from: "Discord bot <discord-bot@tvojedomena.cz>"

storage:
  dsn: "./data/verifier.db"
```

## Konfigurace (Na serveru / Slash Commands)

Bot se po spuštění plně nastavuje přímo z Discordu pomocí příkazů (přístupné jen adminům):

- `/setup` - Inicializuje server, nastaví povolenou doménu, režim (REGEX nebo CSV) a vygeneruje tlačítko "Ověřit se" do zvoleného kanálu.
- `/regex add/remove/list` - V režimu REGEX umožňuje nastavit pravidla (např. e-mail `.*2025@...` -> dostane konkrétní roli).
- `/csv upload` - V režimu CSV nahraje do databáze `.csv` soubor se sloupci `email` a `trida` (nebo jiný název).
- `/csv map` - Přiřadí konkrétní `trida` z CSV na příslušnou Discord roli.

## Jak funguje workflow

1. Uživatel klikne na tlačítko "Ověřit se" (vytvořené přes `/setup`).
2. Otevře se vyskakovací Modal window, kam zadá svůj e-mail.
3. Bot zkontroluje nastavenou doménu, vygeneruje kód a pošle ho e-mailem.
4. Bot pošle uživateli dočasnou (ephemeral) zprávu s tlačítkem "Zadat kód".
5. Uživatel zadá kód do dalšího Modalu.
6. Bot najde roli buď přes nastavený **Regex**, nebo **CSV Mapování**, a roli přidělí.

## Build a spuštění (Docker / Lokálně)

Lokální spuštění:
```
go build ./cmd/bot
./bot -config config.yml
```
*(Tokeny lze předat i jako proměnné prostředí: `DISCORD_TOKEN=... RESEND_API_KEY=...`)*

Bot je navržen tak, aby běžel v 1 Docker kontejneru a databáze (`.db` soubor) by měla být namountovaná ve volume (`/data`). Paměťový otisk je optimalizován do 50 MB RAM.
