package main

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"

	"sspu-verifier/internal/config"
	"sspu-verifier/internal/mailer"
	"sspu-verifier/internal/store"
	"sspu-verifier/internal/verify"
)

var configPath = flag.String("config", "config.yml", "Path to configuration file")

type Bot struct {
	session *discordgo.Session
	store   *store.Store
	verify  *verify.Service
}

func main() {
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	st, err := store.Open(cfg.Storage.DSN)
	if err != nil {
		log.Fatalf("Error loading database: %v", err)
	}
	defer st.Close()

	m := mailer.New(cfg.Email)
	v := verify.New(st, m)

	dg, err := discordgo.New("Bot " + cfg.Discord.Token)
	if err != nil {
		log.Fatalf("Error creating Discord session: %v", err)
	}

	bot := &Bot{
		session: dg,
		store:   st,
		verify:  v,
	}

	dg.AddHandler(bot.onReady)
	dg.AddHandler(bot.onInteractionCreate)

	dg.Identify.Intents = discordgo.IntentsGuilds

	if err := dg.Open(); err != nil {
		log.Fatalf("Error connecting to Discord: %v", err)
	}
	defer dg.Close()

	log.Println("Bot is running. Press CTRL-C to exit.")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("Shutting down...")
}

func (b *Bot) onReady(s *discordgo.Session, r *discordgo.Ready) {
	log.Printf("Logged in as %v#%v", s.State.User.Username, s.State.User.Discriminator)

	// Register commands globally
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "setup",
			Description: "Configure verification parameters for the server",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "domain",
					Description: "Allowed email domain (e.g. sspu-opava.cz)",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "mode",
					Description: "Verification mode",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "Regex Matching", Value: "REGEX"},
						{Name: "CSV Mapping", Value: "CSV"},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionChannel,
					Name:        "channel",
					Description: "Verification channel",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "subject",
					Description: "Email subject",
					Required:    false,
				},
			},
			DefaultMemberPermissions: func(i int64) *int64 { return &i }(discordgo.PermissionAdministrator),
		},
		{
			Name:        "regex",
			Description: "Manage Regex rules",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "add",
					Description: "Add a regex rule",
					Options: []*discordgo.ApplicationCommandOption{
						{Type: discordgo.ApplicationCommandOptionString, Name: "pattern", Description: "Regex pattern", Required: true},
						{Type: discordgo.ApplicationCommandOptionRole, Name: "role", Description: "Target role", Required: true},
						{Type: discordgo.ApplicationCommandOptionInteger, Name: "priority", Description: "Priority (higher = more important)", Required: false},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "list",
					Description: "List all rules",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "remove",
					Description: "Remove a rule",
					Options: []*discordgo.ApplicationCommandOption{
						{Type: discordgo.ApplicationCommandOptionInteger, Name: "id", Description: "Rule ID", Required: true},
					},
				},
			},
			DefaultMemberPermissions: func(i int64) *int64 { return &i }(discordgo.PermissionAdministrator),
		},
		{
			Name:        "csv",
			Description: "Manage CSV data",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "upload",
					Description: "Upload a CSV file (email,class)",
					Options: []*discordgo.ApplicationCommandOption{
						{Type: discordgo.ApplicationCommandOptionAttachment, Name: "file", Description: "CSV file", Required: true},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "map",
					Description: "Map a class to a role",
					Options: []*discordgo.ApplicationCommandOption{
						{Type: discordgo.ApplicationCommandOptionString, Name: "class", Description: "Class name from CSV", Required: true},
						{Type: discordgo.ApplicationCommandOptionRole, Name: "role", Description: "Discord role", Required: true},
					},
				},
			},
			DefaultMemberPermissions: func(i int64) *int64 { return &i }(discordgo.PermissionAdministrator),
		},
	}

	_, err := s.ApplicationCommandBulkOverwrite(s.State.User.ID, "", commands)
	if err != nil {
		log.Printf("Error registering commands: %v", err)
	}
}

func (b *Bot) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		b.handleSlashCommand(s, i)
	case discordgo.InteractionMessageComponent:
		b.handleComponent(s, i)
	case discordgo.InteractionModalSubmit:
		b.handleModal(s, i)
	}
}

func (b *Bot) handleSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	name := i.ApplicationCommandData().Name
	switch name {
	case "setup":
		b.cmdSetup(s, i)
	case "regex":
		b.cmdRegex(s, i)
	case "csv":
		b.cmdCSV(s, i)
	}
}

func (b *Bot) cmdSetup(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := i.ApplicationCommandData().Options
	var domain, mode, channelID, subject string
	subject = "Verification code"
	for _, o := range opts {
		switch o.Name {
		case "domain":
			domain = o.StringValue()
		case "mode":
			mode = o.StringValue()
		case "channel":
			channelID = o.ChannelValue(nil).ID
		case "subject":
			subject = o.StringValue()
		}
	}

	cfg := store.GuildConfig{
		GuildID:          i.GuildID,
		VerifyChannelID:  channelID,
		Domain:           domain,
		Mode:             mode,
		Subject:          subject,
		CodeTTL:          10 * time.Minute,
		MaxAttempts:      5,
		RateLimitPerHour: 3,
	}

	if err := b.store.SaveGuildConfig(context.Background(), cfg); err != nil {
		respondErr(s, i, "Failed to save configuration.")
		return
	}

	// Send message with button to the channel
	_, err := s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{{
			Title:       "School Email Verification",
			Description: "To gain access, click the button and enter your school email (@" + domain + ").",
			Color:       0x3b82f6,
		}},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						CustomID: "btn_verify_start",
						Label:    "Verify",
						Style:    discordgo.PrimaryButton,
					},
				},
			},
		},
	})

	if err != nil {
		respondErr(s, i, "Configuration saved, but failed to send the message to the channel.")
		return
	}

	respondOK(s, i, "Server successfully configured.")
}

func (b *Bot) cmdRegex(s *discordgo.Session, i *discordgo.InteractionCreate) {
	subcmd := i.ApplicationCommandData().Options[0]
	switch subcmd.Name {
	case "add":
		var pattern, roleID string
		priority := 0
		for _, o := range subcmd.Options {
			switch o.Name {
			case "pattern":
				pattern = o.StringValue()
			case "role":
				roleID = o.RoleValue(nil, "").ID
			case "priority":
				priority = int(o.IntValue())
			}
		}
		err := b.store.AddRegexRule(context.Background(), store.RegexRule{
			GuildID:  i.GuildID,
			Pattern:  pattern,
			RoleID:   roleID,
			Priority: priority,
		})
		if err != nil {
			respondErr(s, i, "Failed to add rule.")
			return
		}
		respondOK(s, i, "Rule added.")

	case "list":
		rules, err := b.store.ListRegexRules(context.Background(), i.GuildID)
		if err != nil {
			respondErr(s, i, "Failed to load rules.")
			return
		}
		if len(rules) == 0 {
			respondOK(s, i, "No rules are set.")
			return
		}
		var msg strings.Builder
		for _, r := range rules {
			msg.WriteString(fmt.Sprintf("ID: %d | Pattern: `%s` | Role: <@&%s> | Priority: %d\n", r.ID, r.Pattern, r.RoleID, r.Priority))
		}
		respondOK(s, i, msg.String())

	case "remove":
		id := int(subcmd.Options[0].IntValue())
		if err := b.store.RemoveRegexRule(context.Background(), id); err != nil {
			respondErr(s, i, "Failed to delete rule.")
			return
		}
		respondOK(s, i, "Rule deleted.")
	}
}

func (b *Bot) cmdCSV(s *discordgo.Session, i *discordgo.InteractionCreate) {
	subcmd := i.ApplicationCommandData().Options[0]
	switch subcmd.Name {
	case "upload":
		attID := subcmd.Options[0].Value.(string)
		att := i.ApplicationCommandData().Resolved.Attachments[attID]

		resp, err := http.Get(att.URL)
		if err != nil || resp.StatusCode != http.StatusOK {
			respondErr(s, i, "Error downloading file.")
			return
		}
		defer resp.Body.Close()

		reader := csv.NewReader(resp.Body)
		records, err := reader.ReadAll()
		if err != nil {
			respondErr(s, i, "Invalid CSV format.")
			return
		}

		ctx := context.Background()
		_ = b.store.ClearCSVEmails(ctx, i.GuildID)

		count := 0
		for _, row := range records {
			if len(row) >= 2 {
				email, class := strings.TrimSpace(row[0]), strings.TrimSpace(row[1])
				if email != "" && class != "" {
					b.store.InsertCSVEmail(ctx, i.GuildID, email, class)
					count++
				}
			}
		}
		respondOK(s, i, fmt.Sprintf("Uploaded %d emails into the database.", count))

	case "map":
		var class, roleID string
		for _, o := range subcmd.Options {
			if o.Name == "class" {
				class = o.StringValue()
			} else if o.Name == "role" {
				roleID = o.RoleValue(nil, "").ID
			}
		}
		err := b.store.MapCSVClass(context.Background(), i.GuildID, class, roleID)
		if err != nil {
			respondErr(s, i, "Failed to save mapping.")
			return
		}
		respondOK(s, i, fmt.Sprintf("Class `%s` mapped to role <@&%s>.", class, roleID))
	}
}

func (b *Bot) handleComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.MessageComponentData().CustomID {
	case "btn_verify_start":
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				CustomID: "modal_email",
				Title:    "School Email Verification",
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.TextInput{
								CustomID:    "input_email",
								Label:       "Your email",
								Style:       discordgo.TextInputShort,
								Placeholder: "student@domain.com",
								Required:    true,
							},
						},
					},
				},
			},
		})
		if err != nil {
			log.Println("Error sending modal:", err)
		}
	case "btn_enter_code":
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				CustomID: "modal_code",
				Title:    "Enter code from email",
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.TextInput{
								CustomID:    "input_code",
								Label:       "Verification code",
								Style:       discordgo.TextInputShort,
								Placeholder: "123456",
								Required:    true,
								MinLength:   6,
								MaxLength:   6,
							},
						},
					},
				},
			},
		})
		if err != nil {
			log.Println("Error sending modal:", err)
		}
	}
}

func (b *Bot) handleModal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()

	switch data.CustomID {
	case "modal_email":
		email := data.Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
		err := b.verify.Start(context.Background(), i.GuildID, i.Member.User.ID, email)
		if err != nil {
			respondErr(s, i, "Error: "+err.Error())
			return
		}

		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Code sent to " + email + ". Check your inbox and click the button below to enter it.",
				Flags:   discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{
								CustomID: "btn_enter_code",
								Label:    "Enter Code",
								Style:    discordgo.SuccessButton,
							},
						},
					},
				},
			},
		})
		if err != nil {
			log.Println("Error responding:", err)
		}

	case "modal_code":
		code := data.Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
		roleID, err := b.verify.Confirm(context.Background(), i.GuildID, i.Member.User.ID, code)
		if err != nil {
			respondErr(s, i, "Verification failed: "+err.Error())
			return
		}

		err = s.GuildMemberRoleAdd(i.GuildID, i.Member.User.ID, roleID)
		if err != nil {
			respondErr(s, i, "Verification successful, but failed to assign the role. Contact an administrator.")
			return
		}

		respondOK(s, i, "Verification successful! The role has been assigned.")
	}
}

func respondOK(s *discordgo.Session, i *discordgo.InteractionCreate, msg string) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "✅ " + msg,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func respondErr(s *discordgo.Session, i *discordgo.InteractionCreate, msg string) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "❌ " + msg,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

