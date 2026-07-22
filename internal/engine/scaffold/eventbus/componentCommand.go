package eventbus

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"skyrix/internal/engine/scaffold/support"

	"github.com/spf13/cobra"
)

const eventBusProviderMarker = "// skyrix:eventbus-provider "

func (c *EventBusCommand) newEventCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "event",
		Short: "Manage EventBus event contracts",
		Long:  "Create transport-neutral event payloads and versioned subjects inside a module.",
	}
	var subject string
	create := &cobra.Command{
		Use:   "create <Module> <Name>",
		Short: "Create an event contract",
		Long: `Create an event payload type and subject constant in an existing module.

This command creates only the event contract. It does not create a consumer,
publisher, or subscription.`,
		Example: `  go run ./cmd/console eventbus event create Payment PaymentRequested --subject payment.requested.v1`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			module, name, moduleDir, err := c.componentTarget(args[0], args[1])
			if err != nil {
				return err
			}
			subject = strings.TrimSpace(subject)
			if subject == "" || strings.ContainsAny(subject, " \t\r\n") {
				return fmt.Errorf("subject must be non-empty and contain no whitespace")
			}
			content := fmt.Sprintf(`%s

			package %s

			// Subject%s is the versioned EventBus subject for %s.
			const Subject%s = %q

			// %s is the transport payload for Subject%s.
			type %s struct{}
			`, eventBusGeneratedHeader, module.Package, name.Type, name.Type, name.Type, subject, name.Type, name.Type, name.Type)
			path := filepath.Join(moduleDir, lowerFirst(name.Type)+"Event.go")
			if err := support.WriteNewFile(path, []byte(content)); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "event created: %s.%s (%s)\n", module.Package, name.Type, subject)
			fmt.Fprintf(cmd.OutOrStdout(), "next for incoming events: eventbus consumer create %s %s\n", module.Type, name.Type)
			fmt.Fprintf(cmd.OutOrStdout(), "next for outgoing events: eventbus publisher create %s %s\n", module.Type, name.Type)
			return nil
		},
	}
	create.Flags().StringVar(&subject, "subject", "", "Versioned subject, for example payment.requested.v1 (required)")
	_ = create.MarkFlagRequired("subject")
	command.AddCommand(create)
	return command
}

func (c *EventBusCommand) newConsumerCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "consumer",
		Short: "Manage EventBus message consumers",
		Long:  "Create message handlers that decode an event payload and execute application logic.",
	}
	command.AddCommand(&cobra.Command{
		Use:   "create <Module> <Event>",
		Short: "Create a consumer for an existing event",
		Long: `Create an EventBus message handler for an existing event contract.

The generated Handle method decodes message.Data. Add application behavior to
HandlePayload. This command does not subscribe the consumer to NATS; create a
subscriber separately for that.`,
		Example: `  go run ./cmd/console eventbus consumer create Payment PaymentRequested`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			module, name, moduleDir, err := c.componentTarget(args[0], args[1])
			if err != nil {
				return err
			}
			if err := requireComponent(moduleDir, lowerFirst(name.Type)+"Event.go", "event"); err != nil {
				return err
			}
			constructor := "New" + name.Type + "Consumer"
			content := fmt.Sprintf(`%s
			%s%s

			package %s

			import (
				"context"
				"encoding/json"
				"fmt"

				"gitlab.com/skyrix-lib/eventbus"
			)

			// %sConsumer handles Subject%s messages.
			type %sConsumer struct{}

			// %s constructs a %s consumer.
			func %s() *%sConsumer { return &%sConsumer{} }

			// Handle decodes the EventBus message before dispatching its payload.
			func (c *%sConsumer) Handle(ctx context.Context, message *eventbus.Message) error {
				if message == nil {
					return fmt.Errorf("eventbus message is nil")
				}
				var payload %s
				if err := json.Unmarshal(message.Data, &payload); err != nil {
					return fmt.Errorf("decode %s event: %%w", err)
				}
				return c.HandlePayload(ctx, payload)
			}

			// HandlePayload contains application-specific event handling.
			func (c *%sConsumer) HandlePayload(ctx context.Context, payload %s) error {
				_ = c
				_ = ctx
				_ = payload
				// TODO: call the application service.
				return nil
			}
			`, eventBusGeneratedHeader, eventBusProviderMarker, constructor, module.Package,
				name.Type, name.Type, name.Type, constructor, name.Type, constructor, name.Type, name.Type,
				name.Type, name.Type, name.Type, name.Type, name.Type)
			path := filepath.Join(moduleDir, lowerFirst(name.Type)+"Consumer.go")
			if err := c.writeProviderComponent(moduleDir, module.Package, path, []byte(content)); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "consumer created: %s.%sConsumer\n", module.Package, name.Type)
			fmt.Fprintf(cmd.OutOrStdout(), "next: eventbus subscriber create %s %s\n", module.Type, name.Type)
			return nil
		},
	})
	return command
}

func (c *EventBusCommand) newPublisherCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "publisher",
		Short: "Manage EventBus publishers",
		Long:  "Create typed publishers for existing event contracts.",
	}
	command.AddCommand(&cobra.Command{
		Use:     "create <Module> <Event>",
		Short:   "Create a publisher for an existing event",
		Long:    "Create a typed Publish method that sends an event with Bus.PublishJSON.",
		Example: `  go run ./cmd/console eventbus publisher create Payment PaymentRequested`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			module, name, moduleDir, err := c.componentTarget(args[0], args[1])
			if err != nil {
				return err
			}
			if err := requireComponent(moduleDir, lowerFirst(name.Type)+"Event.go", "event"); err != nil {
				return err
			}
			constructor := "New" + name.Type + "Publisher"
			content := fmt.Sprintf(`%s
			%s%s

			package %s

			import (
				"context"
				"fmt"

				"gitlab.com/skyrix-lib/eventbus"
			)

			// %sPublisher publishes Subject%s messages.
			type %sPublisher struct{ bus eventbus.Bus }

			// %s constructs a typed event publisher.
			func %s(bus eventbus.Bus) *%sPublisher { return &%sPublisher{bus: bus} }

			// Publish sends payload as JSON to Subject%s.
			func (p *%sPublisher) Publish(ctx context.Context, payload %s, options ...eventbus.PublishOption) error {
				if p == nil || p.bus == nil {
					return fmt.Errorf("%s publisher is not configured")
				}
				return p.bus.PublishJSON(ctx, Subject%s, payload, options...)
			}
			`, eventBusGeneratedHeader, eventBusProviderMarker, constructor, module.Package,
				name.Type, name.Type, name.Type, constructor, constructor, name.Type, name.Type,
				name.Type, name.Type, name.Type, strings.ToLower(name.Type), name.Type)
			path := filepath.Join(moduleDir, lowerFirst(name.Type)+"Publisher.go")
			if err := c.writeProviderComponent(moduleDir, module.Package, path, []byte(content)); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "publisher created: %s.%sPublisher\n", module.Package, name.Type)
			return nil
		},
	})
	return command
}

func (c *EventBusCommand) newSubscriberCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "subscriber",
		Short: "Manage EventBus subscriptions",
		Long:  "Create subscription factories that connect workers to generated consumers.",
	}
	command.AddCommand(&cobra.Command{
		Use:   "create <Module> <Event>",
		Short: "Create a subscriber for an existing consumer",
		Long: `Create a Subscribe method that binds an eventbus.Worker to the generated
consumer. The caller owns the returned subscription and must drain it during
shutdown.`,
		Example: `  go run ./cmd/console eventbus subscriber create Payment PaymentRequested`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			module, name, moduleDir, err := c.componentTarget(args[0], args[1])
			if err != nil {
				return err
			}
			if err := requireComponent(moduleDir, lowerFirst(name.Type)+"Consumer.go", "consumer"); err != nil {
				return err
			}
			constructor := "New" + name.Type + "Subscriber"
			content := fmt.Sprintf(`%s
			%s%s

			package %s

			import (
				"context"
				"fmt"

				"gitlab.com/skyrix-lib/eventbus"
			)

			// %sSubscriber subscribes %sConsumer to Subject%s.
			type %sSubscriber struct {
				bus      eventbus.Bus
				consumer *%sConsumer
			}

			// %s constructs an event subscriber.
			func %s(bus eventbus.Bus, consumer *%sConsumer) *%sSubscriber {
				return &%sSubscriber{bus: bus, consumer: consumer}
			}

			// Subscribe starts a worker and returns its lifecycle handle.
			func (s *%sSubscriber) Subscribe(ctx context.Context, worker eventbus.Worker) (eventbus.Subscription, error) {
				if s == nil || s.bus == nil || s.consumer == nil {
					return nil, fmt.Errorf("%s subscriber is not configured")
				}
				if worker.Filter == "" {
					worker.Filter = Subject%s
				}
				return s.bus.Subscribe(ctx, worker, s.consumer.Handle)
			}
			`, eventBusGeneratedHeader, eventBusProviderMarker, constructor, module.Package,
				name.Type, name.Type, name.Type, name.Type, name.Type, constructor, constructor,
				name.Type, name.Type, name.Type, name.Type, strings.ToLower(name.Type), name.Type)
			path := filepath.Join(moduleDir, lowerFirst(name.Type)+"Subscriber.go")
			if err := c.writeProviderComponent(moduleDir, module.Package, path, []byte(content)); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "subscriber created: %s.%sSubscriber\n", module.Package, name.Type)
			return nil
		},
	})
	return command
}

func (c *EventBusCommand) componentTarget(moduleRaw, nameRaw string) (support.Name, support.Name, string, error) {
	module, err := support.NormalizeName(moduleRaw)
	if err != nil {
		return support.Name{}, support.Name{}, "", fmt.Errorf("module: %w", err)
	}
	name, err := support.NormalizeName(nameRaw)
	if err != nil {
		return support.Name{}, support.Name{}, "", fmt.Errorf("component: %w", err)
	}
	root, err := support.ProjectRoot(c.ProjectRoot)
	if err != nil {
		return support.Name{}, support.Name{}, "", err
	}
	moduleDir := filepath.Join(eventBusTransportRoot(root), module.Package)
	if _, err := os.Stat(filepath.Join(moduleDir, eventBusModuleMarker)); err != nil {
		if os.IsNotExist(err) {
			return support.Name{}, support.Name{}, "", fmt.Errorf("eventbus module %q does not exist; run eventbus module create %s", module.Package, module.Type)
		}
		return support.Name{}, support.Name{}, "", fmt.Errorf("inspect eventbus module: %w", err)
	}
	return module, name, moduleDir, nil
}

func requireComponent(moduleDir, filename, kind string) error {
	if _, err := os.Stat(filepath.Join(moduleDir, filename)); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s does not exist; create it first", kind)
		}
		return fmt.Errorf("inspect %s: %w", kind, err)
	}
	return nil
}

func (c *EventBusCommand) writeProviderComponent(moduleDir, packageName, path string, content []byte) error {
	if err := support.WriteNewFile(path, content); err != nil {
		return err
	}
	if err := writeEventBusModuleProvider(moduleDir, packageName); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func writeEventBusModuleProvider(moduleDir, packageName string) error {
	entries, err := os.ReadDir(moduleDir)
	if err != nil {
		return fmt.Errorf("read eventbus module: %w", err)
	}
	providers := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || entry.Name() == "provider.go" {
			continue
		}
		content, err := os.ReadFile(filepath.Join(moduleDir, entry.Name()))
		if err != nil {
			return fmt.Errorf("read eventbus component %s: %w", entry.Name(), err)
		}
		for _, line := range strings.Split(string(content), "\n") {
			if provider, ok := strings.CutPrefix(strings.TrimSpace(line), eventBusProviderMarker); ok {
				provider = strings.TrimSpace(provider)
				if provider != "" {
					providers = append(providers, provider)
				}
			}
		}
	}
	sort.Strings(providers)
	var registrations strings.Builder
	for _, provider := range providers {
		fmt.Fprintf(&registrations, "\t%s,\n", provider)
	}
	content := fmt.Sprintf(`%s

	package %s

	import "github.com/google/wire"

	// ProviderSet contains generated EventBus transport providers.
	var ProviderSet = wire.NewSet(
	%s)
	`, eventBusGeneratedHeader, packageName, registrations.String())
	formatted, err := formatGoSource([]byte(content))
	if err != nil {
		return err
	}
	path := filepath.Join(moduleDir, "provider.go")
	existing, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read module provider: %w", err)
	}
	if !strings.HasPrefix(string(existing), eventBusGeneratedHeader) {
		return fmt.Errorf("refusing to overwrite unmanaged module provider: %s", path)
	}
	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		return fmt.Errorf("write module provider: %w", err)
	}
	return nil
}

func formatGoSource(content []byte) ([]byte, error) {
	formatted, err := format.Source(content)
	if err != nil {
		return nil, fmt.Errorf("format generated eventbus component: %w", err)
	}
	return formatted, nil
}

func lowerFirst(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return value
	}
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}
