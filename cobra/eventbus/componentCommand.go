package eventbus

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	support "gitlab.com/skyrix-lib/framework/scaffold"

	"github.com/spf13/cobra"
)

const eventBusProviderMarker = "// skyrix:eventbus-provider "

func (c *EventBusCommand) newEventCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "event",
		Short: "Manage EventBus event contracts",
		Long:  "Create transport event payloads and versioned subjects in the events package.",
	}
	var subject string
	create := &cobra.Command{
		Use:   "create <Name>",
		Short: "Create an event contract",
		Long: `Create only an event subject and payload in internal/transport/eventbus/events.

For an incoming event, next create its consumer and subscriber. For an outgoing
event, next create its publisher.`,
		Example: `  ./cobra eventbus event create CatalogPaymentHoldRequested --subject catalog.payment_hold.requested.v1`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, name, initialized, err := c.componentRoot(args[0])
			if err != nil {
				return err
			}
			subject = strings.TrimSpace(subject)
			if subject == "" || strings.ContainsAny(subject, " \t\r\n") {
				return fmt.Errorf("subject must be non-empty and contain no whitespace")
			}
			content := fmt.Sprintf(`%s

package events

// Subject%s is the versioned EventBus subject for %s.
const Subject%s = %q

// %s is the transport payload for Subject%s.
type %s struct{}
`, eventBusGeneratedHeader, name.Type, name.Type, name.Type, subject, name.Type, name.Type, name.Type)
			path := componentPath(root, "events", name)
			if err := support.WriteNewFile(path, []byte(content)); err != nil {
				return err
			}
			printInitialization(cmd, root, initialized)
			fmt.Fprintf(cmd.OutOrStdout(), "event created: events.%s (%s)\n", name.Type, subject)
			fmt.Fprintf(cmd.OutOrStdout(), "incoming next: eventbus consumer create %s; eventbus subscriber create %s\n", name.Type, name.Type)
			fmt.Fprintf(cmd.OutOrStdout(), "outgoing next: eventbus publisher create %s\n", name.Type)
			return nil
		},
	}
	create.Flags().StringVar(&subject, "subject", "", "Versioned subject, for example payment.requested.v1 (required)")
	_ = create.MarkFlagRequired("subject")
	command.AddCommand(create)
	return command
}

func (c *EventBusCommand) newConsumerCommand() *cobra.Command {
	command := &cobra.Command{Use: "consumer", Short: "Manage incoming EventBus consumers"}
	command.AddCommand(&cobra.Command{
		Use:   "create <Event>",
		Short: "Create a consumer for an existing event",
		Long: `Create a consumer in internal/transport/eventbus/consumers. It decodes
the typed event envelope and delegates to an injected application handler.
This command does not subscribe the consumer; create a subscriber separately.`,
		Example: `  ./cobra eventbus consumer create CatalogPaymentHoldRequested`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, name, initialized, err := c.componentRoot(args[0])
			if err != nil {
				return err
			}
			if err := requireEvent(root, name); err != nil {
				return err
			}
			module, err := c.modulePath(root)
			if err != nil {
				return err
			}
			constructor := "New" + name.Type + "Consumer"
			content := fmt.Sprintf(`%s
%s%s

package consumers

import (
	"context"
	"encoding/json"
	"fmt"

	"%s/internal/transport/eventbus/events"

	"gitlab.com/skyrix-lib/eventbus"
)

type %sHandler func(context.Context, events.Envelope[events.%s]) error

type %sConsumer struct { handler %sHandler }

func %s(handler %sHandler) *%sConsumer {
	return &%sConsumer{handler: handler}
}

func (c *%sConsumer) Handle(ctx context.Context, message *eventbus.Message) error {
	if c == nil || c.handler == nil {
		return fmt.Errorf("%s consumer is not configured")
	}
	if message == nil {
		return fmt.Errorf("eventbus message is nil")
	}
	var envelope events.Envelope[events.%s]
	if err := json.Unmarshal(message.Data, &envelope); err != nil {
		return fmt.Errorf("decode %s event: %%w", err)
	}
	return c.handler(ctx, envelope)
}
	`, eventBusGeneratedHeader, eventBusProviderMarker, constructor, module,
				name.Type, name.Type, name.Type, name.Type, constructor, name.Type, name.Type,
				name.Type, name.Type, strings.ToLower(name.Type), name.Type, strings.ToLower(name.Type))
			if err := c.writeRoleComponent(root, "consumers", name, constructor, []byte(content)); err != nil {
				return err
			}
			printInitialization(cmd, root, initialized)
			fmt.Fprintf(cmd.OutOrStdout(), "consumer created: consumers.%sConsumer\n", name.Type)
			fmt.Fprintf(cmd.OutOrStdout(), "next: eventbus subscriber create %s\n", name.Type)
			return nil
		},
	})
	return command
}

func (c *EventBusCommand) newPublisherCommand() *cobra.Command {
	command := &cobra.Command{Use: "publisher", Short: "Manage outgoing EventBus publishers"}
	command.AddCommand(&cobra.Command{
		Use:     "create <Event>",
		Short:   "Create a publisher for an existing event",
		Long:    "Create a typed publisher in internal/transport/eventbus/publishers using Bus.PublishJSON.",
		Example: `  ./cobra eventbus publisher create PaymentCatalogHoldSucceeded`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, name, initialized, err := c.componentRoot(args[0])
			if err != nil {
				return err
			}
			if err := requireEvent(root, name); err != nil {
				return err
			}
			module, err := c.modulePath(root)
			if err != nil {
				return err
			}
			constructor := "New" + name.Type + "Publisher"
			content := fmt.Sprintf(`%s
%s%s

package publishers

import (
	"context"
	"fmt"

	"%s/internal/transport/eventbus/events"

	"gitlab.com/skyrix-lib/eventbus"
)

type %sPublisher struct { bus eventbus.Bus }

func %s(bus eventbus.Bus) *%sPublisher { return &%sPublisher{bus: bus} }

func (p *%sPublisher) Publish(ctx context.Context, envelope events.Envelope[events.%s], options ...eventbus.PublishOption) error {
	if p == nil || p.bus == nil {
		return fmt.Errorf("%s publisher is not configured")
	}
	return p.bus.PublishJSON(ctx, events.Subject%s, envelope, options...)
}
`, eventBusGeneratedHeader, eventBusProviderMarker, constructor, module,
				name.Type, constructor, name.Type, name.Type, name.Type, name.Type, strings.ToLower(name.Type), name.Type)
			if err := c.writeRoleComponent(root, "publishers", name, constructor, []byte(content)); err != nil {
				return err
			}
			printInitialization(cmd, root, initialized)
			fmt.Fprintf(cmd.OutOrStdout(), "publisher created: publishers.%sPublisher\n", name.Type)
			return nil
		},
	})
	return command
}

func (c *EventBusCommand) newSubscriberCommand() *cobra.Command {
	command := &cobra.Command{Use: "subscriber", Short: "Manage EventBus subscriptions"}
	command.AddCommand(&cobra.Command{
		Use:   "create <Event>",
		Short: "Create a subscriber for an existing consumer",
		Long: `Create a subscriber in internal/transport/eventbus/subscribers that
binds an eventbus.Worker to its consumer. The returned subscription must be
drained during application shutdown.`,
		Example: `  ./cobra eventbus subscriber create CatalogPaymentHoldRequested`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, name, initialized, err := c.componentRoot(args[0])
			if err != nil {
				return err
			}
			if _, err := os.Stat(componentPath(root, "consumers", name)); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("consumer does not exist; run eventbus consumer create %s", name.Type)
				}
				return fmt.Errorf("inspect consumer: %w", err)
			}
			module, err := c.modulePath(root)
			if err != nil {
				return err
			}
			constructor := "New" + name.Type + "Subscriber"
			content := fmt.Sprintf(`%s
%s%s

package subscribers

import (
	"context"
	"fmt"

	"%s/internal/transport/eventbus/consumers"
	"%s/internal/transport/eventbus/events"

	"gitlab.com/skyrix-lib/eventbus"
)

type %sSubscriber struct {
	bus eventbus.Bus
	consumer *consumers.%sConsumer
}

func %s(bus eventbus.Bus, consumer *consumers.%sConsumer) *%sSubscriber {
	return &%sSubscriber{bus: bus, consumer: consumer}
}

func (s *%sSubscriber) Subscribe(ctx context.Context, worker eventbus.Worker) (eventbus.Subscription, error) {
	if s == nil || s.bus == nil || s.consumer == nil {
		return nil, fmt.Errorf("%s subscriber is not configured")
	}
	if worker.Filter == "" {
		worker.Filter = events.Subject%s
	}
	return s.bus.Subscribe(ctx, worker, s.consumer.Handle)
}
`, eventBusGeneratedHeader, eventBusProviderMarker, constructor, module, module,
				name.Type, name.Type, constructor, name.Type, name.Type, name.Type,
				name.Type, strings.ToLower(name.Type), name.Type)
			if err := c.writeRoleComponent(root, "subscribers", name, constructor, []byte(content)); err != nil {
				return err
			}
			printInitialization(cmd, root, initialized)
			fmt.Fprintf(cmd.OutOrStdout(), "subscriber created: subscribers.%sSubscriber\n", name.Type)
			return nil
		},
	})
	return command
}

func (c *EventBusCommand) newListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List generated EventBus events and components",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := support.DirectoryRoot(c.ProjectRoot)
			if err != nil {
				return err
			}
			eventsDir := filepath.Join(eventBusTransportRoot(root), "events")
			entries, err := os.ReadDir(eventsDir)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("eventbus is not initialized; run eventbus init")
				}
				return err
			}
			var names []string
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || entry.Name() == "envelope.go" || strings.HasSuffix(entry.Name(), "_test.go") {
					continue
				}
				names = append(names, strings.TrimSuffix(entry.Name(), ".go"))
			}
			sort.Strings(names)
			if len(names) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no eventbus events")
				return nil
			}
			for _, name := range names {
				parts := []string{"event"}
				for _, role := range []string{"consumers", "subscribers", "publishers"} {
					if _, err := os.Stat(filepath.Join(eventBusTransportRoot(root), role, name+".go")); err == nil {
						parts = append(parts, strings.TrimSuffix(role, "s"))
					}
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", name, strings.Join(parts, ", "))
			}
			return nil
		},
	}
}

func (c *EventBusCommand) newRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <Event>",
		Short: "Remove a generated event and its generated components",
		Long:  "Remove only files managed by the EventBus scaffold and refresh role providers.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := support.DirectoryRoot(c.ProjectRoot)
			if err != nil {
				return err
			}
			name, err := support.NormalizeName(args[0])
			if err != nil {
				return err
			}
			removed := 0
			for _, role := range eventBusPackages {
				path := componentPath(root, role, name)
				content, err := os.ReadFile(path)
				if os.IsNotExist(err) {
					continue
				}
				if err != nil {
					return err
				}
				if !strings.HasPrefix(string(content), eventBusGeneratedHeader) {
					return fmt.Errorf("refusing to remove unmanaged eventbus file: %s", path)
				}
				if err := os.Remove(path); err != nil {
					return err
				}
				removed++
			}
			if removed == 0 {
				return fmt.Errorf("eventbus event does not exist: %s", name.Type)
			}
			for _, role := range []string{"consumers", "publishers", "subscribers"} {
				if err := writeRoleProvider(root, role); err != nil {
					return err
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "eventbus event removed: %s (%d files)\n", name.Type, removed)
			return nil
		},
	}
}

func (c *EventBusCommand) componentRoot(raw string) (string, support.Name, bool, error) {
	name, err := support.NormalizeName(raw)
	if err != nil {
		return "", support.Name{}, false, fmt.Errorf("event: %w", err)
	}
	root, err := support.DirectoryRoot(c.ProjectRoot)
	if err != nil {
		return "", support.Name{}, false, err
	}
	initialized, err := c.ensureInitialized(root)
	return root, name, initialized, err
}

func componentPath(root, role string, name support.Name) string {
	return filepath.Join(eventBusTransportRoot(root), role, lowerFirst(name.Type)+".go")
}

func requireEvent(root string, name support.Name) error {
	if _, err := os.Stat(componentPath(root, "events", name)); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("event does not exist; run eventbus event create %s --subject <subject>", name.Type)
		}
		return fmt.Errorf("inspect event: %w", err)
	}
	return nil
}

func (c *EventBusCommand) writeRoleComponent(root, role string, name support.Name, _ string, content []byte) error {
	path := componentPath(root, role, name)
	if err := support.WriteNewFile(path, content); err != nil {
		return err
	}
	if err := writeRoleProvider(root, role); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func writeRoleProvider(root, role string) error {
	dir := filepath.Join(eventBusTransportRoot(root), role)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read eventbus %s package: %w", role, err)
	}
	var providers []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || entry.Name() == "provider.go" {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(content), "\n") {
			if provider, ok := strings.CutPrefix(strings.TrimSpace(line), eventBusProviderMarker); ok && strings.TrimSpace(provider) != "" {
				providers = append(providers, strings.TrimSpace(provider))
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

// ProviderSet contains generated %s providers.
var ProviderSet = wire.NewSet(
%s)
`, eventBusGeneratedHeader, role, role, registrations.String())
	formatted, err := format.Source([]byte(content))
	if err != nil {
		return fmt.Errorf("format %s provider: %w", role, err)
	}
	path := filepath.Join(dir, "provider.go")
	if existing, err := os.ReadFile(path); err == nil && !strings.HasPrefix(string(existing), eventBusGeneratedHeader) {
		return fmt.Errorf("refusing to overwrite unmanaged provider: %s", path)
	}
	return os.WriteFile(path, formatted, 0o644)
}

func printInitialization(cmd *cobra.Command, root string, initialized bool) {
	if initialized {
		fmt.Fprintf(cmd.OutOrStdout(), "eventbus initialized: %s\n", eventBusTransportRoot(root))
	}
}

func lowerFirst(value string) string {
	runes := []rune(value)
	if len(runes) > 0 {
		runes[0] = unicode.ToLower(runes[0])
	}
	return string(runes)
}
