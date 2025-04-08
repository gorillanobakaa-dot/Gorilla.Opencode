package cmd

import (
	"context"
	"os"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kujtimiihoxha/termai/internal/app"
	"github.com/kujtimiihoxha/termai/internal/config"
	"github.com/kujtimiihoxha/termai/internal/db"
	"github.com/kujtimiihoxha/termai/internal/llm/agent"
	"github.com/kujtimiihoxha/termai/internal/tui"
	zone "github.com/lrstanley/bubblezone"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "termai",
	Short: "A terminal ai assistant",
	Long:  `A terminal ai assistant`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Flag("help").Changed {
			cmd.Help()
			return nil
		}
		debug, _ := cmd.Flags().GetBool("debug")
		err := config.Load(debug)
		if err != nil {
			return err
		}
		conn, err := db.Connect()
		if err != nil {
			return err
		}
		ctx := context.Background()

		app := app.New(ctx, conn)
		defer app.Close()
		app.Logger.Info("Starting termai...")
		zone.NewGlobal()
		tui := tea.NewProgram(
			tui.New(app),
			tea.WithAltScreen(),
			tea.WithMouseCellMotion(),
		)
		app.Logger.Info("Setting up subscriptions...")
		ch, unsub := setupSubscriptions(app)
		defer unsub()

		go func() {
			// Set this up once
			agent.GetMcpTools(ctx, app.Permissions)
			for msg := range ch {
				tui.Send(msg)
			}
		}()
		if _, err := tui.Run(); err != nil {
			return err
		}
		return nil
	},
}

func setupSubscriptions(app *app.App) (chan tea.Msg, func()) {
	ch := make(chan tea.Msg)
	wg := sync.WaitGroup{}
	ctx, cancel := context.WithCancel(app.Context)

	{
		sub := app.Logger.Subscribe(ctx)
		wg.Add(1)
		go func() {
			for ev := range sub {
				ch <- ev
			}
			wg.Done()
		}()
	}
	{
		sub := app.Sessions.Subscribe(ctx)
		wg.Add(1)
		go func() {
			for ev := range sub {
				ch <- ev
			}
			wg.Done()
		}()
	}
	{
		sub := app.Messages.Subscribe(ctx)
		wg.Add(1)
		go func() {
			for ev := range sub {
				ch <- ev
			}
			wg.Done()
		}()
	}
	{
		sub := app.Permissions.Subscribe(ctx)
		wg.Add(1)
		go func() {
			for ev := range sub {
				ch <- ev
			}
			wg.Done()
		}()
	}
	{
		sub := app.Status.Subscribe(ctx)
		wg.Add(1)
		go func() {
			for ev := range sub {
				ch <- ev
			}
			wg.Done()
		}()
	}
	return ch, func() {
		cancel()
		wg.Wait()
		close(ch)
	}
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("help", "h", false, "Help")
	rootCmd.Flags().BoolP("debug", "d", false, "Help")
}
